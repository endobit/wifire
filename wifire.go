// Package wifire implements a client for connecting to the Traeger REST and
// MQTT APIs. The goal is to support temperature monitoring with a potential
// longterm goal of controlling the grill.
package wifire

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/golang-jwt/jwt/v5"

	"endobit.io/app/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

// Client is a handle for the Client API connection.
type Client struct {
	logger       *slog.Logger
	idToken      string
	refreshToken string
	username     string
	password     string
	baseURL      string
	region       string
	cognito      *cognitoidentityprovider.Client
	clientID     string
}

var (
	defaultBaseURL  = "https://1ywgyc65d1.execute-api.us-west-2.amazonaws.com"
	defaultClientID = "2fuohjtqv1e63dckp5v84rau0j" // Traeger App ID
	defaultRegion   = "us-west-2"
)

// WithLogger is an option setting function for New(). It sets the logger used
// by the WiFire client.
func WithLogger(logger *slog.Logger) func(*Client) {
	return func(w *Client) {
		w.logger = logger
	}
}

// Credentials is an option setting function for New(). It sets the user and
// password credentials for logging into the API. These are the same values used
// by the Traeger App.
func Credentials(username, password string) func(*Client) {
	return func(w *Client) {
		w.username = username
		w.password = password
	}
}

// ClientID is an option setting function for New(). It sets the client
// identifier for the WiFire API. This should be set to the ID of the Traeger
// App.
func ClientID(id string) func(*Client) {
	return func(w *Client) {
		w.clientID = id
	}
}

// URLs is an option setting function for New(). It sets the WiFire API URLs
// used to pull the user information and obtain a token.
func URLs(base string) func(*Client) {
	return func(w *Client) {
		w.baseURL = base
	}
}

// NewClient returns a new WiFire connection or an error.
func NewClient(opts ...func(*Client)) (*Client, error) {
	client := Client{
		logger:   slog.New(slog.DiscardHandler),
		baseURL:  defaultBaseURL,
		clientID: defaultClientID,
		region:   defaultRegion,
	}

	for _, o := range opts {
		o(&client)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	cfg.Region = client.region
	client.cognito = cognitoidentityprovider.NewFromConfig(cfg)

	if err := client.login(); err != nil {
		return nil, err
	}

	return &client, nil
}

// UserData fetches the /prod/users/self information from the WiFire API.
func (c *Client) UserData() (*GetUserDataResponse, error) {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/prod/users/self", http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", c.idToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user data %d", resp.StatusCode)
	}

	if err := c.logResponseBody("prod/users/self", resp); err != nil {
		return nil, err
	}

	var data GetUserDataResponse

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return &data, nil
}

func (c *Client) MQTT() (mqtt.Client, error) {
	client := http.Client{}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/prod/mqtt-connections", http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", c.idToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch mqtt connection: %d", resp.StatusCode)
	}

	if err := c.logResponseBody("prod/mqtt-connections", resp); err != nil {
		return nil, err
	}

	var data getMQTTResponse

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	c.logger.Info("mqtt connection", "expires", time.Unix(int64(data.ExpiresAt), 0))

	opts := mqtt.NewClientOptions()
	opts.AddBroker(data.SignedURL)
	opts.OnConnectionLost = c.mqttConnectionLost

	return mqtt.NewClient(opts), nil
}

// logResponseBody makes a copy of the response body and logs it. Because it
// consumes the body it duplicates it and replaces the original. It also closes
// the body it consumes.
func (c *Client) logResponseBody(name string, resp *http.Response) error {
	ctx := context.Background()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}()

	var msg map[string]any

	if err := json.Unmarshal(body, &msg); err != nil {
		c.logger.Log(ctx, log.LevelTrace, "rx", "endpoint", name, "body", string(body))

		return nil
	}

	return nil
}

func (c *Client) mqttConnectionLost(_ mqtt.Client, _ error) {
	// TODO: find a way to trigger this and figure out what to do.
	c.logger.Error("connection lost callback")
}

func (c *Client) refresh() error { //nolint:unused
	input := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeRefreshTokenAuth,
		ClientId: aws.String(c.clientID),
		AuthParameters: map[string]string{
			"REFRESH_TOKEN": c.refreshToken,
		},
	}

	resp, err := c.cognito.InitiateAuth(context.Background(), input)
	if err != nil {
		return fmt.Errorf("cannot refresh failed: %w", err)
	}

	auth := resp.AuthenticationResult

	if auth.IdToken == nil {
		return errors.New("no ID token in authentication result")
	}

	c.idToken = aws.ToString(resp.AuthenticationResult.IdToken)

	return logTokenInfo(c.logger, "ID token", c.idToken)
}

func (c *Client) login() error {
	input := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(c.clientID),
		AuthParameters: map[string]string{
			"USERNAME": c.username,
			"PASSWORD": c.password,
		},
	}

	resp, err := c.cognito.InitiateAuth(context.Background(), input)
	if err != nil {
		return fmt.Errorf("cannot initiate auth: %w", err)
	}

	auth := resp.AuthenticationResult

	if auth.IdToken == nil {
		return errors.New("no ID token in authentication result")
	}

	if auth.RefreshToken == nil {
		return errors.New("no refresh token in authentication result")
	}

	c.idToken = *auth.IdToken
	c.refreshToken = *auth.RefreshToken // opaque, not JWT

	return logTokenInfo(c.logger, "id token", c.idToken)
}

func logTokenInfo(logger *slog.Logger, name, token string) error {
	tokenData, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", name, err)
	}

	claims, ok := tokenData.Claims.(jwt.MapClaims)
	if !ok {
		return fmt.Errorf("invalid claims format in %s", name)
	}

	if exp, ok := claims["exp"].(float64); ok {
		expires := time.Unix(int64(exp), 0)
		logger.Info(name, "expires", expires)
	}

	return nil
}
