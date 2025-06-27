package wifire

import (
	"encoding/json"
	"net/http"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type getMQTTResponse struct {
	ExpirationSeconds int    `json:"expirationSeconds"`
	ExpiresAt         int    `json:"expiresAt"`
	SignedURL         string `json:"signedUrl"`
}

func (w *WiFire) getMQTT() (mqtt.Client, error) {
	req, err := http.NewRequest(http.MethodPost, w.config.baseURL+"/prod/mqtt-connections", http.NoBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("authorization", w.token)

	c := http.Client{}

	r, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	defer r.Body.Close()

	var data getMQTTResponse

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return nil, err
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(data.SignedURL)
	opts.OnConnect = connect
	opts.OnConnectionLost = connectionLost
	opts.OnReconnecting = reconnecting

	return mqtt.NewClient(opts), nil
}

func connect(_ mqtt.Client) {
	if Logger != nil {
		Logger(LogInfo, "wifire", "connect")
	}
}

func connectionLost(_ mqtt.Client, _ error) {
	if Logger != nil {
		Logger(LogInfo, "wifire", "connectionLost")
	}
}

func reconnecting(_ mqtt.Client, _ *mqtt.ClientOptions) {
	if Logger != nil {
		Logger(LogInfo, "wifire", "reconnecting")
	}
}
