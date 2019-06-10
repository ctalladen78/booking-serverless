package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	gMuxAdapter "github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/gorilla/mux"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

var muxLambda *gMuxAdapter.GorillaMuxAdapter

func init() {
	log.Println("Mux Cold Start")

	r := mux.NewRouter()
	r.HandleFunc("/agent", GetAgent)

	muxLambda = gMuxAdapter.New(r)
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return muxLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(handler)

}

type Item struct {
	AgentId  string `json:"agentId"`
	TicketId string `json:"ticketId"`
}

func GetAgent(w http.ResponseWriter, r *http.Request) {
	tableName := os.Getenv("DATABASE_TABLE")
	log.Printf("%v", r)
	apiGwContext, err := muxLambda.GetAPIGatewayContext(r)
	if err != nil {
		log.Println(err.Error())
		fmt.Fprint(w, err.Error())
		return
	}
	userId := apiGwContext.Identity.CognitoIdentityID

	sess := session.Must(session.NewSession())

	svc := dynamodb.New(sess)

	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"AgentId": {
				S: aws.String(userId),
			},
			"TicketId": {
				S: aws.String(r.URL.Query().Get("ticketId")),
			},
		},
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeInternalServerError:
				log.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
				return
			default:
				log.Println(err.Error())
				return
			}
		} else {
			log.Println(err.Error())
			return
		}
	}

	item := Item{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &item)
	if err != nil {
		log.Println(err.Error())
		return
	}

	fmt.Fprintf(w, "Result: %v", item)
}
