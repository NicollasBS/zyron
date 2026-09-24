//go:build !windows

package config

// hideConfigFile em sistemas não-Windows não requer chamadas de API Win32
func hideConfigFile(path string) {
	// No-op em plataformas não-Windows
}
