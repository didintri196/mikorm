package routeros

import (
	"fmt"
	"strings"
)

func (this *RouterOS) DisableByID(ID string) *RouterOS {
	//set action last command
	this.Query[len(this.Query)-1] += "/disable"

	//cek where
	this.Query = append(this.Query, fmt.Sprintf("=.id=%s", ID))

	this.Run(this.Query)

	this.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(this.Query, " ")))
	this.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d rows updated | message %s", len(this.Reply.Done.Map), this.Reply.Done.Word))
	return this
}
