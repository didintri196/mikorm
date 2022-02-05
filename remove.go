package mikorm

import (
	"fmt"
	"strings"
)

func (route *MikORM) RemoveByID(ID string) *MikORM {
	//set action last command
	route.Query[len(route.Query)-1] += "/remove"

	//cek where
	route.Query = append(route.Query, fmt.Sprintf("=.id=%s", ID))

	route.Run(route.Query)

	route.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(route.Query, " ")))
	route.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d rows updated | message %s", len(route.Reply.Done.Map), route.Reply.Done.Word))
	return route
}
