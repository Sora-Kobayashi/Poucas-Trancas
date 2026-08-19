// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package ui

import "unsafe"

const (
	mbYesNo        = 0x00000004
	mbIconQuestion = 0x00000020
	mbDefButton1   = 0x00000000
	idYes          = 6
)

// AskYesNo mostra um Sim/Nao nativo. Devolve true para Sim.
func AskYesNo(title, message string) bool {
	r, _, _ := pMessageBox.Call(0,
		uintptr(unsafe.Pointer(utf16ptr(message))),
		uintptr(unsafe.Pointer(utf16ptr(title))),
		mbYesNo|mbIconQuestion|mbDefButton1)
	return int(r) == idYes
}
