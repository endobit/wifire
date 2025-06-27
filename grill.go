package wifire

import mqtt "github.com/eclipse/paho.mqtt.golang"

// Grill is a handle for a grills MQTT connection.
type Grill struct {
	name   string
	wifire *WiFire
	client mqtt.Client
}

// NewGrill returns a Grill with the given name.
func (w *WiFire) NewGrill(name string) *Grill {
	return &Grill{
		name:   name,
		wifire: w,
	}
}

// Connect establishes the MQTT connection to the Grill.
func (g *Grill) Connect() error {
	client, err := g.wifire.getMQTT()
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

func (g *Grill) connect() error {
	if token := g.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}
