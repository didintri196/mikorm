
![Logo](https://dev-to-uploads.s3.amazonaws.com/uploads/articles/th5xamgrr6se0x5ro4g6.png)


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
  go get github.com/didintri196/mikorm
```
    
## Usage/Examples

```go
package main

import (
	"fmt"
	"github.com/didintri196/mikrotik-restfull/domain/models"
	"github.com/didintri196/mikrotik-restfull/repositories"
	"github.com/didintri196/mikrotik-restfull/utils/routeros"
)

func main() {
	config := routeros.Configs{
		Ip:        "127.0.0.1",
		Port:      "8728",
		Username:  "admin",
		Password:  "",
		ModeDebug: true,
	}
	connRouteOS := routeros.New(config)

......

}
```


## Documentation

[Documentation](https://linktodocumentation)

