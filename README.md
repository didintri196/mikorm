
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
	config := mikorm.Configs{
		Ip:        "127.0.0.1",
		Port:      "8728",
		Username:  "admin",
		Password:  "",
		ModeDebug: true,
	}
	connRouteOS := mikorm.New(config)

......

}
```


## Documentation

[Documentation](https://linktodocumentation)


## Features

```go

type SecretRepository struct {
    MikORM *mikorm.MikORM
}

func (repo SecretRepository) Browse(filter models.Secret) (secrets []models.Secret, err error) {
    err = repo.MikORM.Command("/ppp/secret").Where(&filter).Scan(&secrets).Error
    return
}

func (repo SecretRepository) Add(secret models.Secret) (err error) {
    return repo.MikORM.Command("/ppp/secret").Add(&secret).Error
}

func (repo SecretRepository) Read(filter models.Secret) (secret models.Secret, err error) {
    err = repo.MikORM.Command("/ppp/secret").Where(&filter).Print(&secret).Error
    return
}

func (repo SecretRepository) Edit(filter models.Secret, data models.Secret) (err error) {
    err = repo.MikORM.Command("/ppp/secret").SetByID("", &data).Error
    return
}

func (repo SecretRepository) Remove(ID string) (err error) {
    err = repo.MikORM.Command("/ppp/secret").RemoveByID(ID).Error
    return
}

func (repo SecretRepository) Enable(ID string) (err error) {
    err = repo.MikORM.Command("/ppp/secret").EnableByID(ID).Error
    return
}

func (repo SecretRepository) Disable(ID string) (err error) {
    err = repo.MikORM.Command("/ppp/secret").DisableByID(ID).Error
    return
}

```
