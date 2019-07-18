package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/apex/gateway"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	d "github.com/hitian/telegram-messager/data"
)

var (
	firebaseToken    []byte
	telegramToken    = ""
	webhookURLPrefix string
	botURI           string
	adminChatID      int64
	isLambda         bool
	isDebug          = false

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

	webhookURLPrefix = os.Getenv("DOMAIN")
	botURI = os.Getenv("BOT_URI")
	adminChatIDString := os.Getenv("ADMIN_CHAT_ID")
	adminChatID, _ = strconv.ParseInt(adminChatIDString, 10, 64)
	isLambda = os.Getenv("AWS_LAMBDA") != ""
	isDebug = os.Getenv("DEBUG") != ""

	listenAddr := "127.0.0.1:9000"
	if portENV := os.Getenv("PORT"); portENV != "" {
		listenAddr = ":" + portENV
	}
	log.Printf("listen Addr: %s\n", listenAddr)
	log.Printf("webhookURLPrefix: %s\n", webhookURLPrefix)
	log.Printf("adminChatID: %d\n", adminChatID)
	log.Printf("is lambda: %t\n", isLambda)

	router := createRouter()

	initTelegramBot(router)

	if isLambda {
		log.Fatal(gateway.ListenAndServe(listenAddr, router))
	} else {
		//start http server.
		log.Fatal(router.Run(listenAddr))
	}
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

	r.GET("/sysinfo", func(c *gin.Context) {
		c.String(http.StatusOK, fmt.Sprintf("NumGoroutine: %d", runtime.NumGoroutine()))
	})

	return r
}

func initTelegramBot(router *gin.Engine) {
	if telegramToken == "" {
		log.Println("WARNING: telegramToken not exists. skip bot init.")
		return
	}
	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatal(err)
	}
	bot.Debug = isDebug

	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURLPrefix + botURI))
	if err != nil {
		log.Fatal(err)
	}
	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}

	ch := make(chan tgbotapi.Update, bot.Buffer)
	router.POST("/"+botURI, func(c *gin.Context) {
		bytes, _ := ioutil.ReadAll(c.Request.Body)
		var update tgbotapi.Update
		json.Unmarshal(bytes, &update)
		ch <- update
	})

	go func() {
		for {
			select {
			case msg := <-ch:
				if msg.Message == nil {
					log.Println("incoming message empty. continue.")
					continue
				}
				log.Printf("%d[%s] %s ", msg.Message.Chat.ID, msg.Message.From.UserName, msg.Message.Text)

				responseMsg := tgbotapi.NewMessage(msg.Message.Chat.ID, msg.Message.Text)
				responseMsg.ReplyToMessageID = msg.Message.MessageID

				bot.Send(responseMsg)
			}
		}
	}()

	router.GET("/send/:name/:token/:data", func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("Error: %s", err))
			}
		}()
		channelName := c.Param("name")
		token := c.Param("token")
		data := c.Param("data")
		if channelName == "" || token == "" || data == "" {
			panic("wrong param")
		}

		ch, err := d.NewChannel(c.Request.Context(), firebaseToken)
		if err != nil {
			panic("db connect err")
		}
		channelInfo, err := ch.Get(channelName)
		if err != nil {
			panic("channel info fetch err")
		}
		if channelInfo == nil || channelInfo.Token != token {
			panic("not allow")
		}

		message := decodeMessage(data)
		//send to owner
		tMessage := tgbotapi.NewMessage(channelInfo.Owner, message)
		bot.Send(tMessage)

		//send to all users
		for _, userID := range channelInfo.Users {
			tMessage := tgbotapi.NewMessage(userID, message)
			bot.Send(tMessage)
		}

		c.String(http.StatusOK, fmt.Sprintf("ok, send to %d user", len(channelInfo.Users)+1))
	})
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

func decodeMessage(msg string) string {
	base64Decoded, err := base64.StdEncoding.DecodeString(msg)
	if err != nil {
		// not base64 encode
		return msg
	}
	return string(base64Decoded)
}
