//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func killOrphanTor(exePath string) int {
	want, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		want = exePath
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	self := os.Getpid()
	killed := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", e.Name(), "exe"))
		if err != nil || !strings.EqualFold(exe, want) {
			continue
		}
		if syscall.Kill(pid, syscall.SIGKILL) == nil {
			killed++
		}
	}
	return killed
}

func KillOrphanTorFor(exePath string) int { return killOrphanTor(exePath) }
