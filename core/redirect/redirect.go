// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// Package redirect desvia o TCP de processos escolhidos para um proxy
// local, sem injetar nada neles.
//
// # COMO FUNCIONA
//
// Duas escutas do WinDivert trabalham juntas:
//
//	SOCKET  (sniff)  descobre qual PID abriu cada porta local
//	NETWORK (desvio) reescreve os pacotes desses fluxos
//
// O truque da reescrita e o padrao classico de proxy transparente:
//
//	saida:   Discord:P -> Destino:D    vira   127.0.0.1:P -> 127.0.0.1:proxy
//	volta:   127.0.0.1:proxy -> :P     vira   Destino:D   -> Discord:P
//
// A tabela NAT guarda o destino original indexado pela porta de origem P,
// que e unica por conexao. O proxy local consulta essa tabela para saber
// para onde a conexao ia de verdade e entao disca por Tor/WARP.
//
// LIMITES CONHECIDOS
//
//   - IPv4 e IPv6, mas so TCP. UDP (voz e tela) passa direto — o Tor nao
//     transporta UDP de qualquer forma, entao desviar nao adiantaria.
//   - Precisa de Administrador: carrega driver.
package redirect

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"poucastrancas/core/divert"
)

const (
	natTTL   = 10 * time.Minute
	sweepGap = 30 * time.Second

	maxPacket = 0xFFFF
)

const loopbackIfIdx = 1

type natKey struct {
	port uint16
	v6   bool
}

// Conn e o destino original de uma conexao desviada.
type Conn struct {
	DstIP   net.IP
	DstPort uint16
	SrcIP   net.IP
	PID     uint32

	ifIdx, subIfIdx uint32

	lastSeen time.Time
}

type Stats struct {
	Redirected  uint64 `json:"redirected"`
	Returned    uint64 `json:"returned"`
	Redirected6 uint64 `json:"redirected6"`
	DroppedV6   uint64 `json:"droppedV6"`
	SkippedUDP  uint64 `json:"skippedUdp"`
	SendErr     uint64 `json:"sendErr"`
	LastErr     string `json:"lastErr"`
	Active      int    `json:"active"`
}

type Redirector struct {
	proxyPort uint16

	ForceIPv4 atomic.Bool
	targets   map[string]bool

	mu    sync.RWMutex
	nat   map[natKey]*Conn
	pids  map[uint32]bool
	ports map[uint16]uint32

	hNet  *divert.Handle
	hSock *divert.Handle

	stats struct {
		redirected, redirected6, returned, skippedUDP, sendErr, droppedV6 atomic.Uint64
		lastErr                                                           atomic.Value
	}

	stop chan struct{}
	wg   sync.WaitGroup
}

