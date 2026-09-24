package config

import (
	"encoding/json"
	"testing"
)

func TestResolveConfig(t *testing.T) {
	tests := []struct {
		name       string
		jsonInput  string
		expectedIP string
		expectedPort int
		expectedURL string
	}{
		{
			name: "server_ip alone defaults to port 8080",
			jsonInput: `{"server_ip": "192.168.1.50"}`,
			expectedIP: "192.168.1.50",
			expectedPort: 8080,
			expectedURL: "http://192.168.1.50:8080",
		},
		{
			name: "server_ip with server_port",
			jsonInput: `{"server_ip": "192.168.1.50", "server_port": 9090}`,
			expectedIP: "192.168.1.50",
			expectedPort: 9090,
			expectedURL: "http://192.168.1.50:9090",
		},
		{
			name: "server_ip with port embedded in string",
			jsonInput: `{"server_ip": "10.10.10.10:7777"}`,
			expectedIP: "10.10.10.10",
			expectedPort: 7777,
			expectedURL: "http://10.10.10.10:7777",
		},
		{
			name: "server_url directly provided",
			jsonInput: `{"server_url": "http://srv.empresa.local:8080"}`,
			expectedIP: "srv.empresa.local",
			expectedPort: 8080,
			expectedURL: "http://srv.empresa.local:8080",
		},
		{
			name: "empty json fallback to localhost",
			jsonInput: `{}`,
			expectedIP: "127.0.0.1",
			expectedPort: 8080,
			expectedURL: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fc fileConfig
			if err := json.Unmarshal([]byte(tt.jsonInput), &fc); err != nil {
				t.Fatalf("Erro ao decodificar JSON de teste: %v", err)
			}
			res := resolveConfig(fc)
			if res.ServerIP != tt.expectedIP {
				t.Errorf("ServerIP: esperado '%s', obteve '%s'", tt.expectedIP, res.ServerIP)
			}
			if res.ServerPort != tt.expectedPort {
				t.Errorf("ServerPort: esperado %d, obteve %d", tt.expectedPort, res.ServerPort)
			}
			if res.ServerURL != tt.expectedURL {
				t.Errorf("ServerURL: esperado '%s', obteve '%s'", tt.expectedURL, res.ServerURL)
			}
		})
	}
}

func TestSetServerURLDisabled(t *testing.T) {
	err := SetServerURL("http://localhost:9999")
	if err == nil {
		t.Errorf("SetServerURL deveria ter retornado erro")
	}
}
