package main

import (
	"net"
)

func nonLoopbackIPv4() (string, bool) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", false
	}
	for _, iface := range ifaces {
		if ip, ok := ifaceNonLoopbackIPv4(iface); ok {
			return ip, true
		}
	}
	return "", false
}

func ifaceNonLoopbackIPv4(iface net.Interface) (string, bool) {
	if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
		return "", false
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return "", false
	}
	return firstNonLoopbackIPv4(addrs)
}

func firstNonLoopbackIPv4(addrs []net.Addr) (string, bool) {
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip := ipnet.IP.To4()
		if ip == nil {
			continue
		}
		return ip.String(), true
	}
	return "", false
}
