//go:build !debug

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

// Build de release: nao grava log em disco. O programa so escreve o que
// precisa para funcionar (config, binarios extraidos), nada de diagnostico.

func LogPath() string     { return "" }
func Logf(string, ...any) {}
