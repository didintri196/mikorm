package mikorm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (route *MikORM) Print(bind interface{}) *MikORM {
	//set action last command
	route.Query[len(route.Query)-1] += "/print"

	//cek filter
	route.Query = append(route.Query, route.Filter...)

	// Run Query
	route.Run(route.Query)

	if len(route.Reply.Re) == 0 {
		route.Error = errors.New("Data Not Found")
	}

	// Deteksi Error
	if route.DetectError() {
		return route
	}

	// Parsing Data Mikrotik
	jsonbody, err := json.Marshal(route.Reply.Re[0].Map)
	if err != nil {
		// do error check
		route.Error = err
		return route
	}

	if err := json.Unmarshal(jsonbody, bind); err != nil {
		// do error check
		route.Error = err
		return route
	}
	route.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(route.Query, " ")))
	route.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d fields | message %s", len(route.Re[0].Map), route.Reply.Done.Word))
	return route
}
