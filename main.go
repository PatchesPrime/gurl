package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

type record struct {
	Expires time.Time `json:"last_vist"`
	Uri     string    `json:"uri"`
	Key     string    `json:"key"`
	Gurl    string    `json:"gurl"`
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
		ctx.Logger().Printf("%s", ctx.UserAgent())
		next(ctx)
	}
}

func main() {

	// config
	host := flag.String("b", ":9999", "A simple bindhost string, eg: \":9999\" or \"127.0.0.1\"")
	uri_size := flag.Uint("l", 10, "set the generated uri string length")
	dash := flag.Uint("d", 5, "set how often to insert a dash")
	cache_to := flag.String("c", "24h", "set the time delta for cache expiry")
	flag.Parse()

	// golang seed is subpar.
	rand.Seed(time.Now().UnixNano())

	ct, err := time.ParseDuration(*cache_to)
	if err != nil {
		log.Println("couldn't parse cache ttl: ", err)
	}

	// make db connection
	db, err := bolt.Open("uri.store", 0600, nil)
	if err != nil {
		// just drop out with the error, we need the db
		log.Fatal(err)
	}
	defer db.Close()

	// make sure our prefered bucket exists.
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("gurls"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})

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
							log.Printf("%s expired %fs ago", rec.Key, time.Now().Sub(rec.Expires).Seconds())
							err = b.Delete([]byte(rec.Key))
							if err != nil {
								log.Println("couldn't delete key ", err)
							}
						}
					}
					return nil
				})
				return nil
			})
			if err != nil {
				log.Printf("db operation failure: %#v", err)
			}
			time.Sleep(time.Second)
		}
	}()

	// set up our routing
	rtr := router.New()
	rtr.GET("/", func(ctx *fasthttp.RequestCtx) {
		fmt.Fprintln(ctx, "Haaaaay, gurl! This is an ultralight url shortener.\nTry /c/your-url!")
	})
	rtr.GET("/c/{uri}", func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", []byte("*"))
		db.Update(func(tx *bolt.Tx) error {
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

			// marshal it
			rec := record{
				Expires: time.Now().Add(ct),
				Key:     key.String(),
				Uri:     "https://" + uri,
				Gurl:    gurl,
			}
			out, err := json.Marshal(rec)
			if err != nil {
				log.Fatal("couldn't marshal: ", err)
			}

			// send it
			err = b.Put(key.Bytes(), out)
			fmt.Fprint(ctx, string(out))
			return err
		})
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
				log.Fatal("couldn't marshal: ", err)
			}
			err = b.Put(key, out)

			// send it
			ctx.Redirect(rec.Uri, 302)
			return nil
		})
	})
	rtr.GET("/d/{key}", func(ctx *fasthttp.RequestCtx) {
		ctx.Response.Header.SetBytesV("Access-Control-Allow-Origin", []byte("*"))
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("gurls"))
			key := []byte(ctx.UserValue("key").(string))
			err = b.Delete(key)
			if err != nil {
				return err
			}
			fmt.Fprint(ctx, "done")
			return nil
		})
		if err != nil {
			log.Println("delete request failed: ", err)
		}
	})
	log.Fatal(fasthttp.ListenAndServe(*host, reqLogger(rtr.Handler)))
}
