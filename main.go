package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fasthttp/router"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type record struct {
	Expires time.Time `json:"expires"`
	Uri     string    `json:"uri"`
	Key     string    `json:"key"`
	Gurl    string    `json:"gurl"`
	Token   string    `json:"token"`
}

func genKey(key_length uint, div_freq uint) *bytes.Buffer {
	// url safe characters
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	// build our key and get uri
	var k bytes.Buffer

	for c := uint(0); c <= key_length; c++ {
		// if it's not the first or last
		if c != uint(0) && c != key_length && c%div_freq == 0 {
			// every 5 characters insert a dash.
			k.WriteRune('-')
		}
		k.WriteRune(alphabet[rand.Intn(len(alphabet))])
	}
	return &k
}

// super basic logger
func reqLogger(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		// Normmaly I'd use log.WithFields() but it adds annoying whitespace for..some reason.
		// TODO: resolve the whitespace thing.
		log.Debugf("[%s] %s %s | (%s, %s, %s)",
			ctx.Time().String(),
			string(ctx.Method()),
			string(ctx.Path()),
			ctx.RemoteAddr().String(),
			string(ctx.Referer()),
			string(ctx.UserAgent()))
		next(ctx)
	}
}

// config
var (
	host       = flag.String("addr", ":9999", "A simple bindhost string, eg: \":9999\" or \"127.0.0.1\"")
	uri_size   = flag.Uint("len", 10, "set the generated uri string length")
	dash       = flag.Uint("sep", 5, "set how often to insert a dash")
	cache_to   = flag.String("exp", "24h", "set the time delta for cache expiry")
	web        = flag.String("dir", "./static", "set the directory for web/html files served at webroot")
	warn_level = flag.String("log", "Info", "set the alert/warn level of the logging. Info, Warn, Error, Fatal, Panic")
)

