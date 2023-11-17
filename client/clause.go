package client

import (
	"fmt"
	"github.com/google/uuid"
	"reflect"
)

type (
	Clause struct {
		*RouterOsClient
		Id uuid.UUID
	}
)

// Where is a method to set where clause in RouterOsClient
//
// filter is a struct that contains field and value
//
// example:
//
//	type Address struct {
//		Id   string `mikorm:"id"`
//		Name string `mikorm:"name"`
//	}
//
//	var user User
//	err := client.Command("/ip/address/print").Where(Address{Id: "1"}).Do().Scan(&user)
func (r *Clause) Where(filter interface{}) IClause {
	var stmt = reflect.ValueOf(filter)

	// Check if stmt is a pointer and is nil
	if stmt.Kind() == reflect.Ptr && stmt.IsNil() {
		// Create a new instance of the underlying struct
		stmt = reflect.New(stmt.Type().Elem())
	}

	// Now check if stmt is a pointer and get the underlying struct value
	if stmt.Kind() == reflect.Ptr {
		stmt = stmt.Elem()
	}

	typeOfS := stmt.Type()
	for i := 0; i < stmt.NumField(); i++ {
		if stmt.Field(i).CanInterface() {
			kName := typeOfS.Field(i).Tag.Get("mikorm")
			kValue := fmt.Sprintf("%v", stmt.Field(i).Interface())
			if kValue != "" {
				r.setWhere(r.Id, fmt.Sprintf(`?%s=%s`, kName, kValue))
			}
		}
	}
	return r
}

// Do is a method to execute the command
func (r *Clause) Do() IExec {
	return &Exec{
		Clause: r,
	}
}
