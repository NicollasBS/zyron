package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Config representa a configuração do cliente Zyron
type Config struct {
	ServerIP   string `json:"server_ip"`
	ServerPort int    `json:"server_port"`
	ServerURL  string `json:"server_url"`
}

type fileConfig struct {
	ServerIP   string          `json:"server_ip"`
	IP         string          `json:"ip"`
	ServerHost string          `json:"server_host"`
	Host       string          `json:"host"`
	ServerPort json.RawMessage `json:"server_port"`
	Port       json.RawMessage `json:"port"`
	ServerURL  string          `json:"server_url"`
	URL        string          `json:"url"`
}

var (
	loadedPath string
	current    Config
	mu         sync.RWMutex
)

func init() {
	Load()
}

// Load recarrega a configuração procurando pelo arquivo config.json
func Load() Config {
	mu.Lock()
	defer mu.Unlock()

	pathsToTry := getConfigCandidates()

	var foundPath string
	var loaded fileConfig
	var found bool

	for _, p := range pathsToTry {
		if data, err := os.ReadFile(p); err == nil {
			if err := json.Unmarshal(data, &loaded); err == nil {
				foundPath = p
				found = true
				break
			}
		}
	}

	if found {
		loadedPath = foundPath
		current = resolveConfig(loaded)
		log.Printf("[Config] Carregado de '%s': IP=%s, Port=%d, URL=%s\n",
			foundPath, current.ServerIP, current.ServerPort, current.ServerURL)
	} else {
		defaultIP := "127.0.0.1"
		defaultURL := "http://localhost:8080"

		if envURL := os.Getenv("ZYRON_SERVER_URL"); envURL != "" {
			defaultURL = envURL
		}
		if envIP := os.Getenv("ZYRON_SERVER_IP"); envIP != "" {
			defaultIP = envIP
		}

		current = resolveConfig(fileConfig{
			ServerIP:  defaultIP,
			ServerURL: defaultURL,
		})
		log.Printf("[Config] Nenhum config.json encontrado. Usando padrão: IP=%s, Port=%d, URL=%s\n",
			current.ServerIP, current.ServerPort, current.ServerURL)
	}

	return current
}

func getConfigCandidates() []string {
	var candidates []string

	// 1. Diretório do executável (build/bin ou pasta instalada)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "config.json"))
	}

	// 2. Diretório de trabalho atual
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "config.json"))
	}

	// 3. Diretório de configuração do usuário (%APPDATA%/ZyronClient/config.json)
	if userConfigDir, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(userConfigDir, "ZyronClient", "config.json"))
	}

	return candidates
}

func parsePort(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var pInt int
	if err := json.Unmarshal(raw, &pInt); err == nil && pInt > 0 {
		return pInt
	}
	var pStr string
	if err := json.Unmarshal(raw, &pStr); err == nil {
		pStr = strings.TrimSpace(pStr)
		if n, err := strconv.Atoi(pStr); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func resolveConfig(fc fileConfig) Config {
	ip := strings.TrimSpace(fc.ServerIP)
	if ip == "" {
		ip = strings.TrimSpace(fc.IP)
	}
	if ip == "" {
		ip = strings.TrimSpace(fc.ServerHost)
	}
	if ip == "" {
		ip = strings.TrimSpace(fc.Host)
	}

	port := parsePort(fc.ServerPort)
	if port == 0 {
		port = parsePort(fc.Port)
	}

	rawURL := strings.TrimSpace(fc.ServerURL)
	if rawURL == "" {
		rawURL = strings.TrimSpace(fc.URL)
	}

	var finalURL string
	var finalIP string
	var finalPort int = port

	if rawURL != "" {
		finalURL = strings.TrimRight(rawURL, "/")
		if !strings.HasPrefix(finalURL, "http://") && !strings.HasPrefix(finalURL, "https://") {
			finalURL = "http://" + finalURL
		}
		if u, err := url.Parse(finalURL); err == nil {
			finalIP = u.Hostname()
			if pStr := u.Port(); pStr != "" {
				if pInt, err := strconv.Atoi(pStr); err == nil {
					finalPort = pInt
				}
			}
		}
	} else if ip != "" {
		scheme := "http"
		cleanIP := ip
		if strings.HasPrefix(cleanIP, "http://") {
			cleanIP = strings.TrimPrefix(cleanIP, "http://")
		} else if strings.HasPrefix(cleanIP, "https://") {
			scheme = "https"
			cleanIP = strings.TrimPrefix(cleanIP, "https://")
		}
		cleanIP = strings.TrimRight(cleanIP, "/")

		if host, pStr, err := net.SplitHostPort(cleanIP); err == nil {
			finalIP = host
			if pInt, err := strconv.Atoi(pStr); err == nil && pInt > 0 {
				finalPort = pInt
			}
		} else {
			finalIP = cleanIP
		}

		if finalPort == 0 {
			finalPort = 8080
		}
		finalURL = fmt.Sprintf("%s://%s:%d", scheme, finalIP, finalPort)
	} else {
		finalIP = "127.0.0.1"
		finalPort = 8080
		finalURL = "http://localhost:8080"
	}

	if finalIP == "" {
		finalIP = "127.0.0.1"
	}
	if finalPort == 0 {
		finalPort = 8080
	}

	return Config{
		ServerIP:   finalIP,
		ServerPort: finalPort,
		ServerURL:  finalURL,
	}
}

// GetConfig retorna uma cópia da configuração atual
func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// GetLoadedPath retorna o caminho do arquivo de configuração carregado (se houver)
func GetLoadedPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return loadedPath
}

// SetServerURL está desabilitado pois a configuração deve ser feita exclusivamente via arquivo JSON
func SetServerURL(serverURL string) error {
	return errors.New("a alteração de servidor pelo aplicativo está desabilitada; configure exclusivamente através do arquivo config.json")
}
