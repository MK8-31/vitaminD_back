package main

import (
	"common"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Response Lambdaが返答するデータ
type Response struct {
    RequestMethod string `json:RequestMethod`
    Result        string   `json:Result`
}

// リクエストボディをUser構造体に変換&登録日時を追加
func setUser(user *common.User, request events.APIGatewayProxyRequest) error {

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
func putItemToDynamoDB(user *common.User) error {
	// 挿入するデータをマッピング
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		fmt.Println("error in item marshalmap")
        return err
    }

	fmt.Println(item)

	// DynamoDBに接続
	db, err := common.ConnectDynamoDB()

	// データを挿入
	_, err = db.PutItem(context.TODO(), &dynamodb.PutItemInput{
		TableName: aws.String(common.TABLE_NAME),
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
    var user common.User

	// リクエストボディをUser構造体に変換&登録日時を追加
	err := setUser(&user, request)

	if err != nil {
		fmt.Println("error in setUser function")
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
    }

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
