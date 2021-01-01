package main

import (
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/boltdb/bolt"
	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

type record struct {
	last_visit time.Time
	uri        fasthttp.URI
	key        string
}

func main() {
	// url safe characters
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

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

	// config
	host := "127.0.0.1:9999"
	uri_size := 10
	// simple fasthttp server
	rtr := router.New()
	rtr.GET("/", func(ctx *fasthttp.RequestCtx) {
		fmt.Fprintln(ctx, "Haaaaay, gurl! This is an ultralight url shortener.\nTry /create/your-url!")
	})
	// debug stuff
	// rtr.GET("/view", func(ctx *fasthttp.RequestCtx) {
	// 	db.View(func(tx *bolt.Tx) error {
	// 		b := tx.Bucket([]byte("gurls"))
	// 		err := b.ForEach(func(k, v []byte) error {
	// 			log.Printf("processing k,v: %s,%s\n", k, v)
	// 			fmt.Fprintf(ctx, "%s\n\t->%s\n", k, v)
	// 			return nil
	// 		})
	// 		return err
	// 	})
	// })
	rtr.GET("/create", func(ctx *fasthttp.RequestCtx) {
		fmt.Fprintln(ctx, "Oops! You forgot a trailing /some-url-here after your /create there!")
	})
	rtr.GET("/create/{uri}", func(ctx *fasthttp.RequestCtx) {
		db.Update(func(tx *bolt.Tx) error {
			var k bytes.Buffer
			for c := 0; c <= uri_size; c++ {
				// if it's not the first or last
				if c != 0 && c != uri_size && c%5 == 0 {
					// every 5 characters insert a dash.
					k.WriteRune('-')
				}
				k.WriteRune(alphabet[rand.Intn(len(alphabet))])
			}

			uri := ctx.UserValue("uri").(string)
			fmt.Fprintf(ctx, "key: %s\nvalue: %s", k.String(), uri)

			b := tx.Bucket([]byte("gurls"))
			err := b.Put(k.Bytes(), []byte(uri))
			return err
		})
	})
	rtr.GET("/b/{req_uri}", func(ctx *fasthttp.RequestCtx) {
		var req_uri bytes.Buffer
		// assume https because why not
		req_uri.WriteString("https://")

		// build redirect
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("gurls"))
			req_uri.WriteString(string(b.Get([]byte(ctx.UserValue("req_uri").(string)))))
			return nil
		})
		rtr.RedirectFixedPath = true
		// send it
		ctx.Redirect(req_uri.String(), 302)
	})
	log.Fatal(fasthttp.ListenAndServe(host, rtr.Handler))
}
