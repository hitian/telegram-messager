package data

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
)

var (
	firebaseApp *firebase.App
	store       *firestore.Client
)

type Channel struct {
	ctx   context.Context
	store *firestore.Client
	db    *firestore.CollectionRef
}

type ChannelData struct {
	ID    string   `json:"id" firestore:"id"`
	Token string   `json:"token" firestore:"token"`
	Users []string `json:"users" firestore:"users"`
}

func NewChannel(ctx context.Context, token []byte) (*Channel, error) {
	firebaseOption := option.WithCredentialsJSON(token)
	firebaseApp, err := firebase.NewApp(ctx, nil, firebaseOption)
	if err != nil {
		log.Fatalf("firebase app create failed err: %s", err.Error())
		return nil, err
	}

	store, err = firebaseApp.Firestore(ctx)
	if err != nil {
		log.Fatalf("firebase store init failed err: %s", err.Error())
		return nil, err
	}
	return &Channel{
		ctx:   ctx,
		store: store,
		db:    store.Collection("channel"),
	}, nil
}

func (c *Channel) Close() {
	c.store.Close()
}

func (c *Channel) GetAll() ([]ChannelData, error) {
	list := make([]ChannelData, 0)
	iter, err := c.db.Documents(c.ctx).GetAll()
	if err != nil {
		return list, err
	}
	for _, row := range iter {
		var d ChannelData
		if err := row.DataTo(&d); err != nil {
			return list, err
		}
		list = append(list, d)
	}
	return list, nil
}

func (c *Channel) Create(data *ChannelData) error {
	doc := c.db.NewDoc()
	res, err := doc.Create(c.ctx, data)
	if err != nil {
		return err
	}
	log.Println("create ", res)
	return nil
}
