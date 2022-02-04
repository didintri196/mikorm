package routeros

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (this *RouterOS) Print(bind interface{}) *RouterOS {
	//set action last command
	this.Query[len(this.Query)-1] += "/print"

	//cek filter
	this.Query = append(this.Query, this.Filter...)

	// Run Query
	this.Run(this.Query)

	if len(this.Reply.Re) == 0 {
		this.Error = errors.New("Data Not Found")
	}

	// Deteksi Error
	if this.DetectError() {
		return this
	}

	// Parsing Data Mikrotik
	jsonbody, err := json.Marshal(this.Reply.Re[0].Map)
	if err != nil {
		// do error check
		this.Error = err
		return this
	}

	if err := json.Unmarshal(jsonbody, bind); err != nil {
		// do error check
		this.Error = err
		return this
	}
	this.Debug().Msg(fmt.Sprintf("| DEBUG | [QUERY] %s", strings.Join(this.Query, " ")))
	this.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d fields | message %s", len(this.Re[0].Map), this.Reply.Done.Word))
	return this
}
