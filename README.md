
![Logo](https://raw.githubusercontent.com/didintri196/mikorm/master/logo.png)


# Mikrotik ORM (mikorm)

Library Mikrotik API menggunakan ORM untuk mempermudah integrasi ke Mikrotik

## Acknowledgements

 - [RouterOS v.2](https://gopkg.in/routeros.v2)
 - [Zero Log](https://github.com/rs/zerolog)


## Badges

[![MIT License](https://img.shields.io/apm/l/atomic-design-ui.svg?)](https://github.com/didintri196/mikorm/blob/master/LICENSEs)
[![GPLv3 License](https://img.shields.io/badge/License-GPL%20v3-yellow.svg)](https://opensource.org/licenses/)
[![AGPL License](https://img.shields.io/badge/license-AGPL-blue.svg)](http://www.gnu.org/licenses/agpl-3.0)


## Authors

- [@didintri196](https://www.github.com/didintri196)


## Installation

Install mikorm with go get

```bash
go get github.com/didintri196/mikorm@v1.0.0
```
    
## Usage/Examples

```go
package main

import (
	"fmt"
	"github.com/didintri196/mikorm"
)

func main() {
	type IpAddress struct {
		Address   string `mikorm:"address"`
		Network   string `mikorm:"network"`
		Interface string `mikorm:"interface"`
	}

	gw := NewGateway(OptionGateway{
		Host:     "localhost",
		Port:     "8728",
		Username: "admin",
		Password: "admin",
	})

	err := gw.Connect()
	if err != nil {
		t.Error(err)
		return
	}

	mikrotik := NewMikorm(gw)

	var ipAddress []IpAddress

	err = mikrotik.
		Command("/ip/address/print").
		Do().
		Print(&ipAddress)

	if err != nil {
		return
	}

	fmt.Println("TestPrint", ipAddress)

}
```


## Documentation

[Documentation](https://linktodocumentation)
