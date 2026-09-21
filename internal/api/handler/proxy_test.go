package handler

import (
	"net"
	"testing"
)

func TestBlockedNet(t *testing.T) {
	for ip, want := range map[string]bool{
		"127.0.0.1": true, "10.1.2.3": true, "172.16.5.5": true, "192.168.1.1": true,
		"100.64.0.1": true, "169.254.1.1": true, "::1": true, "fe80::1": true,
		"78.47.228.253": false, "2606:4700::1": false,
	} {
		if got := blockedNet(net.ParseIP(ip)); got != want {
			t.Errorf("%s: got %v want %v", ip, got, want)
		}
	}
}
