package main

import (
	"common"
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
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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

type ErrUserNameNotFound struct {
	userName string
}

func (e *ErrUserNameNotFound) Error() string {
	return "userName not found"
}

func GetContributeNum(user common.User) (int64, error) {
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

func getContributeData(userNameSlice []common.User) ([]UserData, error) {
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

func getRanking(userNameSlice []common.User) ([]UserData, error) {
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

// groupNameを使って同じグループに所属しているユーザーを全て取得
func getUsersInGroupFromGroupName(groupName string, client *dynamodb.Client) ([]common.User, error) {
	// クエリを用いてグローバルセカンダリインデックス（GSI)内を検索
	input := &dynamodb.QueryInput{
		TableName: aws.String(common.TABLE_NAME),
		IndexName: aws.String("GSI-groupName"),
		KeyConditionExpression: aws.String("groupName = :groupNameValue"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":groupNameValue": &types.AttributeValueMemberS{Value: groupName},
		},
	}

	resp, err := client.Query(context.TODO(), input)
	if err != nil {
		fmt.Println("error in parse users")
		return nil, err
	}

    // 結果を構造体にパース
    users := []common.User{}
    err = attributevalue.UnmarshalListOfMaps(resp.Items, &users)
    if err != nil {
		fmt.Println("error in parse users")
        return nil, err
    }

	return users, nil
}

// ユーザー名からユーザーが所属しているgroupNameを取得
func getGroupName(userName string, client *dynamodb.Client) (string, error) {
	// 検索条件を用意
    getParam := &dynamodb.GetItemInput{
        TableName: aws.String("vitaminDback-userGroup-EPWXXRQCUDMA"),
        Key: map[string]types.AttributeValue{
            "userName": &types.AttributeValueMemberS{Value: userName},
        },
    }

    // 検索
    result, err := client.GetItem(context.TODO(), getParam)

    if err != nil {
		fmt.Println("error in search user")
        return "", err
    }

	// 結果を構造体にパース
    user := common.User{}
    err = attributevalue.UnmarshalMap(result.Item, &user)
    if err != nil {
		fmt.Println("error in parse user")
        return "", err
	}

	fmt.Println("user:", user)

	return user.GroupName, nil
}

// 特定のグループに所属している全てのユーザーを取得
func getUsersInGroup(userName string) ([]common.User, error) {
	// DynamoDBのクライアントを作成
	client, err := common.CreateDynamoDBClient()

	if err != nil {
		fmt.Println("error in CreateDynamoDBClient function")
		return nil, err
	}

	// ユーザー名をもとにユーザー情報を取得
	groupName, err := getGroupName(userName, client)

	if err != nil {
		fmt.Println("error in getGroupName function")
		return nil, err
	}

	// userName（ユーザー）が見つからない場合
	if groupName == "" {
		fmt.Println("not found userName in userGroup table")
		return nil, &ErrUserNameNotFound{userName}
	}

	fmt.Println("groupName:", groupName)

	// groupNameを使って同じグループに所属しているユーザーを全て取得
	users, err := getUsersInGroupFromGroupName(groupName, client)

	if err != nil {
		fmt.Println("error in getUsersInGroupFromGroupName function")
		return nil, err
	}

	return users, nil
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    pathParam := request.PathParameters["userName"]
	fmt.Println(pathParam)

	// 特定のグループに所属している全てのユーザーを取得
	users, err := getUsersInGroup(pathParam)

	if err != nil {
		fmt.Println("error in getUsersInGroup function")
		switch err.(type) {
			// userName（ユーザー）が見つからない場合は401エラーを返す
			case *ErrUserNameNotFound:
				return events.APIGatewayProxyResponse{
					Body:       err.Error(),
					StatusCode: 401,
				}, nil

			default:
				return events.APIGatewayProxyResponse{
					Body:       err.Error(),
					StatusCode: 500,
				}, err
		}
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