func main() {
	flag.Parse()

	// set up log here
	llvl, err := log.ParseLevel(*warn_level)
	if err != nil {
		log.Panicf("couldn't parse log level: %s", err)
	}
	log.SetLevel(llvl)

	// golang seed is subpar.
	rand.Seed(time.Now().UnixNano())

	ct, err := time.ParseDuration(*cache_to)
	if err != nil {
		log.Panicf("couldn't parse cache ttl: %s", err)
	}

	// make db connection
	db, err := bolt.Open("uri.store", 0600, nil)
	if err != nil {
		// just drop out with the error, we need the db
		log.Panicf("couldn't open db: %s", err)
	}
	defer db.Close()

	// make sure our prefered bucket exists.
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("gurls"))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Panicf("bucket creation failure: %s", err)
	}

	// set up cache watcher
	go func() {
		for {
			err := db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("gurls"))
				err = b.ForEach(func(k, v []byte) error {
					if v != nil {
						var rec record
						err = json.Unmarshal(v, &rec)
						if err != nil {
							return err
						}
						if time.Now().After(rec.Expires) {
							log.Infof("%s expired %fs ago", rec.Key, time.Now().Sub(rec.Expires).Seconds())
							err = b.Delete([]byte(rec.Key))
							if err != nil {
								log.Errorf("couldn't delete key ", err)
							}
						}
					}
					return nil
				})
				return nil
			})
			if err != nil {
				log.Errorf("db operation failure: %#v", err)
			}
			time.Sleep(time.Second)
		}
	}()
	// build our file server
	fs := &fasthttp.FS{
		Root:               *web,
		IndexNames:         []string{"index.html"},
		GenerateIndexPages: true,
	}
	fsHandler := fs.NewRequestHandler()

	// set up our routing
	rtr := router.New()
	rtr.GET("/", func(ctx *fasthttp.RequestCtx) {
		fsHandler(ctx)
	})
	rtr.GET("/c/{uri:*}", func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", []byte("*"))
		err := db.Update(func(tx *bolt.Tx) error {
			// build our key and get uri
			b := tx.Bucket([]byte("gurls"))
			key := genKey(*uri_size, *dash)
			// check for collision
			found := b.Get(key.Bytes())
			if found != nil {
				// I'm on the fence about this. Might not even work right.
				// The chances of collision are astronomically low with anything 10 characters+
				ctx.Redirect(ctx.URI().String(), 302)
				return nil
			}
			uri := ctx.UserValue("uri").(string)

			// there still has to be a better way, still learning the library.
			var gurl string
			if ctx.IsTLS() {
				gurl += "https://"
			} else {
				gurl += "http://"
			}
			gurl += string(ctx.Host()) + "/b/" + key.String()

			// get a uuid
			token, err := uuid.NewRandom()
			if err != nil {
				log.Errorf("couldn't get a uuid:", err)
				return err
			}

			// marshal it
			rec := record{
				Expires: time.Now().Add(ct),
				Key:     key.String(),
				Uri:     "https://" + uri,
				Gurl:    gurl,
				Token:   token.String(),
			}
			out, err := json.Marshal(rec)
			if err != nil {
				log.Errorf("couldn't marshal: %s", err)
				return err
			}

			// send it
			err = b.Put(key.Bytes(), out)
			if err != nil {
				log.Errorf("couldn't store gurl in db: %s", err)
				return err
			}
			fmt.Fprint(ctx, string(out))
			return nil
		})
		if err != nil {
			log.Errorf("error in /c/ db update: %s", err)
		}
	})
	rtr.GET("/b/{req_uri}", func(ctx *fasthttp.RequestCtx) {
		var rec record
		// build redirect
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("gurls"))
			key := []byte(ctx.UserValue("req_uri").(string))
			err = json.Unmarshal(b.Get(key), &rec)
			if err != nil {
				ctx.NotFound()
				return err
			}
			rec.Expires = time.Now().Add(ct)
			out, err := json.Marshal(rec)
			if err != nil {
				log.Errorf("couldn't marshal: ", err)
				return err
			}
			err = b.Put(key, out)
			if err != nil {
				// there isn't really an issue here, so we won't return it, just log it.
				// the link will just expire sooner than expected.
				log.Errorf("couldn't update expiry time:", err)
			}

			// send it
			ctx.Redirect(rec.Uri, 302)
			return nil
		})
	})
	rtr.GET("/d/{key}/{token}", func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", []byte("*"))
		err = db.Update(func(tx *bolt.Tx) error {
			var rec record
			b := tx.Bucket([]byte("gurls"))
			key := []byte(ctx.UserValue("key").(string))

			// build the data and handle it.
			data := b.Get(key)
			if data != nil {
				err = json.Unmarshal(data, &rec)
				if err != nil {
					log.Errorf("couldn't unmarshal from db: %s", err)
					return err
				}
			} else {
				log.Infof("bad lookup for %s", key)
				ctx.SetStatusCode(fasthttp.StatusUnauthorized)
				fmt.Fprint(ctx, "401 Access Denied")
				return nil
			}
			if rec.Token == ctx.UserValue("token") {
				err = b.Delete(key)
				if err != nil {
					return err
				}
				fmt.Fprint(ctx, "OK")
				return nil
			}
			ctx.SetStatusCode(fasthttp.StatusUnauthorized)
			fmt.Fprint(ctx, "401 Access Denied")
			return nil
		})
		if err != nil {
			// might be silly, but it will help admin narrow down the specific request.
			id, err := uuid.NewRandom()
			if err != nil {
				log.Errorf("couldn't get uuid: %s", err)
				return
			}
			log.Errorf("(%s) delete request failed: %s", id, err)
			ctx.SetStatusCode(fasthttp.StatusInternalServerError)
			fmt.Fprintf(ctx, "Internal Error, Contact System Administrator with key: %s", id)
		}
	})
	log.Fatal(fasthttp.ListenAndServe(*host, reqLogger(rtr.Handler)))
}
