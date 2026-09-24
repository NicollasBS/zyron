package web

import (
	_ "embed"
)

// ManagerHTML contains the embedded HTML/JS single-page application for the manager dashboard
//
//go:embed manager.html
var ManagerHTML []byte
