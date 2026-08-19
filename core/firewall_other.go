//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import "errors"

// Bloquear UDP por processo em Linux exige nftables com cgroup ou regra por
// uid — nenhum dos dois esta implementado. Falha explicita e melhor do que
// dizer que bloqueou sem ter bloqueado.
func BlockUDP([]Install) error {
	return errors.New("bloqueio de UDP por processo ainda não implementado em Linux")
}
func UnblockUDP() error { return nil }
