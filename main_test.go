package main

import (
	"log"
	"testing"
)

func TestCheckChannelName(t *testing.T) {
	list := make(map[string]bool)
	list["123"] = true
	list["asd"] = true
	list["_jifds"] = true
	list["fd fd"] = false
	list["fd)fdf"] = false
	list[" fdf"] = false

	for name, isOK := range list {
		if checkChannelName(name) != isOK {
			log.Printf("[%s] should %t \n", name, isOK)
			t.Fail()
		}
	}
}
