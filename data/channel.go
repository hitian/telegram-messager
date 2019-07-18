package data

import (
	"context"
	"errors"
	"log"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
	ID        string  `json:"id" firestore:"id"`
	Token     string  `json:"token" firestore:"token"`
	Owner     int64   `json:"owner" firestore:"owner"`
	OwnerName string  `json:"owner_name" firestore:"owner_name"`
	Users     []int64 `json:"users" firestore:"users"`
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

func (c *Channel) Get(ID string) (*ChannelData, error) {
	doc, err := c.db.Doc(ID).Get(c.ctx)
	if err != nil {
		//record not exists.
		if grpc.Code(err) == codes.NotFound {
			return nil, nil
		}
		return nil, err
	}
	var data ChannelData
	if err = doc.DataTo(&data); err != nil {
		return nil, err
	}
	return &data, nil
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
	existData, err := c.Get(data.ID)
	if err != nil {
		return err
	}
	if existData != nil {
		return errors.New("Channel Name exists")
	}
	doc := c.db.Doc(data.ID)
	res, err := doc.Create(c.ctx, data)
	if err != nil {
		return err
	}
	log.Println("create ", res)
	return nil
}

func (c *Channel) Remove(ID string) error {
	res, err := c.db.Doc(ID).Delete(c.ctx)
	if err != nil {
		return err
	}
	log.Println(res)
	return nil
}

func (c *Channel) Update(data *ChannelData) error {
	res, err := c.db.Doc(data.ID).Set(c.ctx, data)
	if err != nil {
		return err
	}
	log.Println("update ok, ", res)
	return nil
}
