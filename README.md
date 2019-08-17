# telegram bot for group message push

## setup

env var require

```text
ADMIN_CHAT_ID   #admin's telegram ID
BOT_URI         #telegram bot's Webhook URL
                #https://core.telegram.org/bots/api#setwebhook
DEBUG
DOMAIN          #bot's URL
TELEGRAM_TOKEN
AWS_LAMBDA      #'1' if use lambda
FIREBASE_TOKEN  #RUN 'go run main.go -tokenFile ./firebase_token_file.json'
```

## build and upload to lamdba

```bash

make build upload

```

## set commands (optional)

chat with @BotFather

send command `/setcommands`, select the bot

send

```text

new - Add new channel
list - List my channel
token - Get channel token
myid - Show my chat ID
follow - follow channel
unfollow - unfollow channel

```

## Usage

GET

`curl https://[SERVER_URL]/send/[channelID]]/[channelToken]/[Message_body]`

POST

`curl -X POST --data "[Message_body]" https://[SERVER_URL]/send/[channelID]]/[channelToken]`

[Message_body] string or base64 string.
