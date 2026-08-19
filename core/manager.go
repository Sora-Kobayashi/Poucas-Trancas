// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"poucastrancas/core/redirect"
	"poucastrancas/torbin"
)

const traceURL = "https://1.1.1.1/cdn-cgi/trace"

// Status e o retrato consumido pela interface.
type Status struct {
	Running        bool           `json:"running"`
	Bootstrap      int            `json:"bootstrap"`
	BootstrapMsg   string         `json:"bootstrapMsg"`
	ExitIP         string         `json:"exitIp"`
	ExitLoc        string         `json:"exitLoc"`
	UDPMode        string         `json:"udpMode"`
	Intercept      InterceptStats `json:"intercept"`
	Proxy          string         `json:"proxy"`
	TorEmbedded    bool           `json:"torEmbedded"`
	Elevated       bool           `json:"elevated"`
	Bridges        int            `json:"bridges"`
	FallbackDirect bool           `json:"fallbackDirect"`
	Upstream       string         `json:"upstream"`
	SocksURL       string         `json:"socksUrl"`
	Installs       []Install      `json:"installs"`
	Err            string         `json:"err"`
}

type Manager struct {
	mu       sync.Mutex
	bridgeLn []string
	fbDirect bool
	upstream Upstream
	socksURL string
	tor      *Tor
	icept    *interceptor
	st       Status
	onStat   func(Status)
}

func NewManager(onStatus func(Status)) *Manager {
	m := &Manager{onStat: onStatus}
	m.st.UDPMode = string(UDPDirect)
	m.upstream = UpTor
	m.st.TorEmbedded = torbin.Available()
	m.st.Elevated = IsElevated()
	m.st.Installs = FindInstalls()
	return m
}

func DataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "poucastrancas")
}

func (m *Manager) Status() Status {
	// FindInstalls varre disco e processos; feito FORA do lock para nao
	// segurar as atualizacoes de progresso do Tor, que disputam o mesmo mutex.
	installs := FindInstalls()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.st.Installs = installs
	m.st.Intercept = m.icept.stats()
	return m.st
}

func (m *Manager) set(mut func(*Status)) {
	m.mu.Lock()
	mut(&m.st)
	snap := m.st
	m.mu.Unlock()
	if m.onStat != nil {
		m.onStat(snap)
	}
}

// Start extrai o Tor embutido, espera o bootstrap e configura o Discord.
func (m *Manager) Start(mode UDPMode) error {
	m.mu.Lock()
	if m.st.Running {
		m.mu.Unlock()
		return fmt.Errorf("ja esta rodando")
	}
	m.mu.Unlock()

	m.set(func(s *Status) {
		s.Err = ""
		s.Bootstrap = 0
		s.BootstrapMsg = "extraindo Tor embutido..."
		s.UDPMode = string(mode)
	})

	Logf("Manager.Start: modo=%s", mode)
	torExe, err := torbin.Extract(filepath.Join(DataDir(), "tor"))
	if err != nil {
		Logf("Manager.Start: extrair Tor falhou: %v", err)
		m.set(func(s *Status) { s.Err = err.Error(); s.BootstrapMsg = "" })
		return err
	}

	if n := killOrphanTor(torExe); n > 0 {
		m.set(func(s *Status) { s.BootstrapMsg = fmt.Sprintf("encerrado %d tor orfao", n) })
	}

	socksPort, ctrlPort, err := freePortPair()
	if err != nil {
		m.set(func(s *Status) { s.Err = err.Error(); s.BootstrapMsg = "" })
		return err
	}

	t := NewTor(torExe, DataDir(), socksPort, ctrlPort)
	t.Lyrebird = filepath.Join(filepath.Dir(torExe), "lyrebird.exe")
	t.Bridges = m.bridges()
	m.set(func(s *Status) { s.BootstrapMsg = "conectando a rede Tor..." })

	Logf("Manager.Start: subindo Tor em %d/%d", socksPort, ctrlPort)
	err = t.Start(func(pct int, line string) {
		m.set(func(s *Status) {
			s.Bootstrap = pct
			s.BootstrapMsg = shortBootstrap(line)
		})
	})
	if err != nil {
		Logf("Manager.Start: Tor falhou: %v", err)
		m.set(func(s *Status) { s.Err = err.Error(); s.BootstrapMsg = "" })
		return err
	}
	Logf("Manager.Start: Tor pronto")

	installs := FindInstalls()

	m.set(func(s *Status) { s.BootstrapMsg = "preparando WinDivert..." })
	if e := EnsureWinDivert(func(msg string) {
		m.set(func(s *Status) { s.BootstrapMsg = msg })
	}); e != nil {
		t.Stop()
		m.set(func(s *Status) { s.Err = e.Error(); s.BootstrapMsg = "" })
		return e
	}
	dial, label, up := m.upstreamDialer(t)
	ic, e := m.startIntercept(dial, func(msg string) {
		m.set(func(s *Status) { s.BootstrapMsg = msg })
	}, m.fallbackDirect(), up == UpTor)
	if e != nil {
		t.Stop()
		m.set(func(s *Status) { s.Err = e.Error(); s.BootstrapMsg = "" })
		return e
	}
	m.mu.Lock()
	m.icept = ic
	m.mu.Unlock()
	proxy := fmt.Sprintf("WinDivert -> %s", label)

	if mode == UDPBlock {
		if e := BlockUDP(installs); e != nil {
			m.set(func(s *Status) { s.Err = "UDP nao foi bloqueado (precisa de Administrador): " + e.Error() })
		}
	} else {
		_ = UnblockUDP()
	}

	m.mu.Lock()
	m.tor = t
	m.mu.Unlock()
	m.set(func(s *Status) {
		s.Running = true
		s.Proxy = proxy
		s.BootstrapMsg = "pronto"
	})

	go m.refreshExit()
	return nil
}

