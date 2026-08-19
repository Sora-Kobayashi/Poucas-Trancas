// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package main

import (
	"embed"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"poucastrancas/core"
	"poucastrancas/core/tray"
	"poucastrancas/core/ui"
)

//go:embed build/windows/icon.ico
var iconFS embed.FS

type App struct {
	mgr *core.Manager
	win *ui.Window

	mu      sync.Mutex
	cfg     core.Config
	tray    *tray.Tray
	quiting bool

	lastRunning bool
	lastExitIP  string
	lastDirect  uint64
}

func NewApp() *App {
	a := &App{cfg: core.LoadConfig()}
	a.mgr = core.NewManager(a.onStatus)
	return a
}

func (a *App) Run() error {
	core.Logf("--- startup ---")
	tray.SetDiagnostic(func(m string) { core.Logf("TRAY: %s", m) })

	a.mgr.SetUpstream(a.cfg.Upstream, a.cfg.SocksURL)
	a.mgr.SetFallbackDirect(a.cfg.FallbackDirect)
	a.mgr.SetBridges(a.cfg.Bridges)

	iconPath := a.extractIcon()
	a.win = ui.New(ui.Callbacks{
		Status:      a.mgr.Status,
		Config:      a.config,
		Start:       a.start,
		Stop:        a.stop,
		NewIdentity: a.newIdentity,
		RefreshExit: a.mgr.RefreshExit,
		SetUpstream: a.setUpstream,
		SetFallback: a.setFallback,
		SetBridges:  a.setBridges,
		SetPrefs:    a.setPrefs,
		Restart:     a.restartDiscord,
		OnClose:     a.onClose,
		Log:         func(s string) { core.Logf("%s", s) },
	})

	a.initTray(iconPath)

	if a.cfg.AutoConnect {
		go a.mgr.Start(core.UDPMode(a.cfg.UDPMode))
	}

	err := a.win.Run(iconPath)

	if t := a.trayRef(); t != nil {
		t.Stop()
	}
	_ = a.mgr.Stop()
	return err
}

func (a *App) extractIcon() string {
	data, err := iconFS.ReadFile("build/windows/icon.ico")
	if err != nil {
		return ""
	}
	if err := os.MkdirAll(core.DataDir(), 0o700); err != nil {
		return ""
	}
	path := filepath.Join(core.DataDir(), "icon.ico")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return ""
	}
	return path
}

// onClose devolve true para esconder na bandeja em vez de sair.
func (a *App) onClose() bool {
	a.mu.Lock()
	quiting, ask := a.quiting, a.cfg.AskOnClose
	a.mu.Unlock()

	if quiting || a.trayRef() == nil {
		return false
	}
	if !ask {
		return true
	}

	msg := "Continuar rodando na bandeja?\n\n" +
		"Sim — some da tela e continua ativo.\n" +
		"Não — encerra o programa."
	if a.mgr.Status().Running {
		msg = "O túnel está ativo.\n\nContinuar rodando na bandeja?\n\n" +
			"Sim — some da tela e o Discord segue pelo túnel.\n" +
			"Não — encerra, e o Discord volta a sair pela sua conexão normal."
	}
	// Erra para o lado que nao destroi estado: so o "nao" explicito encerra.
	if ui.AskYesNo("Poucas Trancas", msg) {
		return true
	}
	a.mu.Lock()
	a.quiting = true
	a.mu.Unlock()
	return false
}

func (a *App) config() core.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) saveCfg(mut func(*core.Config)) {
	a.mu.Lock()
	mut(&a.cfg)
	snapshot := a.cfg
	a.mu.Unlock()
	_ = core.SaveConfig(snapshot)
}

func (a *App) start(mode string) string {
	core.Logf("Conectar: modo=%s", mode)
	m := core.UDPDirect
	if core.UDPMode(mode) == core.UDPBlock {
		m = core.UDPBlock
	}
	a.saveCfg(func(c *core.Config) { c.UDPMode = string(m) })
	if err := a.mgr.Start(m); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) stop() string {
	core.Logf("Desconectar")
	if err := a.mgr.Stop(); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) newIdentity() string {
	if err := a.mgr.NewIdentity(); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) setUpstream(kind, url string) {
	a.saveCfg(func(c *core.Config) { c.Upstream, c.SocksURL = kind, url })
	a.mgr.SetUpstream(kind, url)
}

func (a *App) setFallback(v bool) {
	a.saveCfg(func(c *core.Config) { c.FallbackDirect = v })
	a.mgr.SetFallbackDirect(v)
}

func (a *App) setBridges(text string) int {
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			lines = append(lines, l)
		}
	}
	a.saveCfg(func(c *core.Config) { c.Bridges = lines })
	a.mgr.SetBridges(lines)
	return len(lines)
}

func (a *App) setPrefs(ask, auto, notify bool) {
	a.saveCfg(func(c *core.Config) {
		c.AskOnClose, c.AutoConnect, c.Notify = ask, auto, notify
	})
}

func (a *App) restartDiscord(dir string) string {
	for _, in := range core.FindInstalls() {
		if in.Dir == dir {
			if err := core.RestartDiscord(in); err != nil {
				return err.Error()
			}
			return ""
		}
	}
	return "instalação não encontrada: " + dir
}
