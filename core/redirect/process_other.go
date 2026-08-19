//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package redirect

import (
	"os"
	"path/filepath"
	"strconv"
)

func processName(pid uint32) string {
	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(int(pid)), "exe"))
	if err != nil {
		return ""
	}
	return filepath.Base(exe)
}
