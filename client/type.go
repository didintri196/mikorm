package client

import (
	"github.com/google/uuid"
	"gopkg.in/routeros.v2"
	"sync"
)

type (
	RouterClient  *routeros.Client
	RouterCommand struct {
		Command string
		Where   []string
	}
	Pool           map[uuid.UUID]RouterCommand
	RouterOsClient struct {
		Mtx             *sync.RWMutex
		Client          RouterClient
		CommandPool     Pool
		ResponseSuccess interface{}
		ResponseError   interface{}
	}
)

func MakeCommandPool() Pool {
	return make(map[uuid.UUID]RouterCommand)
}
