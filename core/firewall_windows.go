//go:build windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"fmt"
	"os"
	"strings"
)

const ruleTag = "poucastrancas-block-udp"

func discordExes(installs []Install) []string {
	var exes []string
	for _, in := range installs {
		if fileExists(in.Exe) {
			exes = append(exes, in.Exe)
		}
	}
	return exes
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func BlockUDP(installs []Install) error {
	exes := discordExes(installs)
	if len(exes) == 0 {
		return fmt.Errorf("nenhum executavel do Discord encontrado")
	}
	var errs []string
	for i, exe := range exes {
		name := fmt.Sprintf("%s-%d", ruleTag, i)
		out, err := hiddenCmd("netsh", "advfirewall", "firewall", "add", "rule",
			"name="+name, "dir=out", "action=block", "protocol=UDP",
			"program="+exe, "enable=yes").CombinedOutput()
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v (%s)", exe, err, strings.TrimSpace(string(out))))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("precisa de Administrador — %s", strings.Join(errs, "; "))
	}
	return nil
}

func UnblockUDP() error {
	for i := 0; i < 8; i++ {
		_ = hiddenCmd("netsh", "advfirewall", "firewall", "delete", "rule",
			fmt.Sprintf("name=%s-%d", ruleTag, i)).Run()
	}
	return nil
}
