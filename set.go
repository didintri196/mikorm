package mikorm

import (
	"fmt"
	"reflect"
	"strings"
)

func (route *MikORM) SetByID(ID string, data interface{}) *MikORM {
	//set action last command
	route.Query[len(route.Query)-1] += "/set"

	var stmt reflect.Value = reflect.ValueOf(data)
	if stmt.IsNil() && stmt.CanAddr() {
		stmt.Set(reflect.New(stmt.Type()))
	}

	typeOfS := stmt.Elem().Type()
	for i := 0; i < stmt.Elem().NumField(); i++ {
		if stmt.Elem().Field(i).CanInterface() {
			kName := typeOfS.Field(i).Tag.Get("json")
			kValue := stmt.Elem().Field(i).String()
			if kValue != "" {
				if kName != ".id" {
					route.Query = append(route.Query, fmt.Sprintf("=%s=%s", kName, kValue))
				}
			}
		}
	}

	//cek where
	route.Query = append(route.Query, fmt.Sprintf("=.id=%s", ID))

	route.Run(route.Query)

	route.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(route.Query, " ")))
	route.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d rows updated | message %s", len(route.Reply.Done.Map), route.Reply.Done.Word))
	return route
}
