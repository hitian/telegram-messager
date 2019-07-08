package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

var (
	firebaseToken []byte
	telegramToken = ""
	firebaseApp   *firebase.App
	store         *firestore.Client

	encodeFirebaseTokenFile = flag.String("tokenFile", "", "firebase token file path")
)

func main() {
	flag.Parse()
	if *encodeFirebaseTokenFile != "" {
		result := tokenFileContentBase64Encode(*encodeFirebaseTokenFile)
		fmt.Println(result)
		os.Exit(0)
	}

	firebaseToken = parseFirebaseToken(os.Getenv("FIREBASE_TOKEN"))
	telegramToken = os.Getenv("TELEGRAM_TOKEN")
	if telegramToken == "" {
		log.Fatal("telegram token empty")
	}

	firebaseOption := option.WithCredentialsJSON(firebaseToken)
	var err error
	firebaseApp, err = firebase.NewApp(context.Background(), nil, firebaseOption)
	if err != nil {
		log.Fatalf("firebase app create failed err: %s", err.Error())
	}

	store, err = firebaseApp.Firestore(context.Background())
	if err != nil {
		log.Fatalf("firebase store init failed err: %s", err.Error())
	}
	defer store.Close()

	router := createRouter()

	listenAddr := "127.0.0.1:9000"
	if os.Getenv("PORT") != "" {
		listenAddr = ":" + os.Getenv("PORT")
	}
	log.Fatal(router.Run(listenAddr))
}

func createRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World")
	})

	r.GET("/store", func(c *gin.Context) {
		iter, err := store.Collection("channel").Documents(c.Request.Context()).GetAll()
		if err != nil {
			c.String(http.StatusBadRequest, "")
		}
		for _, data := range iter {
			log.Println(data.Data())
		}
		c.String(http.StatusOK, "test")
	})

	return r
}

func parseFirebaseToken(base64Token string) []byte {
	if base64Token == "" {
		log.Fatal("firebase token empty")
	}
	token, err := base64.StdEncoding.DecodeString(base64Token)
	if err != nil {
		log.Fatalf("firebase token base64 decode err: %s", err.Error())
	}
	return token
}

func tokenFileContentBase64Encode(filePath string) string {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("read file %s failed err: %s", filePath, err.Error())
	}
	return base64.StdEncoding.EncodeToString(data)
}
