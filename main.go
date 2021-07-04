package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

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
	build            = ""

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

	r.GET("/sysinfo", func(c *gin.Context) {
		c.String(http.StatusOK, fmt.Sprintf("Build: %s\nNumGoroutine: %d\nGo version: %s", build, runtime.NumGoroutine(), runtime.Version()))
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

	router.POST("/"+botURI, func(c *gin.Context) {
		bytes, _ := ioutil.ReadAll(c.Request.Body)
		if isDebug {
			log.Println("callback received:", string(bytes))
		}
		var update tgbotapi.Update
		err := json.Unmarshal(bytes, &update)
		if err != nil {
			log.Printf("callback data decode failed: %s \n%s", err, string(bytes))
			c.String(http.StatusInternalServerError, "request recode failed.")
			return
		}

		if update.Message != nil {
			log.Printf("%d[%s] %s ", update.Message.Chat.ID, update.Message.From.UserName, update.Message.Text)
			botMessageProcess(bot, update.Message)
		}
		c.String(http.StatusOK, "OK")
	})

	send := func(c *gin.Context, channelID, token, data string) error {
		defer func() {
			if err := recover(); err != nil {
				c.String(http.StatusBadRequest, fmt.Sprintf("Error: %s", err))
			}
		}()

		if channelID == "" || token == "" || data == "" {
			return errors.New("wrong params")
		}

		ch, err := d.NewChannel(c.Request.Context(), firebaseToken)
		if err != nil {
			log.Println("db connect failed: ", err)
			return errors.New("db connect failed with error")
		}
		defer ch.Close()
		channelInfo, err := ch.Get(channelID)
		if err != nil {
			log.Println("fetch channel info failed:", err)
			return errors.New("fetch channel info failed with error")
		}
		if channelInfo == nil || channelInfo.Token != token {
			return errors.New("channel not exist or token not match")
		}

		message := decodeMessage(data) + "\n\nFrom [" + channelInfo.ID + "]"
		//send to owner
		tMessage := tgbotapi.NewMessage(channelInfo.Owner, message)
		bot.Send(tMessage)

		//send to all users
		for _, userID := range channelInfo.Users {
			tMessage := tgbotapi.NewMessage(userID, message)
			bot.Send(tMessage)
		}

		c.String(http.StatusOK, fmt.Sprintf("ok, send to %d user", len(channelInfo.Users)+1))
		return nil
	}

	router.GET("/send/:name/:token/:data", func(c *gin.Context) {
		channelName := c.Param("name")
		token := c.Param("token")
		data := c.Param("data")
		if channelName == "" || token == "" || data == "" {
			c.String(http.StatusBadRequest, "need more params")
			return
		}
		err := send(c, channelName, token, data)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
		}
	})

	router.POST("/send/:name/:token", func(c *gin.Context) {
		channelName := c.Param("name")
		token := c.Param("token")
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read request body failed.")
			return
		}
		data := string(body)
		if channelName == "" || token == "" || data == "" {
			c.String(http.StatusBadRequest, "need more params")
			return
		}
		err = send(c, channelName, token, data)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
		}
	})

	router.POST("/send", func(c *gin.Context) {
		channelName := c.GetHeader("X-ChannelName")
		token := c.GetHeader("X-ChannelToken")
		if channelName == "" || token == "" {
			c.String(http.StatusBadRequest, "need more params")
			return
		}
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, "read request body failed.")
			return
		}
		data := string(body)
		if len(body) < 1 {
			c.String(http.StatusBadRequest, "request body required.")
			return
		}
		err = send(c, channelName, token, data)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
		}
	})
}

func botMessageProcess(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if isDebug {
		log.Printf("botMessageProcess: %#v", message)
	}
	if !message.IsCommand() {
		bot.Send(buildBotResponse(message, "I can only process command now."))
		return
	}

	command := message.Command()
	args := message.CommandArguments()

	var response *tgbotapi.MessageConfig
	switch command {
	case "follow":
		response = botCommandFollow(message, args)
	case "unfollow":
		response = botCommandUnfollow(message, args)
	case "new":
		response = botCommandNewChannel(message, args)
	case "list":
		response = botCommandList(message, args)
	case "token":
		response = botCommandToken(message, args)
	case "myid":
		response = buildBotResponse(message, fmt.Sprintf("%d", message.Chat.ID))
	case "channel_users":
		response = botCommandChannelUsers(message, args)
	case "channel_kick":
		response = botCommandChannelKick(message, args)
	default:
		bot.Send(buildBotResponse(message, "command not defined"))
		return
	}
	bot.Send(response)
}

