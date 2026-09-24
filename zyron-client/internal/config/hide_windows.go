//go:build windows

package config

import (
	"syscall"
)

// hideConfigFile garante que o arquivo de configuração possua o atributo 'Oculto' (Hidden) no Windows.
func hideConfigFile(path string) {
	if path == "" {
		return
	}
	ptr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, err := syscall.GetFileAttributes(ptr)
	if err != nil {
		return
	}
	if attrs&syscall.FILE_ATTRIBUTE_HIDDEN == 0 {
		_ = syscall.SetFileAttributes(ptr, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
	}
}
