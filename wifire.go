package wifire

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type WiFire struct {
	token        string
	tokenExpires time.Time
	config       Config
}

type Config struct {
	Username   string
	Password   string
	CognitoURL string
	URL        string
	Client     string
}

var defaultConfig = Config{
	CognitoURL: "https://cognito-idp.us-west-2.amazonaws.com/",
	URL:        "https://1ywgyc65d1.execute-api.us-west-2.amazonaws.com",
	Client:     "2fuohjtqv1e63dckp5v84rau0j",
}

type requestTokenBody struct {
	AuthFlow       string         `json:"AuthFlow"`
	AuthParameters authParameters `json:"AuthParameters"`
	ClientID       string         `json:"ClientId"`
}

type authParameters struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

type RequestTokenResponse struct {
	AuthenticationResult AuthenticationResult
}

type AuthenticationResult struct {
	AccessToken  string `json:"AccessToken"`
	ExpiresIn    int    `json:"ExpiresIn"`
	IDToken      string `json:"IdToken"`
	RefreshToken string `json:"RefreshToken"`
	TokenType    string `json:"TokenType"`
}

// New returns a new WiFire connection or an error.
func New(cfg Config) (*WiFire, error) {
	w := WiFire{config: defaultConfig}

	if cfg.Username != "" {
		w.config.Username = cfg.Username
	}

	if cfg.Password != "" {
		w.config.Password = cfg.Password
	}

	if cfg.CognitoURL != "" {
		w.config.CognitoURL = cfg.CognitoURL
	}

	if cfg.URL != "" {
		w.config.URL = cfg.URL
	}

	if cfg.Client != "" {
		w.config.Client = cfg.Client
	}

	if err := w.Refresh(); err != nil {
		return nil, err
	}

	return &w, nil

}

func (w *WiFire) Refresh() error {
	body := requestTokenBody{
		AuthFlow: "USER_PASSWORD_AUTH",
		AuthParameters: authParameters{
			Username: w.config.Username,
			Password: w.config.Password,
		},
		ClientID: w.config.Client,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	client := http.Client{}
	req, err := http.NewRequest("POST", w.config.CognitoURL, bytes.NewReader(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Target", "AWSCognitoIdentityProviderService.InitiateAuth")

	t0 := time.Now()

	r, err := client.Do(req)
	if err != nil {
		return err
	}

	resp, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	var auth RequestTokenResponse

	if err := json.Unmarshal(resp, &auth); err != nil {
		return err
	}

	w.token = auth.AuthenticationResult.IDToken
	w.tokenExpires = t0.Add(time.Second * time.Duration(auth.AuthenticationResult.ExpiresIn))

	return nil
}
