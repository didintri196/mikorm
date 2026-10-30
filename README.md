![Logo](https://raw.githubusercontent.com/didintri196/mikorm/master/logo.png)

# Mikrotik ORM (mikorm)

[![Go Reference](https://pkg.go.dev/badge/github.com/didintri196/mikorm.svg)](https://pkg.go.dev/github.com/didintri196/mikorm)
[![GPLv3 License](https://img.shields.io/badge/License-GPL%20v3-yellow.svg)](LICENSE)

Library Go untuk MikroTik RouterOS API dengan gaya ORM: query builder yang
fluent, mapping otomatis ke struct, dan aman dipakai bersamaan (concurrent).

## Fitur

- **Tanpa dependensi** — implementasi protokol RouterOS API sendiri.
- **Mapping struct** lewat tag `mikorm` (string, bool, angka, `time.Duration`,
  `[]string`, `encoding.TextUnmarshaler` seperti `netip.Prefix`, pointer).
- **CRUD lengkap**: `Find`, `First`, `Count`, `IDs`, `Add`, `Set`, `Remove`,
  `Enable`, `Disable`, `Exec`.
- **Query**: `Where`, `WhereNot`, `WhereIn`, `WhereExists`, `WhereNotExists`,
  `WhereLess`, `WhereGreater`, `WhereModel`, `Select` (`.proplist`).
- **`context.Context`** di setiap operasi — timeout/cancel otomatis mengirim
  `/cancel` ke router.
- **Concurrency-safe**: satu koneksi bisa dipakai banyak goroutine (command
  di-tag dan balasan didistribusikan sesuai tag).
- **Streaming** (`Listen`) untuk command seperti `/interface/listen`,
  `/interface/monitor-traffic`, `/tool/torch`.
- Login RouterOS ≥ 6.43 maupun versi lama (challenge/response), API-SSL (TLS).
- Error dari router bertipe `*mikorm.DeviceError`, logging opsional via `log/slog`
  (password/secret otomatis disensor).

## Instalasi

```bash
go get github.com/didintri196/mikorm
```

Butuh Go 1.21 atau lebih baru.

## Penggunaan

### Koneksi

```go
ctx := context.Background()

client, err := mikorm.Dial(ctx, mikorm.Config{
	Address:  "192.168.88.1", // port default 8728 (8729 jika TLS)
	Username: "admin",
	Password: "secret",
	// TLS:     &tls.Config{InsecureSkipVerify: true}, // untuk API-SSL
	// Timeout: 5 * time.Second,                       // timeout dial + login
	// Logger:  slog.Default(),                        // log debug setiap command
})
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

### Model

```go
type IPAddress struct {
	ID        string `mikorm:".id"`
	Address   string `mikorm:"address"`
	Network   string `mikorm:"network,readonly"` // tidak dikirim saat Add/Set
	Interface string `mikorm:"interface"`
	Disabled  bool   `mikorm:"disabled"`
	Dynamic   bool   `mikorm:"dynamic,readonly"`
	Comment   string `mikorm:"comment"`
	Internal  string `mikorm:"-"` // diabaikan
}
```

Aturan tag:

| Tag                         | Arti                                                       |
|-----------------------------|------------------------------------------------------------|
| `mikorm:"name"`             | nama property RouterOS                                     |
| `mikorm:"name,readonly"`    | hanya dibaca, tidak pernah dikirim pada `Add`/`Set`        |
| `mikorm:"-"`                | field diabaikan                                            |
| tanpa tag                   | nama field diubah ke kebab-case (`MACAddress` → `mac-address`) |

`.id` selalu dianggap read-only. Saat encode (`Add`, `Set`, `WhereModel`)
nilai kosong/zero dilewati — gunakan field pointer (mis. `*bool`) untuk
mengirim nilai zero secara eksplisit.

### Membaca data

```go
addresses := client.Query("/ip/address") // "/ip address" juga diterima

// Semua data
var list []IPAddress
err := addresses.Find(ctx, &list)

// Dengan filter
err = addresses.
	Where("interface", "ether1").
	WhereNot("disabled", true).
	Find(ctx, &list)

// Filter dari struct (field non-zero)
err = addresses.WhereModel(IPAddress{Interface: "ether1"}).Find(ctx, &list)

// Satu data
var addr IPAddress
err = addresses.Where("comment", "wan").First(ctx, &addr)
if errors.Is(err, mikorm.ErrNotFound) {
	// tidak ada
}

// Hitung & ambil ID saja
n, err := addresses.Where("dynamic", true).Count(ctx)
ids, err := addresses.WhereIn("interface", "ether1", "ether2").IDs(ctx)

// Batasi property yang diambil; hasil bisa juga ke map
var rows []map[string]string
err = addresses.Select("address", "interface").Find(ctx, &rows)
```

`Query` bersifat immutable: setiap method builder mengembalikan `Query` baru,
sehingga aman disimpan dan dipakai ulang.

### Menulis data

```go
id, err := addresses.Add(ctx, IPAddress{
	Address:   "10.10.10.1/24",
	Interface: "ether2",
	Comment:   "lan",
})

err = addresses.Set(ctx, id, map[string]any{"comment": "lan-baru"})
err = addresses.Disable(ctx, id)
err = addresses.Enable(ctx, id)
err = addresses.Remove(ctx, id)

// Update / hapus berdasarkan filter
ids, _ := addresses.Where("comment", "temp").IDs(ctx)
err = addresses.Remove(ctx, ids...)
```

### Command bebas

```go
reply, err := client.Run(ctx, "/system/resource/print")
fmt.Println(reply.Records[0]["uptime"])

var res []struct {
	Uptime  time.Duration `mikorm:"uptime"`
	Version string        `mikorm:"version"`
}
err = reply.Scan(&res)

// Command lain pada sebuah menu
_, err = client.Query("/interface/ethernet").Exec(ctx, "reset-counters", map[string]string{"numbers": "ether1"})
```

### Streaming

```go
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

err := client.Listen(ctx, func(rec map[string]string) error {
	fmt.Println(rec["rx-bits-per-second"])
	return nil // kembalikan error untuk berhenti
}, "/interface/monitor-traffic", "=interface=ether1")
```

### Error handling

```go
_, err := addresses.Add(ctx, map[string]string{"address": "salah"})

var devErr *mikorm.DeviceError
if errors.As(err, &devErr) {
	fmt.Println("ditolak router:", devErr.Message)
}
```

| Error                   | Kapan                                                      |
|-------------------------|------------------------------------------------------------|
| `*mikorm.DeviceError`   | router membalas `!trap` / `!fatal`                         |
| `mikorm.ErrNotFound`    | `First` tidak menemukan data                               |
| `mikorm.ErrClosed`      | client dipakai setelah `Close`                             |
| `context.Canceled` dll. | context selesai sebelum router membalas                   |

## Migrasi dari versi lama

| Lama                                                   | Baru                                               |
|--------------------------------------------------------|----------------------------------------------------|
| `NewGateway(OptionGateway{...})` + `gw.Connect()`      | `mikorm.Dial(ctx, mikorm.Config{...})`             |
| `NewMikorm(gw)`                                        | (tidak perlu, `Dial` langsung mengembalikan client) |
| `Command("/ip/address/print").Where(m).Do().Print(&v)` | `Query("/ip/address").WhereModel(m).Find(ctx, &v)` |

## Testing

```bash
go test -race ./...
```

Test memakai fake RouterOS server in-memory yang berbicara protokol API asli,
jadi tidak butuh router sungguhan.

## Author

- [@didintri196](https://www.github.com/didintri196)
