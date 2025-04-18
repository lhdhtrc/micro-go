package micro

import (
	"net"
	"strings"
)

type Network struct {
	SN       string `json:"sn"`
	Internal string `json:"internal"`
	External string `json:"external"`
}

// GetInternalNetworkIp 获取内网ip
func GetInternalNetworkIp() string {
	dial, err := net.Dial("udp", "114.114.114.114:80")
	if err != nil {
		return "127.0.0.1"
	}
	addr := dial.LocalAddr().String()

	index := strings.LastIndex(addr, ":")
	return addr[:index]
}
