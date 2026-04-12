-include .env
export

buildLambda:
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap ./cmd/lambda
buildGameserverAgent:
	GOOS=linux GOARCH=amd64 go build -o gameserver-agent ./cmd/gameserver
uploadGameserverAgent: buildGameserverAgent
	aws s3 cp gameserver-agent s3://$(R2_BUCKET)/bin/gameserver-agent \
		--endpoint-url https://$(R2_ACCOUNT_ID).r2.cloudflarestorage.com \
		--region auto
registerCommands:
	SECRET_STORE_REGION=us-west-2 SECRET_STORE_ID=discordServerManagerBot go run ./cmd/registerCommands
package:
	zip function.zip bootstrap
updateAWS:
	aws lambda update-function-code --function-name discordGameServerBot \
    --zip-file fileb://function.zip \
    --publish \
    --no-cli-pager \
    --query '{FunctionName: FunctionName, Version: Version, LastUpdateStatus: LastUpdateStatus}' \
    --profile ajgia
	aws lambda put-function-event-invoke-config --function-name discordGameServerBot \
    --maximum-retry-attempts 0 \
    --no-cli-pager \
    --profile ajgia
deploy: buildLambda registerCommands package updateAWS
invoke:
	aws lambda invoke --function-name discordGameServerBot \
	--cli-binary-format raw-in-base64-out \
	--payload '{"foo":"bar"}' \
	response.json
