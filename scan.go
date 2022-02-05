package mikorm

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (route *MikORM) Scan(bind interface{}) *MikORM {
	//set action last command
	route.Query[len(route.Query)-1] += "/print"

	//cek filter
	route.Query = append(route.Query, route.Filter...)

	// Run Query
	route.Run(route.Query)

	// Deteksi Error
	if route.DetectError() {
		return route
	}

	var dataArr []interface{}
	for i := 0; i < len(route.Reply.Re); i++ {
		dataArr = append(dataArr, route.Reply.Re[i].Map)
	}

	jsonbody, err := json.Marshal(dataArr)
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
	route.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d rows | message %s", len(route.Re), route.Reply.Done.Word))
	return route
}
