//go:build debug

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var logMu sync.Mutex

func LogPath() string { return filepath.Join(DataDir(), "debug.log") }

// Logf so existe no build com -tags debug. Compilar sem a tag remove a
// funcao inteira, entao nem a string do caminho fica no binario de release.
func Logf(format string, a ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	if err := os.MkdirAll(DataDir(), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
}
