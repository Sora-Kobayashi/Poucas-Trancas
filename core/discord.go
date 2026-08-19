// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// UDPMode define o que fazer com o UDP, que o Tor nao transporta.
type UDPMode string

const (
	UDPDirect UDPMode = "direct"
	UDPBlock  UDPMode = "block"
)

// Install e um cliente Discord encontrado no disco.
type Install struct {
	Dir     string `json:"dir"`
	Flavor  string `json:"flavor"`
	Exe     string `json:"exe"`
	Running bool   `json:"running"`
}

var flavors = []struct{ dir, exe string }{
	{"Discord", "Discord.exe"},
	{"DiscordCanary", "DiscordCanary.exe"},
	{"DiscordPTB", "DiscordPTB.exe"},
}

var flavorsLinux = []struct{ dir, exe string }{
	{"discord", "Discord"},
	{"discordcanary", "DiscordCanary"},
	{"discordptb", "DiscordPTB"},
}

// ExeNames sao os alvos do desvio.
func ExeNames() []string {
	if runtime.GOOS != "windows" {
		out := make([]string, 0, len(flavorsLinux))
		for _, f := range flavorsLinux {
			out = append(out, f.exe)
		}
		return out
	}
	out := make([]string, 0, len(flavors))
	for _, f := range flavors {
		out = append(out, f.exe)
	}
	return out
}

// FindInstalls varre %LOCALAPPDATA% atras das pastas app-<versao>.
func FindInstalls() []Install {
	if runtime.GOOS != "windows" {
		return findInstallsUnix()
	}
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	procs := runningProcs()

	var found []Install
	for _, f := range flavors {
		matches, _ := filepath.Glob(filepath.Join(local, f.dir, "app-*"))
		sort.Strings(matches)
		for _, dir := range matches {
			exe := filepath.Join(dir, f.exe)
			if _, err := os.Stat(exe); err != nil {
				continue
			}
			found = append(found, Install{
				Dir:     dir,
				Flavor:  f.dir,
				Exe:     exe,
				Running: procs[strings.ToLower(f.exe)],
			})
		}
	}
	return found
}

func RestartDiscord(in Install) error {
	exeName := filepath.Base(in.Exe)
	_ = hiddenCmd("taskkill", "/F", "/IM", exeName, "/T").Run()

	updater := filepath.Join(filepath.Dir(filepath.Dir(in.Exe)), "Update.exe")
	if _, err := os.Stat(updater); err == nil {
		cmd := hiddenCmd(updater, "--processStart", exeName)
		cmd.Dir = filepath.Dir(updater)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}

	cmd := hiddenCmd(in.Exe)
	cmd.Dir = filepath.Dir(in.Exe)
	return cmd.Start()
}

func findInstallsUnix() []Install {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	procs := runningProcs()

	var found []Install
	for _, f := range flavorsLinux {
		for _, base := range []string{
			filepath.Join(home, ".config", f.dir),
			filepath.Join("/usr/share", f.dir),
			filepath.Join("/opt", f.dir),
		} {
			exe := filepath.Join(base, f.exe)
			if _, err := os.Stat(exe); err != nil {
				continue
			}
			found = append(found, Install{
				Dir:     base,
				Flavor:  f.dir,
				Exe:     exe,
				Running: procs[strings.ToLower(f.exe)],
			})
		}
	}
	return found
}
