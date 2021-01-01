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
	Last_visit time.Time `json:"last_vist"`
	Uri        string    `json:"uri"`
	Key        string    `json:"key"`
}

func main() {
	// url safe characters
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	// config
	host := flag.String("b", ":9999", "A simple bindhost string, eg: \":9999\" or \"127.0.0.1\"")
	uri_size := flag.Uint("l", 10, "set the generated uri string length")
	dash := flag.Uint("d", 5, "set how often to insert a dash")
	dt, err := time.ParseDuration("10s")
	if err != nil {
		dt = time.Second * 10
	}
	cache_to := flag.String("c", dt.String(), "set the time in seconds for cache expiry")
	flag.Parse()

	// golang seed is subpar.
	rand.Seed(time.Now().UnixNano())

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
						ct, err := time.ParseDuration(*cache_to)
						if err != nil {
							ct = time.Second * 10
						}
						if time.Now().Sub(rec.Last_visit) >= ct {
							log.Printf("%s has expired @ %d seconds", rec.Key, time.Now().Sub(rec.Last_visit))
							err = b.Delete([]byte(rec.Key))
							if err != nil {
								log.Fatal("couldn't delete key ", err)
							}
						}
					}
					return nil
				})
				return nil
			})
			if err != nil {
				log.Fatalf("watcher died: %#v", err)
			}
			time.Sleep(time.Second)
		}
	}()

	// simple fasthttp server
	rtr := router.New()
	rtr.GET("/", func(ctx *fasthttp.RequestCtx) {
		fmt.Fprintln(ctx, "Haaaaay, gurl! This is an ultralight url shortener.\nTry /create/your-url!")
	})
	rtr.GET("/create", func(ctx *fasthttp.RequestCtx) {
		fmt.Fprintln(ctx, "Oops! You forgot a trailing /some-url-here after your /create there!")
	})
	rtr.GET("/create/{uri}", func(ctx *fasthttp.RequestCtx) {
		db.Update(func(tx *bolt.Tx) error {
			// build our key and get uri
			var k bytes.Buffer
			for c := uint(0); c <= *uri_size; c++ {
				// if it's not the first or last
				if c != uint(0) && c != *uri_size && c%*dash == 0 {
					// every 5 characters insert a dash.
					k.WriteRune('-')
				}
				k.WriteRune(alphabet[rand.Intn(len(alphabet))])
			}
			uri := ctx.UserValue("uri").(string)

			// marshal it
			rec := record{Last_visit: time.Now(), Key: k.String(), Uri: "https://" + uri}
			out, err := json.Marshal(rec)
			if err != nil {
				log.Fatal("couldn't marshal: ", err)
			}

			b := tx.Bucket([]byte("gurls"))
			err = b.Put(k.Bytes(), out)
			fmt.Fprint(ctx, string(out))
			return err
		})
	})
	rtr.GET("/b/{req_uri}", func(ctx *fasthttp.RequestCtx) {
		var rec record
		// assume https because why not
		// req_uri.WriteString("https://")

		// build redirect
		err = db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("gurls"))
			err = json.Unmarshal(b.Get([]byte(ctx.UserValue("req_uri").(string))), &rec)
			if err != nil {
				ctx.NotFound()
				return err
			}

			// send it
			ctx.Redirect(rec.Uri, 302)
			return nil
		})
	})
	log.Fatal(fasthttp.ListenAndServe(*host, rtr.Handler))
}
