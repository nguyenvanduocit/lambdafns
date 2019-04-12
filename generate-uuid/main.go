package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	iUuid, err := uuid.NewUUID()
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	return events.APIGatewayProxyResponse{
		Headers: map[string]string{
			" Content-type": "text/plain",
		},
		Body:       iUuid.String(),
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
