package sunsynk

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var SSApiNewTokenEndpoint = func() string {
	if v := os.Getenv("SS_API_ENDPOINT"); v != "" {
		return v + "/oauth/token/new"
	}
	return "https://api.sunsynk.net/oauth/token/new"
}()

type SSApiNewTokenResponse struct {
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

type SSApiNewErrorResponse struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     error  `json:"error"`
	Path      string `json:"path"`
}

// Matches python: response.json()['data']
type publicKeyResponse struct {
	Data string `json:"data"`
}

func nonceMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func loadRSAPublicKeyFromBareString(bare string) (*rsa.PublicKey, error) {
	// Python wraps it like:
	// -----BEGIN PUBLIC KEY-----
	// <bare>
	// -----END PUBLIC KEY-----
	pemStr := "-----BEGIN PUBLIC KEY-----\n" + bare + "\n-----END PUBLIC KEY-----\n"
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to PEM decode public key")
	}

	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return pub, nil
}

func GetNewAuthToken(user, pass string) (SSApiNewTokenResponse, error) {
	httpClient := http.Client{
		Timeout: time.Second * 30,
	}

	// Derive API server host from SSApiNewTokenEndpoint
	tokenURL, err := url.Parse(SSApiNewTokenEndpoint)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}
	apiServer := tokenURL.Host

	source := "sunsynk"

	publicKeyNonce := nonceMillis()
	publicKeySigInput := fmt.Sprintf("nonce=%d&source=%sPOWER_VIEW", publicKeyNonce, source)
	publicKeySign := md5Hex(publicKeySigInput)

	publicKeyURL := url.URL{
		Scheme: tokenURL.Scheme,
		Host:   apiServer,
		Path:   "/anonymous/publicKey",
	}
	q := publicKeyURL.Query()
	q.Set("source", source)
	q.Set("nonce", fmt.Sprintf("%d", publicKeyNonce))
	q.Set("sign", publicKeySign)
	publicKeyURL.RawQuery = q.Encode()

	reqPK, err := http.NewRequest("GET", publicKeyURL.String(), nil)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}

	resPK, err := httpClient.Do(reqPK)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}
	defer resPK.Body.Close()

	pkBody, err := io.ReadAll(resPK.Body)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}
	if resPK.StatusCode < 200 || resPK.StatusCode > 299 {
		return SSApiNewTokenResponse{}, fmt.Errorf("publicKey http %d: %s", resPK.StatusCode, string(pkBody))
	}

	var pkResp publicKeyResponse
	if err := json.Unmarshal(pkBody, &pkResp); err != nil {
		return SSApiNewTokenResponse{}, err
	}
	publicKeyString := strings.TrimSpace(pkResp.Data)
	if publicKeyString == "" {
		return SSApiNewTokenResponse{}, fmt.Errorf("publicKey response missing data")
	}

	pubKey, err := loadRSAPublicKeyFromBareString(publicKeyString)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}

	encBytes, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, []byte(pass))
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}
	encryptedPassword := base64.StdEncoding.EncodeToString(encBytes)

	tokenNonce := nonceMillis()

	first10 := publicKeyString
	if len(first10) > 10 {
		first10 = first10[:10]
	}
	tokenSignString := fmt.Sprintf("nonce=%d&source=sunsynk%s", tokenNonce, first10)
	tokenSign := md5Hex(tokenSignString)

	type PostBody struct {
		ClientId  string `json:"client_id"`
		GrantType string `json:"grant_type"`
		Password  string `json:"password"`
		Source    string `json:"source"`
		Username  string `json:"username"`
		Nonce     int64  `json:"nonce"`
		Sign      string `json:"sign"`
	}

	postbody := PostBody{
		Username:  user,
		Password:  encryptedPassword,
		GrantType: "password",
		ClientId:  "csp-web",
		Source:    source,
		Nonce:     tokenNonce,
		Sign:      tokenSign,
	}

	postbodyJSON, err := json.Marshal(postbody)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}

	req, err := http.NewRequest("POST", SSApiNewTokenEndpoint, bytes.NewBuffer(postbodyJSON))
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := httpClient.Do(req)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return SSApiNewTokenResponse{}, err
	}

	d := SSApiNewTokenResponse{}
	e := SSApiNewErrorResponse{}

	if res.StatusCode == 200 {
		if err := json.Unmarshal(body, &d); err != nil {
			return SSApiNewTokenResponse{}, err
		}
		if d.Message != "Success" || d.Data.AccessToken == "" {
			return d, fmt.Errorf("login failed: msg=%q", d.Message)
		}
		return d, nil
	}

	// best-effort parse error body (your old struct has `Error error` which rarely unmarshals nicely)
	_ = json.Unmarshal(body, &e)
	return d, fmt.Errorf("token http %d: %s", res.StatusCode, string(body))
}
