//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import "os"

// IsElevated diz se o processo roda como root.
func IsElevated() bool { return os.Geteuid() == 0 }
