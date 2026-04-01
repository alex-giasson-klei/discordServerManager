AGENT_R2_KEY=bin/gameserver-agent

buildLambda:
	GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap ./lambda
buildGameserverAgent:
	GOOS=linux GOARCH=amd64 go build -o gameserver-agent ./cmd/gameserver
uploadGameserverAgent: buildGameserverAgent
	AWS_ACCESS_KEY_ID=$$R2_ACCESS_KEY_ID \
	AWS_SECRET_ACCESS_KEY=$$R2_SECRET_ACCESS_KEY \
	aws s3 cp gameserver-agent s3://$$R2_BUCKET/bin/gameserver-agent \
		--endpoint-url https://$$R2_ACCOUNT_ID.r2.cloudflarestorage.com
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
