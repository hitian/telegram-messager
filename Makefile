.PHONY: clean build

FUNCTION_NAME=telegram-bot

clean: 
	rm -rf ./main ./upload.zip
	
build:
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.build=`date +'%Y-%m-%d_%H_%M_%S'`" -o ./main .

upload:
	zip -X -r ./upload.zip ./main
	aws lambda update-function-code --function-name ${FUNCTION_NAME} --zip-file fileb://upload.zip