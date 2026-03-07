package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type Input struct{}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	resp := events.APIGatewayProxyResponse{
		StatusCode:        400,
		Headers:           nil,
		MultiValueHeaders: nil,
		Body:              "Hello!",
		IsBase64Encoded:   false,
	}
	return resp, nil
}

func main() {
	lambda.Start(handler)
}
