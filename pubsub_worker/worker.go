// Copyright 2015 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Sample pubsub_worker demonstrates the use of the Cloud Pub/Sub API to communicate between two modules.
// See https://cloud.google.com/go/getting-started/using-pub-sub
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"

	"github.com/Rican7/retry"
	"github.com/Rican7/retry/strategy"
	"github.com/Rican7/retry/backoff"
	"github.com/tusharm/bookshelf"

	"golang.org/x/net/context"

	"google.golang.org/api/books/v1"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine"
)

const subName = "book-worker-sub"

var (
	countMu sync.Mutex
	count int

	booksClient  *books.Service
	subscription *pubsub.Subscription
)

func main() {
	ctx := context.Background()

	if bookshelf.PubsubClient == nil {
		log.Fatal("You must configure the Pub/Sub client in config.go before running pubsub_worker.")
	}

	var err error
	booksClient, err = books.New(http.DefaultClient)
	if err != nil {
		log.Fatalf("could not access Google Books API: %v", err)
	}

	// ignore returned errors, which will be "already exists". If they're fatal
	// errors, then following calls (e.g. in the subscribe function) will also fail.
	topic, _ := bookshelf.PubsubClient.NewTopic(ctx, bookshelf.PubsubTopicID)
	subscription, _ = bookshelf.PubsubClient.NewSubscription(ctx, subName, topic, 0, nil)

	// Start worker goroutine.
	go subscribe()

	// Sync cache
	go syncBooksCache()

	// [START http]
	// Publish a count of processed requests to the server homepage.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		countMu.Lock()
		defer countMu.Unlock()
		fmt.Fprintf(w, "This worker has processed %d books.", count)
	})

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	log.Fatal(http.ListenAndServe(":" + port, nil))
	// [END http]
}

func subscribe() {
	ctx := context.Background()
	it, err := subscription.Pull(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for {
		msg, err := it.Next()
		if err != nil {
			log.Fatalf("could not pull: %v", err)
		}
		var id int64
		if err := json.Unmarshal(msg.Data, &id); err != nil {
			log.Printf("could not decode message data: %#v", msg)
			msg.Done(true)
			continue
		}

		log.Printf("[ID %d] Processing.", id)
		go func(ctxt context.Context) {
			if err := update(ctxt, id); err != nil {
				log.Printf("[ID %d] could not update: %v", id, err)
				msg.Done(false) // NACK
				return
			}

			countMu.Lock()
			count++
			countMu.Unlock()

			msg.Done(true) // ACK
			log.Printf("[ID %d] ACK", id)
		}(appengine.BackgroundContext())
	}
}

// update retrieves the book with the given ID, finds metata from the Books
// server and updates the database with the book's details.
func update(ctxt context.Context, bookID int64) error {
	book, err := bookshelf.DB.GetBook(bookID)
	if err != nil {
		return err
	}

	vols, err := booksClient.Volumes.List(book.Title).Do()
	if err != nil {
		return err
	}

	if len(vols.Items) == 0 {
		return nil
	}

	info := vols.Items[0].VolumeInfo
	book.ID = bookID
	book.Title = info.Title
	book.Author = strings.Join(info.Authors, ", ")
	book.PublishedDate = info.PublishedDate
	if book.Description == "" {
		book.Description = info.Description
	}
	if book.ImageURL == "" && info.ImageLinks != nil {
		url := info.ImageLinks.Thumbnail
		// Replace http with https to prevent Content Security errors on the page.
		book.ImageURL = strings.Replace(url, "http://", "https://", 1)
	}

	if err = bookshelf.DB.UpdateBook(book); err != nil {
		return err
	}

	if err = bookshelf.IndexBook(ctxt, book); err != nil {
		log.Printf("search: failed to update index for book [%v]: %v", book.ID, err)
	}

	return nil
}

// syncs the books cache regularly with the datastore
func syncBooksCache() {
	loop:
	for {
		select {
		case <-time.Tick(5 * time.Second):
			ctxt := appengine.BackgroundContext()

			newBooks, err := bookshelf.DB.ListBooks()
			if err != nil {
				log.Printf("sync-worker: failed to get list from DB; sync failed: %v", err)
				continue loop
			}

			_ = retry.Retry(func(attempt uint) error {
				books := make([]*bookshelf.Book, 0)
				item, err := memcache.JSON.Get(ctxt, bookshelf.BooksKey, &books)
				if err != nil {
					log.Printf("sync-worker: failed to get books from cache; sync failed, will retry: %v", err)
					return err
				}

				item.Object = newBooks
				if err := memcache.JSON.CompareAndSwap(ctxt, item); err != nil {
					log.Printf("sync-worker: failed to set books in cache; sync failed, will retry: %v", err)
					return err
				}

				return nil
			}, strategy.Limit(5), strategy.Backoff(backoff.BinaryExponential(10 * time.Millisecond)))
		}
	}
}