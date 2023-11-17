package mikorm

import "testing"

func TestPrint(t *testing.T) {
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

	err = mikrotik.Command("/ip/address/print").Where(IpAddress{
		Network: "172.16.0.0",
	}).Do().Print(&ipAddress)

	if err != nil {
		t.Error(err)
		return
	}

	t.Log("TestPrint", ipAddress)
}
