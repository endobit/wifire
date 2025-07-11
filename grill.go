package wifire

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"endobit.io/app/log"
)

// Grill is a handle for a grills MQTT connection.
type Grill struct {
	logger *slog.Logger
	name   string
	wifire *Client
	client mqtt.Client
}

// NewGrill returns a Grill with the given name.
func NewGrill(name string, wifire *Client) *Grill {
	return &Grill{
		name:   name,
		wifire: wifire,
		logger: wifire.logger.With("grill", name),
	}
}

// Connect establishes the MQTT connection to the Grill.
func (g *Grill) Connect() error {
	client, err := g.wifire.MQTT()
	if err != nil {
		return err
	}

	g.client = client

	return g.connect()
}

// Disconnect closed the MQTT connection to the Grill.
func (g *Grill) Disconnect() {
	g.client.Disconnect(0)
}

// SubscribeStatus subscribes to the prod/thing/update for the grill. SubscribeStatus
// updates are pushed to the returned channel.
func (g *Grill) SubscribeStatus(subscriber chan Status) error {
	if !g.client.IsConnected() {
		if err := g.connect(); err != nil {
			return err
		}
	}

	token := g.client.Subscribe("prod/thing/update/"+g.name, 1, func(_ mqtt.Client, m mqtt.Message) {
		var msg map[string]any

		payload := m.Payload()

		if err := json.Unmarshal(payload, &msg); err != nil {
			slog.Error("bad message", "msg", string(payload))
		}

		m.Ack() // doesn't do anything

		g.logger.LogAttrs(context.Background(), log.LevelTrace, "rx",
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

func (g *Grill) connect() error {
	if token := g.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
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
		RealTime:        msg.Status.RealTime,
		Smoke:           msg.Status.Smoke,
		Time:            time.Unix(msg.Status.Time, 0),
		Units:           Units(msg.Status.Units),
		SystemStatus:    SystemStatus(msg.Status.SystemStatus),
	}
}
