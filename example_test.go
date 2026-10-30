package mikorm_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/didintri196/mikorm"
)

type IPAddress struct {
	ID        string `mikorm:".id"`
	Address   string `mikorm:"address"`
	Network   string `mikorm:"network,readonly"`
	Interface string `mikorm:"interface"`
	Disabled  bool   `mikorm:"disabled"`
	Dynamic   bool   `mikorm:"dynamic,readonly"`
	Comment   string `mikorm:"comment"`
}

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mikorm.Dial(ctx, mikorm.Config{
		Address:  "192.168.88.1",
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	addresses := client.Query("/ip/address")

	// Create.
	id, err := addresses.Add(ctx, IPAddress{
		Address:   "10.10.10.1/24",
		Interface: "ether2",
		Comment:   "lan",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Read.
	var list []IPAddress
	if err := addresses.Where("interface", "ether2").Find(ctx, &list); err != nil {
		log.Fatal(err)
	}
	fmt.Println(list)

	// Update.
	if err := addresses.Set(ctx, id, map[string]string{"comment": "lan-updated"}); err != nil {
		log.Fatal(err)
	}

	// Delete.
	if err := addresses.Remove(ctx, id); err != nil {
		log.Fatal(err)
	}
}

func ExampleQuery_First() {
	var client *mikorm.Client // from mikorm.Dial
	ctx := context.Background()

	var addr IPAddress
	err := client.Query("/ip/address").Where("comment", "wan").First(ctx, &addr)
	switch {
	case errors.Is(err, mikorm.ErrNotFound):
		fmt.Println("no WAN address")
	case err != nil:
		log.Fatal(err)
	default:
		fmt.Println(addr.Address)
	}
}

func ExampleClient_Run() {
	var client *mikorm.Client // from mikorm.Dial
	ctx := context.Background()

	reply, err := client.Run(ctx, "/system/resource/print")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(reply.Records[0]["uptime"])
}

func ExampleClient_Listen() {
	var client *mikorm.Client // from mikorm.Dial
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := client.Listen(ctx, func(rec map[string]string) error {
		fmt.Println(rec["name"], rec["rx-bits-per-second"])
		return nil
	}, "/interface/monitor-traffic", "=interface=ether1")
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		log.Fatal(err)
	}
}

func ExampleDeviceError() {
	var client *mikorm.Client // from mikorm.Dial
	ctx := context.Background()

	_, err := client.Query("/ip/address").Add(ctx, map[string]string{"address": "invalid"})
	var devErr *mikorm.DeviceError
	if errors.As(err, &devErr) {
		fmt.Println("router rejected the command:", devErr.Message)
	}
}
