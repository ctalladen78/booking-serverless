package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"encoding/json"

	core "github.com/awslabs/aws-lambda-go-api-proxy/core"
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
	r.HandleFunc("/agent", GetItem)

	muxLambda = gMuxAdapter.New(r)
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return muxLambda.ProxyWithContext(ctx, req)
}

func main() {
	lambda.Start(handler)

}

type Item struct {
	ItemId  string `json:"itemId"`
	TicketId string `json:"ticketId"`
}

func GetItem(w http.ResponseWriter, r *http.Request) {
	tableName := os.Getenv("DATABASE_TABLE")
	log.Printf("%v", r)
	apiGwContext, ok := core.GetAPIGatewayContextFromContext(r.Context())
	if !ok {
		log.Println("Api Gateway Context Not Found!")
		return
	}
	userId := apiGwContext.Identity.CognitoIdentityID

	sess := session.Must(session.NewSession())

	svc := dynamodb.New(sess)

	result, err := svc.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"ItemId": {
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

	b, err := json.Marshal(&item)
	if err != nil {
		log.Printf("Couldn't marshal dynamodb query results: %s", err)
		return
	}
	fmt.Fprintf(w, "Result: %s", b)
}