func botCommandFollow(message *tgbotapi.Message, args string) *tgbotapi.MessageConfig {
	userID := message.Chat.ID
	channelID := strings.TrimSpace(args)

	if channelID == "" {
		return buildBotResponse(message, "channel name can't empty")
	}

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, "conect to db failed.")
	}
	defer ch.Close()

	channelInfo, err := ch.Get(channelID)
	if err != nil {
		return buildBotResponse(message, err.Error())
	}

	if channelInfo == nil {
		return buildBotResponse(message, "channel ID not exists")
	}

	if userID == channelInfo.Owner {
		return buildBotResponse(message, "can't follow the channel you owned")
	}

	for _, user := range channelInfo.Users {
		if user == userID {
			return buildBotResponse(message, "already followed")
		}
	}

	channelInfo.Users = append(channelInfo.Users, userID)
	err = ch.Update(channelInfo)
	if err != nil {
		log.Println("update channel info failed ", err)
		return buildBotResponse(message, "update failed")
	}

	return buildBotResponse(message, "followed "+channelInfo.ID)
}
func botCommandUnfollow(message *tgbotapi.Message, args string) *tgbotapi.MessageConfig {
	userID := message.Chat.ID
	channelID := strings.TrimSpace(args)

	if channelID == "" {
		return buildBotResponse(message, "channel name can't empty")
	}

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, "conect to db failed.")
	}
	defer ch.Close()

	channelInfo, err := ch.Get(channelID)
	if err != nil {
		return buildBotResponse(message, err.Error())
	}

	if channelInfo == nil {
		return buildBotResponse(message, "channel ID not exists")
	}

	if userID == channelInfo.Owner {
		return buildBotResponse(message, "can't unfollow the channel you owned")
	}

	isFound := false
	for i, user := range channelInfo.Users {
		if user == userID {
			channelInfo.Users = append(channelInfo.Users[:i], channelInfo.Users[i+1:]...)
			isFound = true
			break
		}
	}
	if !isFound {
		return buildBotResponse(message, "not followed")
	}

	err = ch.Update(channelInfo)
	if err != nil {
		log.Println("update channel info failed ", err)
		return buildBotResponse(message, "update failed")
	}

	return buildBotResponse(message, "unfollowed "+channelInfo.ID)
}
func botCommandList(message *tgbotapi.Message, args string) *tgbotapi.MessageConfig {
	userID := message.Chat.ID

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, "conect to db failed.")
	}
	defer ch.Close()

	list, err := ch.GetAll()
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, "fetch list error")
	}

	ownedList := make([]string, 0)
	followedList := make([]string, 0)

	for _, item := range list {
		if item.Owner == userID {
			ownedList = append(ownedList, item.ID)
		}
		for _, follower := range item.Users {
			if follower == userID {
				followedList = append(followedList, item.ID)
			}
		}
	}

	result := []string{
		"owned channel: ",
	}
	result = append(result, ownedList...)
	result = append(result, "")
	result = append(result, "followed channel: ")
	result = append(result, followedList...)

	return buildBotResponse(message, strings.Join(result, "\n"))
}
func botCommandNewChannel(message *tgbotapi.Message, args string) (result *tgbotapi.MessageConfig) {
	log.Printf("create new channel: User: %d args: %s", message.Chat.ID, args)
	userID := message.Chat.ID
	channelName := strings.TrimSpace(args)

	result = buildBotResponse(message, "")

	defer func() {
		if err := recover(); err != nil {
			log.Println("Error: ", err)
			result.Text = fmt.Sprintf("Error: %s", err)
		}
	}()

	if !checkChannelName(channelName) {
		panic("name only accept [a-zA-Z0-9_]")
	}

	if userID != adminChatID {
		panic("only admin can create new channel")
	}

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		panic("db connect err")
	}
	defer ch.Close()

	data := &d.ChannelData{
		ID:        channelName,
		Token:     generateToken(),
		Owner:     userID,
		OwnerName: message.Chat.UserName,
	}

	err = ch.Create(data)
	if err != nil {
		log.Println("Error: ", err)
		panic("create channel failed")
	}
	result.Text = fmt.Sprintf("create channel ok\nID: %s\ntoken: %s", data.ID, data.Token)
	return
}

