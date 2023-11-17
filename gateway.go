package mikorm

import (
	"fmt"
	"gopkg.in/routeros.v2"
)

// Gateway is a struct for setting up the connection to the mikrotik API
// Host is required
// Port is optional, default is 8728
// Username is required
// Password default is empty
type Gateway struct {
	OptionGateway

	client *routeros.Client
}

type OptionGateway struct {
	Host     string
	Port     string
	Username string
	Password string
}

func NewGateway(option OptionGateway) *Gateway {
	return &Gateway{
		OptionGateway: option,
	}
}

func (g *Gateway) Connect() (err error) {
	if g.Host == "" {
		return fmt.Errorf("host is required")
	}

	if g.Port == "" {
		g.Port = "8728"
	}

	if g.Username == "" {
		return fmt.Errorf("username is required")
	}

	g.client, err = routeros.Dial(fmt.Sprintf("%s:%s", g.Host, g.Port), g.Username, g.Password)
	if err != nil {
		return fmt.Errorf("failed to connect to mikrotik: %w", err)
	}

	return nil
}

func (g *Gateway) Client() *routeros.Client {
	return g.client
}

func (g *Gateway) Close(client *routeros.Client) {
	client.Close()
}
