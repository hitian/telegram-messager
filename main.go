package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	d "github.com/hitian/telegram-messager/data"
)

var (
	firebaseToken []byte
	telegramToken = ""
	// firebaseApp   *firebase.App
	// store         *firestore.Client

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

	r.GET("/list", func(c *gin.Context) {
		ch, err := d.NewChannel(c.Request.Context(), firebaseToken)
		if err != nil {
			panic(err)
		}
		defer ch.Close()

		list, err := ch.GetAll()
		if err != nil {
			panic(err)
		}

		var output []string
		for _, item := range list {
			output = append(output, fmt.Sprintf("ID: %s", item.ID))
		}

		c.String(http.StatusOK, strings.Join(output, "\n"))
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
