package routeros

import (
	"fmt"
	"github.com/rs/zerolog"
	"gopkg.in/routeros.v2"
	"os"
	"strings"
)

type RouterOS struct {
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

func New(config Configs) RouterOS {
	zerolog.SetGlobalLevel(zerolog.NoLevel)
	if config.ModeDebug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	return RouterOS{config, &routeros.Reply{}, nil, RegisterLog(), []string{}, []string{}}
}

func (this *RouterOS) Run(query []string) *RouterOS {
	host := fmt.Sprintf("%s:%s", this.Ip, this.Port)
	c, err := routeros.Dial(host, this.Username, this.Password)
	if err != nil {
		this.Debug().Msg(fmt.Sprintf("| ERROR | %s", err.Error()))
		this.Error = err
		return this
	}
	defer c.Close()

	re, err := c.RunArgs(query)
	if err != nil {
		this.Error = err
		return this
	}
	this.Reply = re
	return this
}

func (this *RouterOS) Command(query string) *RouterOS {
	this.Query = []string{query}
	return this
}

func (this *RouterOS) DetectError() bool {
	if this.Error != nil {
		this.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(this.Query, " ")))
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
