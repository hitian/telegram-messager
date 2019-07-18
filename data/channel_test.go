package data

import (
	"context"
	"io/ioutil"
	"log"
	"os"
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
	tokenData, _ := ioutil.ReadFile(os.Getenv("HOME") + string(os.PathSeparator) + ".firebase.json")
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
		ID:        "channel_name",
		Token:     "channel_token",
		Users:     []int64{},
		Owner:     12345678,
		OwnerName: "admin",
	}

	err := ch.Create(row)
	if err != nil {
		t.FailNow()
	}

}

func TestGet(t *testing.T) {
	prepare(t)
	row, err := ch.Get("channel_name")
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	if row == nil {
		log.Println("data empty??")
		t.FailNow()
	}
}

func TestGetNotExistData(t *testing.T) {
	prepare(t)
	row, err := ch.Get("channel_name_not_exists")
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	if row != nil {
		log.Println("data exists??")
		t.FailNow()
	}
}

func TestRemove(t *testing.T) {
	prepare(t)
	err := ch.Remove("channel_name")
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
}

func TestUpdate(t *testing.T) {
	prepare(t)
	data, err := ch.Get("channel_name")
	if err != nil {
		log.Println(err)
		t.FailNow()
	}
	if data == nil {
		log.Println("data not exist, skip")
		return
	}
	data.Users = append(data.Users, 123456)
	err = ch.Update(data)
	if err != nil {
		log.Println("update failed.", err)
		t.FailNow()
	}
}