func New(proxyPort uint16, exeNames []string) *Redirector {
	t := map[string]bool{}
	for _, n := range exeNames {
		t[lower(n)] = true
	}
	return &Redirector{
		proxyPort: proxyPort,
		targets:   t,
		nat:       map[natKey]*Conn{},
		pids:      map[uint32]bool{},
		ports:     map[uint16]uint32{},
		stop:      make(chan struct{}),
	}
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

// Lookup devolve o destino original de uma conexao que chegou no proxy.
// O proxy chama isto com a porta de origem do socket que ele aceitou.
func (r *Redirector) Lookup(clientPort uint16, v6 bool) (*Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.nat[natKey{clientPort, v6}]
	return c, ok
}

func (r *Redirector) Stats() Stats {
	r.mu.RLock()
	active := len(r.nat)
	r.mu.RUnlock()
	return Stats{
		Redirected:  r.stats.redirected.Load(),
		Returned:    r.stats.returned.Load(),
		Redirected6: r.stats.redirected6.Load(),
		DroppedV6:   r.stats.droppedV6.Load(),
		SkippedUDP:  r.stats.skippedUDP.Load(),
		SendErr:     r.stats.sendErr.Load(),
		LastErr:     lastErrStr(&r.stats.lastErr),
		Active:      active,
	}
}

// Start abre as duas escutas e comeca a desviar.
func (r *Redirector) Start() error {
	hs, err := divert.Open("tcp", divert.LayerSocket, 0, divert.FlagSniff|divert.FlagRecvOnly)
	if err != nil {
		return fmt.Errorf("abrindo camada SOCKET: %w", err)
	}
	r.hSock = hs

	filter := fmt.Sprintf(
		"tcp and ("+
			"(outbound and ip and ip.DstAddr != 127.0.0.1) or "+
			"(outbound and ipv6 and ipv6.DstAddr != ::1) or "+
			"(ip and ip.SrcAddr == 127.0.0.1 and tcp.SrcPort == %d) or "+
			"(ipv6 and ipv6.SrcAddr == ::1 and tcp.SrcPort == %d))",
		r.proxyPort, r.proxyPort)
	hn, err := divert.Open(filter, divert.LayerNetwork, 0, 0)
	if err != nil {
		hs.Close()
		return fmt.Errorf("abrindo camada NETWORK: %w", err)
	}
	r.hNet = hn

	r.wg.Add(3)
	go r.runSockets()
	go r.runNetwork()
	go r.sweep()
	return nil
}

func (r *Redirector) Stop() {
	select {
	case <-r.stop:
		return
	default:
		close(r.stop)
	}
	if r.hSock != nil {
		r.hSock.Close()
	}
	if r.hNet != nil {
		r.hNet.Close()
	}
	r.wg.Wait()
}

func (r *Redirector) runSockets() {
	defer r.wg.Done()
	var addr divert.Address
	for {
		select {
		case <-r.stop:
			return
		default:
		}
		if _, err := r.hSock.Recv(nil, &addr); err != nil {
			return
		}
		s := addr.Socket()
		switch addr.Event() {
		case divert.EventSocketConnect, divert.EventSocketBind:
			if r.isTarget(s.ProcessID) {
				r.mu.Lock()
				r.ports[s.LocalPort] = s.ProcessID
				r.mu.Unlock()
			}
		case divert.EventSocketClose:
			r.mu.Lock()
			delete(r.ports, s.LocalPort)
			// Esquece a classificacao do PID que fechou: sem isto o mapa
			// cresce sem limite e um PID reciclado herdaria o veredito antigo.
			if !r.pidHasPort(s.ProcessID) {
				delete(r.pids, s.ProcessID)
			}
			r.mu.Unlock()
		}
	}
}

// pidHasPort diz se ainda ha alguma porta aberta mapeada a este PID. So
// removemos a classificacao quando o processo nao tem mais nenhuma.
func (r *Redirector) pidHasPort(pid uint32) bool {
	for _, p := range r.ports {
		if p == pid {
			return true
		}
	}
	return false
}

func (r *Redirector) isTarget(pid uint32) bool {
	r.mu.RLock()
	v, known := r.pids[pid]
	r.mu.RUnlock()
	if known {
		return v
	}
	name := lower(processName(pid))
	hit := name != "" && r.targets[name]
	r.mu.Lock()
	r.pids[pid] = hit
	r.mu.Unlock()
	return hit
}

func (r *Redirector) runNetwork() {
	defer r.wg.Done()
	packet := make([]byte, maxPacket)
	var addr divert.Address

	for {
		select {
		case <-r.stop:
			return
		default:
		}
		n, err := r.hNet.Recv(packet, &addr)
		if err != nil {
			return
		}
		buf := packet[:n]

		p, ok := parsePacket(buf)
		if !ok {
			r.pass(buf, &addr)
			continue
		}

		switch {
		case p.dstPort() == r.proxyPort && p.srcIsLoopback():
			r.pass(buf, &addr)

		case p.srcPort() == r.proxyPort:
			r.handleReturn(buf, p, &addr)

		default:
			r.handleOutbound(buf, p, &addr)
		}
	}
}

func (r *Redirector) handleOutbound(buf []byte, p pkt, addr *divert.Address) {
	srcPort := p.srcPort()

	r.mu.RLock()
	pid, isOurs := r.ports[srcPort]
	r.mu.RUnlock()
	if !isOurs {
		r.pass(buf, addr)
		return
	}

	if p.v6 && r.ForceIPv4.Load() {
		r.stats.droppedV6.Add(1)
		return
	}

	key := natKey{srcPort, p.v6}
	r.mu.Lock()
	if c, ok := r.nat[key]; ok {
		c.lastSeen = time.Now()
	} else {
		n := addr.Network()
		r.nat[key] = &Conn{
			DstIP:    copyIP(p.dst),
			DstPort:  p.dstPort(),
			SrcIP:    copyIP(p.src),
			PID:      pid,
			ifIdx:    n.IfIdx,
			subIfIdx: n.SubIfIdx,
			lastSeen: time.Now(),
		}
	}
	r.mu.Unlock()

	lo := p.loopbackAddr()
	copy(p.src, lo)
	copy(p.dst, lo)
	p.setDstPort(r.proxyPort)

	n := addr.Network()
	n.IfIdx, n.SubIfIdx = loopbackIfIdx, 0

	r.stats.redirected.Add(1)
	if p.v6 {
		r.stats.redirected6.Add(1)
	}
	r.send(buf, addr)
}

func (r *Redirector) handleReturn(buf []byte, p pkt, addr *divert.Address) {
	r.mu.RLock()
	c, ok := r.nat[natKey{p.dstPort(), p.v6}]
	r.mu.RUnlock()
	if !ok {
		r.pass(buf, addr)
		return
	}

	copy(p.src, c.DstIP)
	copy(p.dst, c.SrcIP)
	p.setSrcPort(c.DstPort)

	n := addr.Network()
	n.IfIdx, n.SubIfIdx = c.ifIdx, c.subIfIdx

	addr.SetOutbound(false)

	r.stats.returned.Add(1)
	r.send(buf, addr)
}

func (r *Redirector) send(buf []byte, addr *divert.Address) {
	if err := r.hNet.CalcChecksums(buf, addr); err != nil {
		r.noteErr("checksum: " + err.Error())
		return
	}
	if _, err := r.hNet.Send(buf, addr); err != nil {
		r.noteErr("send: " + err.Error())
	}
}

func (r *Redirector) noteErr(msg string) {
	r.stats.sendErr.Add(1)
	r.stats.lastErr.Store(msg)
}

func lastErrStr(v *atomic.Value) string {
	if s, ok := v.Load().(string); ok {
		return s
	}
	return ""
}

func (r *Redirector) pass(buf []byte, addr *divert.Address) {
	_, _ = r.hNet.Send(buf, addr)
}

func (r *Redirector) sweep() {
	defer r.wg.Done()
	t := time.NewTicker(sweepGap)
	defer t.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-t.C:
			cut := time.Now().Add(-natTTL)
			r.mu.Lock()
			for k, c := range r.nat {
				if c.lastSeen.Before(cut) {
					delete(r.nat, k)
				}
			}
			r.mu.Unlock()
		}
	}
}
