package batch

import (
	"fmt"
	"io"
	"log"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

type WireDialer struct {
	tun    tun.Device
	tnet   *netstack.Net
	Device *device.Device
}

type WGDialer struct {
	WireDialer *WireDialer
}

func NewDialerFromConfiguration(config_reader io.Reader) (*WireDialer, error) {
	iface_addresses, dns_addresses, mtu, ipcConfig, err := ParseConfig(config_reader)
	if err != nil {
		return nil, err
	}

	tun, tnet, err := netstack.CreateNetTUN(
		iface_addresses,
		dns_addresses,
		mtu)
	if err != nil {
		log.Panic(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, ""))
	err = dev.IpcSet(ipcConfig)
	err = dev.Up()
	if err != nil {
		log.Panic(err)
	}

	return &WireDialer{
		tun:    tun,
		tnet:   tnet,
		Device: dev,
	}, nil
}

func NewWGDialer(wgPrivateKey, wgEndpoint string) (*WGDialer, error) {
	config := fmt.Sprintf(VALID_CONFIG, wgPrivateKey, wgEndpoint)
	reader := strings.NewReader(config)

	d, err := NewDialerFromConfiguration(reader)
	if err != nil {
		return nil, err
	}

	return &WGDialer{
		WireDialer: d,
	}, nil
}
