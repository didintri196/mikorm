// Package mikorm is a small ORM-style client for the MikroTik RouterOS API.
//
// It wraps the RouterOS API protocol with a fluent, context-aware query
// builder and maps API records to and from Go structs using the "mikorm"
// struct tag.
//
// # Connecting
//
//	client, err := mikorm.Dial(ctx, mikorm.Config{
//		Address:  "192.168.88.1",
//		Username: "admin",
//		Password: "secret",
//	})
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
// A Client is safe for concurrent use by multiple goroutines.
//
// # Models
//
// Struct fields are mapped to RouterOS properties with the "mikorm" tag:
//
//	type IPAddress struct {
//		ID        string `mikorm:".id"`
//		Address   string `mikorm:"address"`
//		Network   string `mikorm:"network,readonly"`
//		Interface string `mikorm:"interface"`
//		Disabled  bool   `mikorm:"disabled"`
//		Dynamic   bool   `mikorm:"dynamic,readonly"`
//		Comment   string `mikorm:"comment"`
//	}
//
// The tag value is the property name, optionally followed by options:
//
//   - "-" ignores the field.
//   - "readonly" decodes the field but never sends it in Add or Set.
//
// Fields without a tag use the kebab-case form of the field name
// (MACAddress becomes "mac-address"). The ".id" property is always treated
// as read-only. Embedded structs are flattened.
//
// Supported field types are string, bool, all integer and float kinds,
// time.Duration (RouterOS format such as "1d2h3m4s"), []string (comma
// separated), types implementing encoding.TextMarshaler /
// encoding.TextUnmarshaler, and pointers to any of these.
//
// When encoding (Add, Set, WhereModel) zero values are skipped. Use a
// pointer field to send a zero value explicitly, for example a *bool set to
// false.
//
// # Queries
//
//	var addrs []IPAddress
//	err := client.Query("/ip/address").
//		Where("interface", "ether1").
//		Find(ctx, &addrs)
//
//	id, err := client.Query("/ip/address").Add(ctx, IPAddress{
//		Address:   "10.0.0.1/24",
//		Interface: "ether1",
//	})
//
//	err = client.Query("/ip/address").Set(ctx, id, map[string]string{"comment": "lan"})
//	err = client.Query("/ip/address").Remove(ctx, id)
//
// Arbitrary commands can be sent with Client.Run.
package mikorm
