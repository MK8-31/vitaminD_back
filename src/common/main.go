package common

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// for dev
const AWS_REGION = "ap-northeast-1"
// const DYNAMO_ENDPOINT = "http://dynamodb:8000"

// for prod
// 本番環境にアップする時はこっちに切り替える
const DYNAMO_ENDPOINT = "https://dynamodb.ap-northeast-1.amazonaws.com"

const TABLE_NAME = "vitaminDback-userGroup-EPWXXRQCUDMA"

var ORIGIN_HEADERS = map[string]string{
	"Access-Control-Allow-Headers" : "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
	"Content-Type": "application/json",
	"Access-Control-Allow-Origin": "https://demetara.vercel.app",
	"Access-Control-Allow-Methods": "OPTIONS,POST,GET,PUT,DELETE",
}

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

// GitHubにアクセスしてユーザー名が存在するか確認する
func AccessGitHubWithUserName(userName string) error {
	urlAddress := "https://github-contributions-api.deno.dev/" + userName + ".text"

	req, _ := http.NewRequest(http.MethodGet, urlAddress, nil)
	client := new(http.Client)
	resp, _ := client.Do(req)

	if resp.StatusCode != 200 {
		err := errors.New("userName is not found in GitHub")

		return err
	}

	return nil
}
