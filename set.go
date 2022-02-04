package routeros

import (
	"fmt"
	"reflect"
	"strings"
)

func (this *RouterOS) SetByID(ID string, data interface{}) *RouterOS {
	//set action last command
	this.Query[len(this.Query)-1] += "/set"

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
					this.Query = append(this.Query, fmt.Sprintf("=%s=%s", kName, kValue))
				}
			}
		}
	}

	//cek where
	this.Query = append(this.Query, fmt.Sprintf("=.id=%s", ID))

	this.Run(this.Query)

	this.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(this.Query, " ")))
	this.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d rows updated | message %s", len(this.Reply.Done.Map), this.Reply.Done.Word))
	return this
}