func botCommandChannelUsers(message *tgbotapi.Message, args string) (result *tgbotapi.MessageConfig) {
	log.Printf("channel list users: User: %d args: %s\n", message.Chat.ID, args)
	userID := message.Chat.ID
	channelName := strings.TrimSpace(args)
	result = buildBotResponse(message, "")

	if channelName == "" {
		result.Text = "channel name cannot empty"
		return
	}

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, err.Error())
	}
	defer ch.Close()

	channelInfo, err := ch.Get(channelName)
	if err != nil {
		return buildBotResponse(message, err.Error())
	}
	if channelInfo == nil {
		return buildBotResponse(message, "channel ID not exists")
	}

	if channelInfo.Owner != userID {
		return buildBotResponse(message, "only owner can get user list")
	}

	var s strings.Builder
	s.WriteString("channel members: \n\n")
	for _, userID := range channelInfo.Users {
		fmt.Fprintf(&s, " %d\n", userID)
	}
	s.WriteString("\n===End===\n")
	result.Text = s.String()
	return
}

func botCommandChannelKick(message *tgbotapi.Message, args string) (result *tgbotapi.MessageConfig) {
	log.Printf("channel kick: User: %d args: %s\n", message.Chat.ID, args)
	userID := message.Chat.ID
	result = buildBotResponse(message, "")
	params := strings.Split(strings.TrimSpace(args), " ")
	if len(params) != 2 {
		result.Text = "wrong params, channel_kick [channel_name] [user_id]"
		return
	}

	channelName := params[0]
	targetUserID, err := strconv.ParseInt(params[1], 10, 64)
	if err != nil {
		result.Text = "params parse failed"
		return
	}

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, err.Error())
	}
	defer ch.Close()

	channelInfo, err := ch.Get(channelName)
	if err != nil {
		return buildBotResponse(message, err.Error())
	}
	if channelInfo == nil {
		return buildBotResponse(message, "channel ID not exists")
	}

	if channelInfo.Owner != userID {
		return buildBotResponse(message, "only owner can do this")
	}

	isExist := false
	for i, memberID := range channelInfo.Users {
		if memberID == targetUserID {
			copy(channelInfo.Users[i:], channelInfo.Users[i+1:])
			channelInfo.Users = channelInfo.Users[:len(channelInfo.Users)-1]
			isExist = true
			break
		}
	}

	if !isExist {
		result.Text = "user not found"
		return
	}
	err = ch.Update(channelInfo)
	if err != nil {
		log.Println("update channel info failed ", err)
		return buildBotResponse(message, "update failed")
	}

	var s strings.Builder
	s.WriteString(" current channel members: \n")
	for _, userID := range channelInfo.Users {
		fmt.Fprintf(&s, " %d\n", userID)
	}
	s.WriteString("\n===End===\n")
	result.Text = s.String()
	return
}

func botCommandToken(message *tgbotapi.Message, args string) *tgbotapi.MessageConfig {
	log.Printf("fetch channel token: User: %d args: %s", message.Chat.ID, args)
	userID := message.Chat.ID
	channelName := strings.TrimSpace(args)

	ch, err := d.NewChannel(context.Background(), firebaseToken)
	if err != nil {
		log.Println("Error: ", err)
		return buildBotResponse(message, err.Error())
	}
	defer ch.Close()

	channelInfo, err := ch.Get(channelName)
	if err != nil {
		return buildBotResponse(message, err.Error())
	}
	if channelInfo == nil {
		return buildBotResponse(message, "channel ID not exists")
	}

	if channelInfo.Owner != userID {
		return buildBotResponse(message, "only owner can fetch token")
	}

	return buildBotResponse(message, fmt.Sprintf("token: %s", channelInfo.Token))
}

func buildBotResponse(message *tgbotapi.Message, reply string) *tgbotapi.MessageConfig {
	response := tgbotapi.NewMessage(message.Chat.ID, reply)
	response.ReplyToMessageID = message.MessageID
	return &response
}

func checkChannelName(name string) bool {
	r, _ := regexp.Compile("^[0-9a-zA-Z_]{2,}$")
	return r.MatchString(name)
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

func generateToken() string {
	rand.Seed(time.Now().UnixNano())
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 12)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
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
