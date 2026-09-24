package system

import (
	"strings"
	"testing"
)

func TestGetCurrentUser(t *testing.T) {
	user := GetCurrentUser()
	if strings.TrimSpace(user) == "" {
		t.Errorf("GetCurrentUser() retornou vazio")
	}
	t.Logf("Usuário atual detectado: %s", user)
}

func TestGetHostname(t *testing.T) {
	hostname := GetHostname()
	if strings.TrimSpace(hostname) == "" {
		t.Errorf("GetHostname() retornou vazio")
	}
	t.Logf("Hostname detectado: %s", hostname)
}

func TestGetLocalIPv4(t *testing.T) {
	ip := GetLocalIPv4()
	if strings.TrimSpace(ip) == "" {
		t.Errorf("GetLocalIPv4() retornou vazio")
	}
	t.Logf("IP Local detectado: %s", ip)
}

func TestGetSystemInfo(t *testing.T) {
	info := GetSystemInfo()
	if info.Username == "" || info.Hostname == "" || info.IP == "" {
		t.Errorf("GetSystemInfo() retornou campos vazios: %+v", info)
	}
}
