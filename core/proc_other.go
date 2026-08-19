//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func hiddenCmd(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }

func runningProcs() map[string]bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil {
			continue
		}
		set[strings.ToLower(strings.TrimSpace(string(b)))] = true
	}
	return set
}
