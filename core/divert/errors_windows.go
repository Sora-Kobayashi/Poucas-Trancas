//go:build windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package divert

import "errors"

// ErrUnsupported existe nas duas plataformas para o codigo comum poder
// compara-la sem condicional.
var ErrUnsupported = errors.New("WinDivert indisponível")
