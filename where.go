package mikorm

import (
	"fmt"
	"reflect"
)

func (route *MikORM) Where(filter interface{}) *MikORM {
	var stmt reflect.Value = reflect.ValueOf(filter)
	if stmt.IsNil() && stmt.CanAddr() {
		stmt.Set(reflect.New(stmt.Type()))
	}

	typeOfS := stmt.Elem().Type()
	for i := 0; i < stmt.Elem().NumField(); i++ {
		if stmt.Elem().Field(i).CanInterface() {
			kName := typeOfS.Field(i).Tag.Get("json")
			kValue := stmt.Elem().Field(i).String()
			if kValue != "" {
				route.Filter = append(route.Filter, fmt.Sprintf(`?%s=%s`, kName, kValue))
			}
		}
	}

	return route
}
