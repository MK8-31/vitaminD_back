package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"time"

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

type UserData struct {
	UserName  string `json:"userName"`
	GroupName string `json:"groupName"`
	Rank      int64  `json:"rank"`
	Exp       int64  `json:"exp"`
	Lv        int64  `json:"lv"`
}

// Response Lambdaが返答するデータ
type Response struct {
    Ranking        []UserData   `json:"ranking"`
}

func GetContributeNum(user User) (int64, error) {
	/*
		userName: githubのユーザー名（例： MK8-31)
		registerDate: 登録日（例： 2023-06-01T19:52:21+09:00)

		githubのユーザー名をもとに入力された日付以降のコントリビュート数を合計して出力する

		privateのコントリビュート数を取得したい場合、ユーザー自身にGithubのマイページにあるcontribution settingの
		Private contributionsをON（チェックをいれる）にしてもらう必要がある
	*/
	url := "https://github-contributions-api.deno.dev/" + user.UserName + ".json"

	req, _ := http.NewRequest(http.MethodGet, url, nil)
	client := new(http.Client)
	resp, _ := client.Do(req)

	if resp.StatusCode != 200 {
		err := errors.New("userName is not found in github")
		fmt.Println(err.Error())
		fmt.Println("return not 200 from github")

		return 0, err
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// JSONを構造体にエンコード
	// var response interface{}
	var response map[string]any
	json.Unmarshal(body, &response)

	registrationDate, err := time.Parse(time.RFC3339, user.RegisterDate)
	if err != nil {
		fmt.Println(err)
	}

	// GithubのContributionsはTimeZoneに関係なく日付に依存するため、標準のUTCに変換する
	formattedRegistrationDate := time.Date(registrationDate.Year(), registrationDate.Month(), registrationDate.Day(), 0, 0, 0, 0, time.UTC)

	fmt.Println("registrationDate", formattedRegistrationDate)

	layout := "2006-01-02"

	var sum_contribute float64 = 0
	for _, harfManthData := range response["contributions"].([]interface{}) {
		for _, AdayData := range harfManthData.([]interface{}) {
			date_unformated := AdayData.(map[string]interface{})["date"].(string)
			date, err := time.Parse(layout, date_unformated)
			if err != nil {
				fmt.Println(err)
			}

			// registrationDate <= date
			if !formattedRegistrationDate.After(date) {
				contribute := AdayData.(map[string]interface{})["contributionCount"].(float64)
				sum_contribute = sum_contribute + contribute
			}
		}
	}
	return int64(sum_contribute), nil
}

func calculateLevel(exp int64) int64 {
	return exp / 10
}

func getContributeData(userNameSlice []User) ([]UserData, error) {
	var userDataSlice []UserData
	for _, data := range userNameSlice {
		exp, err := GetContributeNum(data)
		if err != nil {
			return nil, err
		}
		lv := calculateLevel(exp)
		userDataSlice = append(userDataSlice, UserData{UserName: data.UserName, Exp: exp, Lv: lv, GroupName: data.GroupName})
	}
	return userDataSlice, nil
}

func sortWithExp(userDataSlice []UserData) []UserData {
	sort.Slice(userDataSlice, func(i, j int) bool { return userDataSlice[i].Exp > userDataSlice[j].Exp })
	return userDataSlice
}

func addRank(sortedUserDataSlice []UserData) []UserData {
	var rank_i int64 = 0
	var pred_exp int64 = -1
	var rankedUserDataSlice []UserData
	for _, data := range sortedUserDataSlice {
		if pred_exp != data.Exp {
			pred_exp = data.Exp
			rank_i += 1
			data.Rank = rank_i
		} else {
			// 前の人と同じ経験値の場合は同じ順位をつける
			data.Rank = rank_i
		}
		rankedUserDataSlice = append(rankedUserDataSlice, data)
	}
	return rankedUserDataSlice
}

func P(t interface{}) {
	fmt.Println(reflect.TypeOf(t))
}

func getRanking(userNameSlice []User) ([]UserData, error) {
	/*
		入力されたgithubのユーザーデータをもとにランキングを作成する

	*/
	//コントリビュート数を取り出し、経験値とレベルを計算する
	contributeData, err := getContributeData(userNameSlice)

	if err != nil {
		return nil, err
	}

	// 経験値をもとにソートする
	sortedContributeData := sortWithExp(contributeData)

	// ソート結果をもとにランキングを作成する
	ranking := addRank(sortedContributeData)

	return ranking, nil
}

// 特定のグループに所属している全てのユーザーを取得
func getUsersInGroup(userName string) ([]User, error) {
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

    db := dynamodb.NewFromConfig(cfg)

	// ユーザー名をもとにユーザー情報を取得
	// 検索条件を用意
    getParam := &dynamodb.GetItemInput{
        TableName: aws.String("vitaminDback-userGroup-EPWXXRQCUDMA"),
        Key: map[string]types.AttributeValue{
            "userName": &types.AttributeValueMemberS{Value: userName},
        },
    }

    // 検索
    result, err := db.GetItem(context.TODO(), getParam)

    if err != nil {
		fmt.Println("error in search user")
        return nil, err
    }

	// 結果を構造体にパース
    user := User{}
    err = attributevalue.UnmarshalMap(result.Item, &user)
    if err != nil {
		fmt.Println("error in parse user")
        return nil, err
	}

	fmt.Println("user:", user)

	// user.GroupNameを使って同じグループに所属しているユーザーを全て取得
	// クエリを用いてグローバルセカンダリインデックス（GSI)内を検索
	input := &dynamodb.QueryInput{
		TableName: aws.String(TABLE_NAME),
		IndexName: aws.String("GSI-groupName"),
		KeyConditionExpression: aws.String("groupName = :groupNameValue"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":groupNameValue": &types.AttributeValueMemberS{Value: user.GroupName},
		},
	}

	resp, err := db.Query(context.TODO(), input)
	if err != nil {
		fmt.Println("error in parse users")
		return nil, err
	}

    // 結果を構造体にパース
    users := []User{}
    err = attributevalue.UnmarshalListOfMaps(resp.Items, &users)
    if err != nil {
		fmt.Println("error in parse users")
        return nil, err
    }

	return users, nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    pathParam := request.PathParameters["userName"]
	fmt.Println(pathParam)

	fmt.Println(DYNAMO_ENDPOINT)

	// 特定のグループに所属している全てのユーザーを取得
	users, err := getUsersInGroup(pathParam)

	if err != nil {
		fmt.Println("error in getUsersInGroup function")
        return events.APIGatewayProxyResponse{
            Body:       err.Error(),
            StatusCode: 500,
        }, err
    }

	fmt.Println(users)

	// ランキングを作成
	ranking, err := getRanking(users)

	// HTTPエラー応答を直接返す
	if err != nil {
		fmt.Println("error in getRanking function")
        return events.APIGatewayProxyResponse{
            Body:       "existing invalid userName in group",
            StatusCode: 401,
        }, nil
    }

	fmt.Println(ranking)

    // httpレスポンス作成
    res := Response{
        Ranking: ranking,
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
