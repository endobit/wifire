package wifire

import mqtt "github.com/eclipse/paho.mqtt.golang"

type Grill struct {
	name   string
	wifire WiFire
	client mqtt.Client
}

func (w WiFire) NewGrill(name string) *Grill {
	return &Grill{
		name:   name,
		wifire: w,
	}
}

func (g *Grill) Connect() error {
	client, err := g.wifire.GetMQTT()
	if err != nil {
		return err
	}

	g.client = client
	if err := g.connect(); err != nil {
		return err
	}

	return nil
}

func (g Grill) Disconnect() {
	g.client.Disconnect(0)
}

func (g Grill) connect() error {
	if token := g.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}
