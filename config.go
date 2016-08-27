// Copyright 2015 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package bookshelf

import (
	"errors"
	"log"
	"os"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"

	"gopkg.in/mgo.v2"

	"github.com/gorilla/sessions"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	DB BookDatabase
	OAuthConfig *oauth2.Config

	StorageBucket     *storage.BucketHandle
	StorageBucketName string

	SessionStore sessions.Store

	PubsubClient *pubsub.Client

	// Force import of mgo library.
	_ mgo.Session
)

const PubsubTopicID = "fill-book-details"

func init() {
	var err error

	if DB, err = configureDatastoreDB(os.Getenv("DATASTORE_NAME")); err != nil {
		log.Fatal(err)
	}

	StorageBucketName = os.Getenv("BUCKET_NAME")
	if StorageBucket, err = configureStorage(StorageBucketName); err != nil {
		log.Fatal(err)
	}

	OAuthConfig = configureOAuthClient(os.Getenv("OAUTH_CLIENT_ID"), os.Getenv("OAUTH_CLIENT_SECRET"))

	cookieStore := sessions.NewCookieStore([]byte(os.Getenv("COOKIE_SECRET")))
	cookieStore.Options = &sessions.Options{
		HttpOnly: true,
	}
	SessionStore = cookieStore

	if PubsubClient, err = configurePubsub(os.Getenv("PUBSUB_CLIENT_ID")); err != nil {
		log.Fatal(err)
	}
}

func configureDatastoreDB(projectID string) (BookDatabase, error) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return newDatastoreDB(client)
}

func configureStorage(bucketID string) (*storage.BucketHandle, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return client.Bucket(bucketID), nil
}

func configurePubsub(projectID string) (*pubsub.Client, error) {
	if _, ok := DB.(*memoryDB); ok {
		return nil, errors.New("Pub/Sub worker doesn't work with the in-memory DB " +
		"(worker does not share its memory as the main app). Configure another " +
		"database in bookshelf/config.go first (e.g. MySQL, Cloud Datastore, etc)")
	}

	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Create the topic if it doesn't exist.
	if exists, err := client.Topic(PubsubTopicID).Exists(ctx); err != nil {
		return nil, err
	} else if !exists {
		if _, err := client.NewTopic(ctx, PubsubTopicID); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func configureOAuthClient(clientID, clientSecret string) *oauth2.Config {
	redirectURL := os.Getenv("OAUTH2_CALLBACK")
	if redirectURL == "" {
		redirectURL = "http://localhost:8080/oauth2callback"
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"email", "profile"},
		Endpoint:     google.Endpoint,
	}
}
