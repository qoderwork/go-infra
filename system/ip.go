package system

import (
	"net"
	"strconv"
	"strings"
)

// PortUsed 检测端口  true:已使用;false:未使用
func PortUsed(mode string, port int) bool {
	if port > 65535 || port < 0 {
		return true
	}

	switch strings.ToLower(mode) {
	case "tcp":
		return tcpPortUsed(port)
	default:
		return udpPortUsed(port)
	}
}

func tcpPortUsed(port int) bool {
	addr, _ := net.ResolveTCPAddr("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	conn, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}

func udpPortUsed(port int) bool {
	addr, _ := net.ResolveUDPAddr("udp", net.JoinHostPort("", strconv.Itoa(port)))
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return true
	}
	_ = conn.Close()
	return false
}
