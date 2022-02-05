package mikorm

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/routeros.v2"
)

type MikORM struct {
	Configs
	*routeros.Reply
	Error error
	zerolog.Logger
	Query  []string
	Filter []string
}

type Configs struct {
	Ip        string
	Port      string
	Username  string
	Password  string
	ModeDebug bool
}

func New(config Configs) MikORM {
	zerolog.SetGlobalLevel(zerolog.NoLevel)
	if config.ModeDebug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return MikORM{config, &routeros.Reply{}, nil, RegisterLog(), []string{}, []string{}}
}

func (route *MikORM) Run(query []string) *MikORM {
	host := fmt.Sprintf("%s:%s", route.Ip, route.Port)
	c, err := routeros.Dial(host, route.Username, route.Password)
	if err != nil {
		route.Debug().Msg(fmt.Sprintf("| ERROR | %s", err.Error()))
		route.Error = err
		return route
	}
	defer c.Close()

	re, err := c.RunArgs(query)
	if err != nil {
		route.Error = err
		return route
	}
	route.Reply = re
	return route
}

func (route *MikORM) Command(query string) *MikORM {
	route.Query = []string{query}
	return route
}

func (route *MikORM) DetectError() bool {
	if route.Error != nil {
		route.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(route.Query, " ")))
		return true
	}
	return false
}

func RegisterLog() zerolog.Logger {
	output := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "2006-01-02 15:04:05"}
	output.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf(""))
	}
	output.FormatMessage = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}
	output.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}
	output.FormatFieldValue = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("%s", i))
	}

	return zerolog.New(output).With().Timestamp().Logger()
}
