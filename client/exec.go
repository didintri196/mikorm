package client

import (
	"encoding/json"
	"fmt"
	"gopkg.in/routeros.v2"
	"strings"
)

type (
	Exec struct {
		*Clause
	}
)

// Print is a method to print the result of the command
//
// model is a struct that contains field and value
//
// example:
//
//	type IpAddress struct {
//		Id   string `mikorm:"id"`
//		Name string `mikorm:"name"`
//	}
//
//	var ipAddress IpAddress
//	err := client.Command("/ip/address/print").Where(IpAddress{Id: "1"}).Print(&IpAddress)
func (r *Clause) Print(model interface{}) (err error) {
	com := r.getRequest(r.Id)

	command := []string{com.Command}
	// add where clause if exist
	if len(com.Where) > 0 {
		command = append(command, com.Where...)
	}

	runArgs := (*routeros.Client).RunArgs
	resp, err := runArgs(r.Client, command)
	if err != nil {
		return
	}

	var dataArr []interface{}
	for i := 0; i < len(resp.Re); i++ {
		dataArr = append(dataArr, resp.Re[i].Map)
	}

	jsonbody, err := json.Marshal(dataArr)
	if err != nil {
		// do error check
		return
	}

	fmt.Println("COMMAND", strings.Join(command, " "))
	fmt.Println("RESPONSE", string(jsonbody))

	if err = json.Unmarshal(jsonbody, model); err != nil {
		// do error check
		return
	}

	return
}
