package wifire

import (
	"encoding/json"
	"io"
	"net/http"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

type getMQTTResponse struct {
	ExpirationSeconds int    `json:"expirationSeconds"`
	ExpiresAt         int    `json:"expiresAt"`
	SignedURL         string `json:"signedUrl"`
}

// type MQTT struct {
// 	Expires time.Time
// 	URL     string
// }

func (w WiFire) GetMQTT() (mqtt.Client, error) {
	req, err := http.NewRequest("POST", w.config.baseURL+"/prod/mqtt-connections", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", w.token)

	c := http.Client{}

	r, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	resp, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	var data getMQTTResponse

	if err := json.Unmarshal(resp, &data); err != nil {
		return nil, err
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(data.SignedURL)
	opts.OnConnect = connect
	opts.OnConnectionLost = connectionLost
	opts.OnReconnecting = reconnecting

	return mqtt.NewClient(opts), nil
}

func connect(c mqtt.Client) {
	log.Info().Msg("connect")
}

func connectionLost(c mqtt.Client, err error) {
	log.Err(err).Msg("connectionLost")
}

func reconnecting(c mqtt.Client, o *mqtt.ClientOptions) {
	log.Info().Msg("reconnecting")
}
