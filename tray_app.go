// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package main

// Bandeja e notificacoes: o que a janela nao cobre quando ela esta
// escondida.

import (
	"fmt"

	"poucastrancas/core"
	"poucastrancas/core/tray"
)

func (a *App) trayRef() *tray.Tray {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tray
}

func (a *App) initTray(iconPath string) {
	t, err := tray.New("Poucas Trancas — parado", iconPath, a.toggleWindow, nil)
	if err != nil {
		core.Logf("bandeja FALHOU: %v", err)
		return
	}
	core.Logf("bandeja: ok")
	a.mu.Lock()
	a.tray = t
	a.mu.Unlock()
	a.refreshTrayMenu(a.mgr.Status())
}

func (a *App) refreshTrayMenu(s core.Status) {
	t := a.trayRef()
	if t == nil {
		return
	}
	action := "Conectar"
	if s.Running {
		action = "Desconectar"
	}
	t.SetItems([]tray.Item{
		{Label: "Mostrar janela", OnClick: func() { a.win.Show() }},
		{Label: action, OnClick: a.toggleConnection},
		{Sep: true},
		{Label: "Sair", OnClick: a.quit},
	})
}

func (a *App) toggleWindow() {
	if a.win.Visible() {
		a.win.Hide()
	} else {
		a.win.Show()
	}
}

func (a *App) toggleConnection() {
	if a.mgr.Status().Running {
		_ = a.mgr.Stop()
		return
	}
	a.mu.Lock()
	mode := a.cfg.UDPMode
	a.mu.Unlock()
	_ = a.mgr.Start(core.UDPMode(mode))
}

func (a *App) quit() {
	a.mu.Lock()
	a.quiting = true
	a.mu.Unlock()
	a.win.Close()
}

func (a *App) notify(title, body string, level tray.Level) {
	a.mu.Lock()
	on := a.cfg.Notify
	a.mu.Unlock()
	if !on {
		return
	}
	if t := a.trayRef(); t != nil {
		t.Notify(title, body, level)
	}
}

// onStatus so dispara nas transicoes: a janela sonda a cada segundo, e
// notificar por sondagem viraria spam.
func (a *App) onStatus(s core.Status) {
	t := a.trayRef()
	if t == nil {
		return
	}

	a.mu.Lock()
	wasRunning, lastIP, lastDirect := a.lastRunning, a.lastExitIP, a.lastDirect
	a.lastRunning, a.lastExitIP, a.lastDirect = s.Running, s.ExitIP, s.Intercept.Direct
	a.mu.Unlock()

	switch {
	case s.Running && !wasRunning:
		t.SetTooltip("Poucas Trancas — conectado")
	case !s.Running && wasRunning:
		t.SetTooltip("Poucas Trancas — parado")
		a.notify("Desconectado", "O Discord voltou a sair pela sua conexão normal.", tray.Warning)
	case s.Running && s.ExitIP != "" && s.ExitIP != lastIP:
		label := s.ExitIP
		if s.ExitLoc != "" {
			label += " (" + s.ExitLoc + ")"
		}
		t.SetTooltip("Poucas Trancas — " + label)
		if lastIP == "" {
			a.notify("Conectado", "Saindo por "+label, tray.Info)
		}
	}

	if s.Intercept.Direct > lastDirect {
		a.notify("Vazamento de IP",
			fmt.Sprintf("%d conexão(ões) saíram por fora da rota anônima.", s.Intercept.Direct),
			tray.Error)
	}
	if s.Err != "" && !wasRunning && !s.Running {
		a.notify("Falhou", s.Err, tray.Error)
	}

	a.refreshTrayMenu(s)
}
