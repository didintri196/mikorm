package routeros

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (this *RouterOS) Scan(bind interface{}) *RouterOS {
	//set action last command
	this.Query[len(this.Query)-1] += "/print"

	//cek filter
	this.Query = append(this.Query, this.Filter...)

	// Run Query
	this.Run(this.Query)

	// Deteksi Error
	if this.DetectError() {
		return this
	}

	var dataArr []interface{}
	for i := 0; i < len(this.Reply.Re); i++ {
		dataArr = append(dataArr, this.Reply.Re[i].Map)
	}

	jsonbody, err := json.Marshal(dataArr)
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
	this.Debug().Msg(fmt.Sprintf("| DEBUG | [REPLY] %d rows | message %s", len(this.Re), this.Reply.Done.Word))
	return this
}
