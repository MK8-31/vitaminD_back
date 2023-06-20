package common

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
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

// DynamoDBのクライアントを作成
func CreateDynamoDBClient() (*dynamodb.Client, error) {
	fmt.Println("ConnectDynamnoDB", DYNAMO_ENDPOINT)

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
        return nil, err
    }

    client := dynamodb.NewFromConfig(cfg)

	return client, nil
}
