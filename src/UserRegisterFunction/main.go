package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// for dev
const AWS_REGION = "ap-northeast-1"
const DYNAMO_ENDPOINT = "http://dynamodb:8000"

// for prod
// 本番環境にアップする時はこっちに切り替える
// const DYNAMO_ENDPOINT = "https://dynamodb.ap-northeast-1.amazonaws.com"

const TABLE_NAME = "vitaminDback-userGroup-EPWXXRQCUDMA"

type User struct {
    UserName  string `dynamodbav:"userName" json:userName`
    GroupName string `dynamodbav:"groupName" json:groupName`
	RegisterDate string `dynamodbav:"registerDate" json:registerDate`
}

// Response Lambdaが返答するデータ
type Response struct {
    RequestMethod string `json:RequestMethod`
    Result        string   `json:Result`
}

// リクエストボディをUser構造体に変換&登録日時を追加
func setUser(user *User, request events.APIGatewayProxyRequest) error {

	err := json.Unmarshal([]byte(request.Body), &user)

	if err != nil {
		fmt.Println("error in request body marshal")
		return err
    }

	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	now := time.Now().In(jst)

	user.RegisterDate = now.Format("2006-01-02T15:04:05+09:00")

	fmt.Println(user)

	return nil
}

// dynamodbにデータを挿入
func putItemToDynamoDB(user *User) error {
	// dynamodbのエンドポイントを指定
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == dynamodb.ServiceID && region == AWS_REGION {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           DYNAMO_ENDPOINT,
				SigningRegion: AWS_REGION,
			}, nil
		}
		// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithEndpointResolverWithOptions(customResolver))

    if err != nil {
		fmt.Println("error in cfg")
        return err
    }

    db := dynamodb.NewFromConfig(cfg)

	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		fmt.Println("error in item marshalmap")
        return err
    }

	fmt.Println(item)

	// データを挿入
	_, err = db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(TABLE_NAME),
		Item: item,
		ConditionExpression: aws.String("attribute_not_exists(userName)"),
	})

	if err != nil {
		fmt.Println("error in item dynamodb putitem")
        return err
    }

	return nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    method := request.HTTPMethod
    var user User

	// リクエストボディをUser構造体に変換&登録日時を追加
	err := setUser(&user, request)

	if err != nil {
		fmt.Println("error in setUser function")
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
    }

	fmt.Println(DYNAMO_ENDPOINT)

	// dynamodbにデータを挿入
	err = putItemToDynamoDB(&user)

	// HTTPエラー応答を直接返す
	if err != nil {
		fmt.Println("error in putItemToDynamoDB function")
        return events.APIGatewayProxyResponse{
            Body:       "user already exists",
            StatusCode: 401,
        }, nil
    }

    // httpレスポンス作成
    res := Response{
        RequestMethod: method,
        Result:        "success",
    }
    jsonBytes, _ := json.Marshal(res)

    return events.APIGatewayProxyResponse{
        Body:       string(jsonBytes),
        StatusCode: 200,
    }, nil
}

func main() {
	lambda.Start(handler)
}