// Stop derruba o Tor e devolve o Discord ao normal.
func (m *Manager) Stop() error {
	m.mu.Lock()
	t := m.tor
	ic := m.icept
	m.tor, m.icept = nil, nil
	m.mu.Unlock()

	ic.stop()
	if t != nil {
		t.Stop()
	}
	_ = UnblockUDP()

	m.set(func(s *Status) {
		s.Running = false
		s.Bootstrap = 0
		s.BootstrapMsg = ""
		s.ExitIP = ""
		s.ExitLoc = ""
		s.Proxy = ""
	})
	return nil
}

// NewIdentity troca o circuito — util quando o no de saida esta bloqueado.
func (m *Manager) NewIdentity() error {
	m.mu.Lock()
	t := m.tor
	m.mu.Unlock()
	if t == nil {
		return fmt.Errorf("o Tor nao esta rodando")
	}
	if err := t.NewIdentity(); err != nil {
		return err
	}
	m.set(func(s *Status) { s.ExitIP = "trocando circuito..."; s.ExitLoc = "" })
	go func() {
		time.Sleep(4 * time.Second)
		m.refreshExit()
	}()
	return nil
}

func (m *Manager) refreshExit() {
	m.mu.Lock()
	t := m.tor
	m.mu.Unlock()
	if t == nil {
		return
	}
	ip, loc, err := exitIP(t)
	m.set(func(s *Status) {
		if err != nil {
			s.ExitIP = "nao confirmado"
			s.ExitLoc = ""
			return
		}
		s.ExitIP = ip
		s.ExitLoc = loc
	})
}

// SetBridges guarda as linhas de ponte coladas pelo usuario. Vazio volta
// a conexao direta.
func (m *Manager) SetBridges(lines []string) {
	m.mu.Lock()
	m.bridgeLn = lines
	m.mu.Unlock()
	m.set(func(s *Status) { s.Bridges = len(lines) })
}

func (m *Manager) bridges() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.bridgeLn...)
}

// SetFallbackDirect liga/desliga a discagem fora do Tor para conexoes que
// ele recusa. Vale na proxima conexao.
func (m *Manager) SetFallbackDirect(v bool) {
	m.mu.Lock()
	m.fbDirect = v
	m.mu.Unlock()
	m.set(func(s *Status) { s.FallbackDirect = v })
}

func (m *Manager) fallbackDirect() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.fbDirect
}

// SetUpstream escolhe por onde o trafego desviado sai. Vazio em socksURL
// mantem o Tor.
func (m *Manager) SetUpstream(kind, socksURL string) {
	m.mu.Lock()
	if Upstream(kind) == UpSocks && strings.TrimSpace(socksURL) != "" {
		m.upstream, m.socksURL = UpSocks, strings.TrimSpace(socksURL)
	} else {
		m.upstream, m.socksURL = UpTor, ""
	}
	up, url := m.upstream, m.socksURL
	m.mu.Unlock()
	m.set(func(s *Status) { s.Upstream = string(up); s.SocksURL = url })
}

func (m *Manager) upstreamDialer(t *Tor) (redirect.DialFunc, string, Upstream) {
	m.mu.Lock()
	up, url := m.upstream, m.socksURL
	m.mu.Unlock()
	if up == UpSocks && url != "" {
		return socksDialer(url), "SOCKS5 " + url, UpSocks
	}
	return t.Dial, "Tor (IPv4 forcado)", UpTor
}

// RefreshExit expoe a checagem para a interface.
func (m *Manager) RefreshExit() { m.refreshExit() }

func exitIP(t *Tor) (ip, loc string, err error) {
	client := &http.Client{
		Timeout:   45 * time.Second,
		Transport: &http.Transport{Dial: t.Dial},
	}
	resp, err := client.Get(traceURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	for _, line := range strings.Split(string(buf), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "ip":
			ip = v
		case "loc":
			loc = v
		}
	}
	if ip == "" {
		return "", "", fmt.Errorf("resposta inesperada do trace")
	}
	return ip, loc, nil
}

func shortBootstrap(line string) string {
	if i := strings.Index(line, "Bootstrapped"); i >= 0 {
		s := line[i:]
		if j := strings.Index(s, "("); j >= 0 {
			if k := strings.Index(s[j:], ")"); k > 0 {
				return strings.TrimSpace(s[j+1 : j+k])
			}
		}
		return strings.TrimSpace(s)
	}
	return ""
}
