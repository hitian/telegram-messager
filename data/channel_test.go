package data

import (
	"context"
	"io/ioutil"
	"log"
	"testing"
)

var (
	token []byte
	ch    *Channel
)

func prepare(t *testing.T) {
	if ch != nil {
		return
	}
	tokenData, _ := ioutil.ReadFile("/Users/jia/.firebase.json")
	token = tokenData
	c, err := NewChannel(context.Background(), token)
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	ch = c
}

func TestGetAll(t *testing.T) {
	prepare(t)

	list, err := ch.GetAll()
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	log.Println(list)
}

func TestCreate(t *testing.T) {
	prepare(t)

	row := &ChannelData{
		ID:    "test_1",
		Token: "fdjlskafjdas",
		Users: []string{"tian"},
	}

	err := ch.Create(row)
	if err != nil {
		t.FailNow()
	}

}
