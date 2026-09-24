package system

import (
	"net"
	"os"
	"os/exec"
	"strings"
)

type SystemInfo struct {
	Username string `json:"username"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

// GetCurrentUser executa whoami para obter o formato DOMINIO\usuario no Windows
func GetCurrentUser() string {
	cmd := exec.Command("whoami")
	out, err := cmd.Output()
	if err == nil {
		user := strings.TrimSpace(string(out))
		if user != "" {
			return user
		}
	}
	// Fallback para variáveis de ambiente
	domain := os.Getenv("USERDOMAIN")
	username := os.Getenv("USERNAME")
	if domain != "" && username != "" {
		return domain + "\\" + username
	}
	if username != "" {
		return username
	}
	return "UNKNOWN_USER"
}

// GetHostname retorna o nome da máquina local
func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "UNKNOWN_HOST"
	}
	return hostname
}

// GetLocalIPv4 retorna o IP local ativo da máquina na rede LAN
func GetLocalIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return localAddr.IP.String()
		}
	}

	// Fallback para varredura de interfaces
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ipNet.IP.To4() != nil {
					return ipNet.IP.String()
				}
			}
		}
	}

	return "127.0.0.1"
}

// GetSystemInfo reúne todas as informações da máquina
func GetSystemInfo() SystemInfo {
	return SystemInfo{
		Username: GetCurrentUser(),
		Hostname: GetHostname(),
		IP:       GetLocalIPv4(),
	}
}
