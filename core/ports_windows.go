//go:build windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func killOrphanTor(exePath string) int {
	want := strings.ToLower(filepath.Clean(exePath))
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(snap)

	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	if err := windows.Process32First(snap, &e); err != nil {
		return 0
	}

	killed := 0
	self := uint32(windows.GetCurrentProcessId())
	for {
		name := strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))
		if name == "tor.exe" && e.ProcessID != self {
			if strings.ToLower(filepath.Clean(processPath(e.ProcessID))) == want {
				if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, e.ProcessID); err == nil {
					_ = windows.TerminateProcess(h, 1)
					windows.CloseHandle(h)
					killed++
				}
			}
		}
		if err := windows.Process32Next(snap, &e); err != nil {
			break
		}
	}
	return killed
}

func processPath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:size])
}
