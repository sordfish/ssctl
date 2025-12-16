package sunsynk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var SSApiTokenEndpoint = func() string {
	if v := os.Getenv("SS_API_ENDPOINT"); v != "" {
		return v + "/oauth/token/new"
	}
	return "https://api.sunsynk.net/oauth/token/new"
}()

type SSApiTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
	Success bool   `json:"success"`
	Data    struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		TokenExpiry  int    `json:"expires_in"`
		Scope        string `json:"scope"`
	} `json:"data"`
}

type SSApiErrorResponse struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     error  `json:"error"`
	Path      string `json:"path"`
}

func GetAuthToken(user, pass string) (SSApiTokenResponse, error) {

	httpClient := http.Client{
		Timeout: time.Second * 30,
	}

	type PostBody struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		GrantType string `json:"grant_type"`
		ClientId  string `json:"client_id"`
		Source    string `json:"source"`
		AreaCode  string `json:"areaCode"`
	}

	postbody := PostBody{
		Username:  user,
		Password:  pass,
		GrantType: "password",
		ClientId:  "csp-web",
		Source:    "sunsynk",
		AreaCode:  "elinter",
	}

	postbodyJSON, err := json.Marshal(postbody)
	if err != nil {
		return SSApiTokenResponse{}, err
	}

	req, err := http.NewRequest("POST", SSApiTokenEndpoint, bytes.NewBuffer(postbodyJSON))
	if err != nil {
		return SSApiTokenResponse{}, err
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return SSApiTokenResponse{}, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return SSApiTokenResponse{}, err
	}

	d := SSApiTokenResponse{}
	e := SSApiErrorResponse{}

	if res.StatusCode == 200 {
		err := json.Unmarshal(body, &d)
		if err != nil {
			return SSApiTokenResponse{}, err
		}
		return d, nil
	} else {
		err := json.Unmarshal(body, &e)
		if err != nil {
			return SSApiTokenResponse{}, err
		}
		return d, fmt.Errorf("error: %s path: %s status: %d timestamp: %s", e.Error, e.Path, e.Status, e.Path)
	}
}
