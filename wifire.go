// Package wifire implements a client for connecting to the Traeger REST and
// MQTT APIs. The goal is to support temperature monitoring with a potential
// longterm goal of controlling the grill.
package wifire

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
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
	logger          *slog.Logger
	username        string
	password        string
	baseURL         string
	region          string
	clientID        string
	cognito         *cognitoidentityprovider.Client
	conn            connection
	isMQTTConnected atomic.Bool
}

type connection struct {
	mutex          sync.RWMutex
	idToken        string
	idTokenExpires time.Time
	refreshToken   string
	mqttClient     mqtt.Client
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
	c.conn.mutex.RLock()
	defer c.conn.mutex.RUnlock()

	client := http.Client{}

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/prod/users/self", http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", c.conn.idToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user data %d", resp.StatusCode)
	}

	if err := logResponseBody(c.logger, "prod/users/self", resp); err != nil {
		return nil, err
	}

	var data GetUserDataResponse

	dec := jsontext.NewDecoder(resp.Body)
	if err := json.UnmarshalDecode(dec, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

// MQTTConnect establishes the MQTT connection to the Grill.
func (c *Client) MQTTConnect() error {
	if c.isMQTTConnected.Load() {
		return nil
	}

	// The ID Token has a 60m TTL and mqtt connections are much longer. If a
	// subscription gets disconnected it may need to re-login before
	// reconnecting.

	if err := c.mqttConnect(); err != nil {
		if err := c.login(); err != nil {
			return err
		}

		return c.mqttConnect()
	}

	return nil
}

// MQTTDisconnect closed the MQTT connection to the Grill.
func (c *Client) MQTTDisconnect() {
	c.conn.mutex.RLock()
	defer c.conn.mutex.RUnlock()

	c.conn.mqttClient.Disconnect(0)
}

// MQTTSubscribeStatus subscribes to the prod/thing/update for the grill.
// Updates are pushed to the returned channel.
func (c *Client) MQTTSubscribeStatus(grill string, subscriber chan Status) error {
	c.conn.mutex.RLock()
	defer c.conn.mutex.RUnlock()

	token := c.conn.mqttClient.Subscribe("prod/thing/update/"+grill, 1, func(_ mqtt.Client, m mqtt.Message) {
		var msg map[string]any

		payload := m.Payload()

		if err := json.Unmarshal(payload, &msg); err != nil {
			slog.Error("bad message", "msg", string(payload))
		}

		m.Ack() // doesn't do anything

		c.logger.LogAttrs(context.Background(), log.LevelTrace, "rx",
			slog.Bool("duplicate", m.Duplicate()),
			slog.Any("qos", m.Qos()),
			slog.Bool("retained", m.Retained()),
			slog.String("topic", m.Topic()),
			slog.Any("message_id", m.MessageID()),
			slog.Any("payload", msg))

		subscriber <- newUpdate(payload)
	})

	token.Wait()

	return nil
}

func (c *Client) MQTTIsConnected() bool {
	return c.isMQTTConnected.Load()
}

// login performs the login to the cognito service and obtains an ID token. If
// an ID token already exists use the refresh token to obtain a new one. There
// is no check if the current ID token is expired, it is up to the caller to
// decide when to refresh it.
//
// Caller should monitor service connection and refresh as needed.
func (c *Client) login() error {
	c.conn.mutex.Lock()
	defer c.conn.mutex.Unlock()

	var input *cognitoidentityprovider.InitiateAuthInput

	if c.conn.idToken == "" { // basic auth
		c.logger.Info("logging in with basic auth",
			slog.String("username", c.username),
		)

		input = &cognitoidentityprovider.InitiateAuthInput{
			AuthFlow: types.AuthFlowTypeUserPasswordAuth,
			ClientId: aws.String(c.clientID),
			AuthParameters: map[string]string{
				"USERNAME": c.username,
				"PASSWORD": c.password,
			},
		}
	} else { // refresh token
		c.logger.Info("refreshing id token with refresh token")

		input = &cognitoidentityprovider.InitiateAuthInput{
			AuthFlow: types.AuthFlowTypeRefreshTokenAuth,
			ClientId: aws.String(c.clientID),
			AuthParameters: map[string]string{
				"REFRESH_TOKEN": c.conn.refreshToken,
			},
		}
	}

	resp, err := c.cognito.InitiateAuth(context.Background(), input)
	if err != nil {
		c.conn.idToken = ""

		return fmt.Errorf("cannot initiate auth: %w", err)
	}

	auth := resp.AuthenticationResult

	if auth.IdToken == nil {
		return errors.New("no id token in authentication result")
	}

	c.conn.idToken = *auth.IdToken

	if auth.RefreshToken == nil {
		c.logger.Warn("no refresh token in authentication result")
	} else {
		c.conn.refreshToken = *auth.RefreshToken // opaque, not JWT
	}

	exp, err := tokenInfo(c.conn.idToken)
	if err != nil {
		return fmt.Errorf("id token: %w", err)
	}

	c.conn.idTokenExpires = *exp

	return nil
}

func (c *Client) mqttConnect() error {
	c.conn.mutex.Lock()
	defer c.conn.mutex.Unlock()

	client := http.Client{}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/prod/mqtt-connections", http.NoBody)
	if err != nil {
		return err
	}

	req.Header.Set("authorization", c.conn.idToken)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch mqtt connection: %d", resp.StatusCode)
	}

	if err := logResponseBody(c.logger, "prod/mqtt-connections", resp); err != nil {
		return err
	}

	var data getMQTTResponse

	dec := jsontext.NewDecoder(resp.Body)
	if err := json.UnmarshalDecode(dec, &data); err != nil {
		return err
	}

	c.logger.Info("mqtt connection", "expires", time.Unix(int64(data.ExpiresAt), 0))

	opts := mqtt.NewClientOptions()
	opts.AddBroker(data.SignedURL)
	opts.OnConnect = c.mqttConnectCallback
	opts.OnConnectionLost = c.mqttConnectionLostCallback

	c.conn.mqttClient = mqtt.NewClient(opts)

	if token := c.conn.mqttClient.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (c *Client) mqttConnectCallback(_ mqtt.Client) {
	c.logger.Info("mqtt connection established")
	c.isMQTTConnected.Store(true)
}

func (c *Client) mqttConnectionLostCallback(_ mqtt.Client, _ error) {
	c.logger.Error("connection lost callback")
	c.isMQTTConnected.Store(false)
}

func tokenInfo(token string) (expires *time.Time, err error) {
	tokenData, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := tokenData.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("failed to parse token claims")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, errors.New("no expiration in token claims")
	}

	t := time.Unix(int64(exp), 0)

	return &t, nil
}

