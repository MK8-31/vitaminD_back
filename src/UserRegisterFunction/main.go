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

	// DynamoDBのクライアントを作成
	client, err := common.CreateDynamoDBClient()

	// データを挿入
	_, err = client.PutItem(context.TODO(), &dynamodb.PutItemInput{
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
    var user common.User

	// リクエストボディをUser構造体に変換&登録日時を追加
	err := setUser(&user, request)

	if err != nil {
		fmt.Println("error in setUser function")
        return events.APIGatewayProxyResponse{
			Headers: common.ORIGIN_HEADERS,
			Body: common.CreateResponseBody(err.Error()),
            StatusCode: 500,
        }, err
    }

	// userNameとgroupNameが空の場合はエラーを返す
	if user.UserName == "" || user.GroupName == "" {
		fmt.Println("userName or groupName is empty")
		return events.APIGatewayProxyResponse{
			Headers: common.ORIGIN_HEADERS,
			Body: common.CreateResponseBody("userName or groupName is empty"),
            StatusCode: 401,
        }, nil
	}

	// TODO: GitHubのアクセストークンで確認するように変更する
	// GitHubにアクセスしてユーザー名が存在するか確認
	err = common.AccessGitHubWithUserName(user.UserName)

	// userNameがGitHubにない場合はエラーを返す
	if err != nil {
		fmt.Println(err.Error())
		return events.APIGatewayProxyResponse{
			Headers: common.ORIGIN_HEADERS,
			Body: common.CreateResponseBody(err.Error()),
            StatusCode: 401,
        }, nil
	}

	// dynamodbにデータを挿入
	err = putItemToDynamoDB(&user)

	// HTTPエラー応答を直接返す
	if err != nil {
		fmt.Println("error in putItemToDynamoDB function")
        return events.APIGatewayProxyResponse{
			Headers: common.ORIGIN_HEADERS,
			Body: common.CreateResponseBody("userName already exists"),
            StatusCode: 401,
        }, nil
    }

    return events.APIGatewayProxyResponse{
		Headers: common.ORIGIN_HEADERS,
        Body: common.CreateResponseBody("success"),
        StatusCode: 200,
    }, nil
}

func main() {
	lambda.Start(handler)
}
