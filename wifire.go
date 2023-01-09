// Package wifire implements a client for connecting to the Traeger REST and
// MQTT APIs. The goal is to support temperature monitoring with a potential
// longterm goal of controlling the grill.
package wifire

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// WiFire is a handle for the WiFire API connection.
type WiFire struct {
	token        string
	tokenExpires time.Time
	config       config
}

type config struct {
	username   string
	password   string
	cognitoURL string
	baseURL    string
	clientID   string
}

var defaultConfig = config{
	cognitoURL: "https://cognito-idp.us-west-2.amazonaws.com/",
	baseURL:    "https://1ywgyc65d1.execute-api.us-west-2.amazonaws.com",
	clientID:   "2fuohjtqv1e63dckp5v84rau0j",
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

type requestTokenResponse struct {
	AuthenticationResult authenticationResult
}

type authenticationResult struct {
	AccessToken  string `json:"AccessToken"`
	ExpiresIn    int    `json:"ExpiresIn"`
	IDToken      string `json:"IdToken"`
	RefreshToken string `json:"RefreshToken"`
	TokenType    string `json:"TokenType"`
}

// Credentials is an option setting function for New(). It sets the user and
// password credentials for logging into the API. These are the same values used
// by the Traeger App.
func Credentials(username, password string) func(*WiFire) {
	return func(w *WiFire) {
		w.config.username = username
		w.config.password = password
	}
}

// ClientID is an option setting function for New(). It sets the client
// identifier for the WiFire API. This should be set to the ID of the Traeger
// App.
func ClientID(id string) func(*WiFire) {
	return func(w *WiFire) {
		w.config.clientID = id
	}
}

// URLs is an option setting function for New(). It sets the WiFire API URLs
// used to pull the user information and obtain a token.
func URLs(base, cognito string) func(*WiFire) {
	return func(w *WiFire) {
		w.config.baseURL = base
		w.config.cognitoURL = cognito
	}
}

// New returns a new WiFire connection or an error.
func New(opts ...func(*WiFire)) (*WiFire, error) {
	w := WiFire{config: defaultConfig}

	for _, o := range opts {
		o(&w)
	}

	if err := w.refresh(); err != nil {
		return nil, err
	}

	return &w, nil

}

func (w *WiFire) refresh() error {
	body := requestTokenBody{
		AuthFlow: "USER_PASSWORD_AUTH",
		AuthParameters: authParameters{
			Username: w.config.username,
			Password: w.config.password,
		},
		ClientID: w.config.clientID,
	}

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	client := http.Client{}
	req, err := http.NewRequest("POST", w.config.cognitoURL, bytes.NewReader(b))
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

	defer r.Body.Close()

	resp, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	var auth requestTokenResponse

	if err := json.Unmarshal(resp, &auth); err != nil {
		return err
	}

	w.token = auth.AuthenticationResult.IDToken
	w.tokenExpires = t0.Add(time.Second * time.Duration(auth.AuthenticationResult.ExpiresIn))

	return nil
}
