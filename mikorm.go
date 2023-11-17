package mikorm

import (
	"github.com/didintri196/mikorm/client"
	"sync"
)

// NewMikorm creates a new instance
func NewMikorm(session *Gateway) client.IClient {
	mtx := sync.RWMutex{}
	comPool := client.MakeCommandPool()
	return &client.RouterOsClient{
		Mtx:         &mtx,
		Client:      session.Client(),
		CommandPool: comPool,
	}
}
