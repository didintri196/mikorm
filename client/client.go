package client

import (
	"github.com/didintri196/mikorm/utils"
	"github.com/google/uuid"
)

func (c *RouterOsClient) Command(cmd string) IClause {
	return &Clause{
		Id:             c.commandInit(cmd),
		RouterOsClient: c,
	}
}

func (c *RouterOsClient) commandInit(cmd string) uuid.UUID {
	id := uuid.New()
	c.setCommand(id, cmd)
	return id
}

func (c *RouterOsClient) setCommand(id uuid.UUID, command string) {
	utils.WithLock(c.Mtx, func() {
		c.CommandPool[id] = RouterCommand{
			Command: command,
			Where:   []string{},
		}
	})
}

func (c *RouterOsClient) setWhere(id uuid.UUID, where string) {
	utils.WithLock(c.Mtx, func() {
		c.CommandPool[id] = RouterCommand{
			Command: c.CommandPool[id].Command,
			Where:   append(c.CommandPool[id].Where, where),
		}
	})
}

func (c *RouterOsClient) getRequest(id uuid.UUID) (com RouterCommand) {
	utils.WithRLock(c.Mtx, func() {
		com = c.CommandPool[id]
	})
	return
}
