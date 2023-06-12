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

type UserGroup struct {
    UserName  string `dynamodbav:"user_name" json:userName`
    GroupName string `dynamodbav:"group_name" json:groupName`
}

// Response Lambdaが返答するデータ
type Response struct {
    RequestMethod string `json:RequestMethod`
    Result        UserGroup   `json:Result`
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    method := request.HTTPMethod
    pathparam := request.PathParameters["groupName"]
	fmt.Println(pathparam)

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
        panic(err)
    }

    db := dynamodb.NewFromConfig(cfg)


    // 検索条件を用意
    getParam := &dynamodb.GetItemInput{
        TableName: aws.String("vitaminDback-userGroup-1FD8R77KUXU3S"),
        Key: map[string]types.AttributeValue{
            "user_name": &types.AttributeValueMemberS{Value: pathparam},
			"group_name": &types.AttributeValueMemberS{Value: "vitaminD"},
        },
    }

    // 検索
    result, err := db.GetItem(context.TODO(), getParam)
	// fmt.Println(err.Error())
    if err != nil {
		fmt.Println("error in search")
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 404,
        }, err
    }

	fmt.Println(result)


    // 結果を構造体にパース
    userGroup := UserGroup{}
    err = attributevalue.UnmarshalMap(result.Item, &userGroup)
    if err != nil {
		fmt.Println("error in parse")
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
    }

	fmt.Println(userGroup)

    // httpレスポンス作成
    res := Response{
        RequestMethod: method,
        Result:        userGroup,
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
