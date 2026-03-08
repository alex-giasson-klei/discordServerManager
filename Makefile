buildLambda:
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap ./lambda
registerCommands:
	SECRET_STORE_REGION=us-west-2 SECRET_STORE_ID=discordServerManagerBot go run ./registerCommands
package:
	zip function.zip bootstrap
updateAWS:
	aws lambda update-function-code --function-name discordGameServerBot \
    --zip-file fileb://function.zip \
    --publish
deploy: buildLambda registerCommands package updateAWS
invoke:
	aws lambda invoke --function-name discordGameServerBot \
	--cli-binary-format raw-in-base64-out \
	--payload '{"foo":"bar"}' \
	response.json
