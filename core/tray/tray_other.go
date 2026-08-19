//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package tray

// Fora do Windows a bandeja ainda nao tem implementacao: seria DBus com
// StatusNotifierItem. As chamadas viram no-op para o resto do programa
// continuar funcionando sem condicional espalhada.

type Level int

const (
	Info Level = iota
	Warning
	Error
)

type Item struct {
	Label    string
	Disabled bool
	Sep      bool
	OnClick  func()
}

var onPanic func(string)

// SetDiagnostic instala o coletor de falhas da bandeja.
func SetDiagnostic(f func(string)) { onPanic = f }

type Tray struct{}

func New(tip, iconPath string, onClick func(), items []Item) (*Tray, error) {
	return &Tray{}, nil
}

func (t *Tray) SetTooltip(string)            {}
func (t *Tray) SetItems([]Item)              {}
func (t *Tray) Notify(string, string, Level) {}
func (t *Tray) Stop()                        {}
