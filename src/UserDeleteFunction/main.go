package main

import (
	"common"
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// dynamodbにあるユーザーのデータを削除する
func deleteUser(userName string) error {
	// DynamoDBのクライアントを作成
	client, err := common.CreateDynamoDBClient()

	// 検索条件を用意
	deleteParam := &dynamodb.DeleteItemInput{
		TableName: aws.String(common.TABLE_NAME),
		Key: map[string]types.AttributeValue{
			"userName": &types.AttributeValueMemberS{Value: userName},
		},
		ConditionExpression: aws.String("attribute_exists(userName)"),
	}

	// データを削除
	// userNameがDBにない場合はエラーを返す
	_, err = client.DeleteItem(context.TODO(), deleteParam)

	if err != nil {
		fmt.Println("error in item dynamodb DeleteItem")
		fmt.Println(err.Error())
        return err
    }

	return nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	pathParam := request.PathParameters["userName"]
	fmt.Println(pathParam)

	// dynamodbにデータを挿入
	err := deleteUser(pathParam)

	// HTTPエラー応答を直接返す
	if err != nil {
		fmt.Println("error in deleteUser function")
        return events.APIGatewayProxyResponse{
            Body:       "userName not found",
            StatusCode: 401,
        }, nil
    }

    return events.APIGatewayProxyResponse{
        Body:       "success",
        StatusCode: 200,
    }, nil
}

func main() {
	lambda.Start(handler)
}
