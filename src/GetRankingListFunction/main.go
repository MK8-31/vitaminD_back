package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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
    Result        []User   `json:Result`
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    method := request.HTTPMethod
    pathparam := request.PathParameters["groupName"]
	fmt.Println(pathparam)

	fmt.Println(DYNAMO_ENDPOINT)

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
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
    }

    db := dynamodb.NewFromConfig(cfg)

	// クエリを用いてグローバルセカンダリインデックス（GSI)を検索
	input := &dynamodb.QueryInput{
		TableName: aws.String(TABLE_NAME),
		IndexName: aws.String("GSI-groupName"),
		KeyConditionExpression: aws.String("groupName = :groupNameValue"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":groupNameValue": &types.AttributeValueMemberS{Value: pathparam},
		},
	}

	resp, err := db.Query(context.TODO(), input)
	if err != nil {
		fmt.Println(err)
		return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
	}

	fmt.Println(resp)

	for _, item := range resp.Items {
		fmt.Println(item)
	}


    // 結果を構造体にパース
    users := []User{}
    err = attributevalue.UnmarshalListOfMaps(resp.Items, &users)
    if err != nil {
		fmt.Println("error in parse")
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
    }

	fmt.Println(users)

    // httpレスポンス作成
    res := Response{
        RequestMethod: method,
        Result:        users,
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
