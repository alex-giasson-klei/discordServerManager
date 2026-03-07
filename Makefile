build:
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap
package:
	zip function.zip bootstrap
updateAWS:
	aws lambda update-function-code --function-name discordGameServerBot \
    --zip-file fileb://function.zip \
    --publish
deploy: build package updateAWS
invoke:
	aws lambda invoke --function-name discordGameServerBot \
	--cli-binary-format raw-in-base64-out \
	--payload '{"foo":"bar"}' \
	response.json