// logResponseBody makes a copy of the response body and logs it. Because it
// consumes the body it duplicates it and replaces the original. It also closes
// the body it consumes.
func logResponseBody(logger *slog.Logger, name string, resp *http.Response) error {
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
		logger.Log(ctx, log.LevelTrace, "rx", "endpoint", name, "body", string(body))

		return nil
	}

	return nil
}

func newUpdate(data []byte) Status {
	var msg prodThingUpdate

	if err := json.Unmarshal(data, &msg); err != nil {
		return Status{Error: err}
	}

	return Status{
		Ambient:         msg.Status.Ambient,
		Connected:       msg.Status.Connected,
		Grill:           msg.Status.Grill,
		GrillSet:        msg.Status.Set,
		KeepWarm:        msg.Status.KeepWarm,
		PelletLevel:     msg.Status.PelletLevel,
		Probe:           msg.Status.Probe,
		ProbeAlarmFired: msg.Status.ProbeAlarmFired != 0,
		ProbeConnected:  msg.Status.ProbeConnected != 0,
		ProbeSet:        msg.Status.ProbeSet,
		Smoke:           msg.Status.Smoke,
		Time:            time.Unix(msg.Status.Time, 0),
		Units:           Units(msg.Status.Units),
		SystemStatus:    SystemStatus(msg.Status.SystemStatus),
	}
}
