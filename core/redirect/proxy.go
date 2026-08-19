// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package redirect

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// DialFunc e o transporte de saida. Assinatura igual a net.Dial para
// aceitar tanto o SOCKS do Tor quanto um discador direto.
type DialFunc func(network, addr string) (net.Conn, error)

type Proxy struct {
	port  uint16
	red   *Redirector
	dial  DialFunc
	onLog func(string)

	FallbackDirect atomic.Bool

	lns    []net.Listener
	closed atomic.Bool
	wg     sync.WaitGroup

	Served atomic.Uint64
	Failed atomic.Uint64
	Direct atomic.Uint64
}

func NewProxy(port uint16, red *Redirector, dial DialFunc, onLog func(string)) *Proxy {
	return &Proxy{port: port, red: red, dial: dial, onLog: onLog}
}

func (p *Proxy) logf(format string, a ...any) {
	if p.onLog != nil {
		p.onLog(fmt.Sprintf(format, a...))
	}
}

func (p *Proxy) Start() error {
	for _, addr := range []string{
		fmt.Sprintf("127.0.0.1:%d", p.port),
		fmt.Sprintf("[::1]:%d", p.port),
	} {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			if len(p.lns) == 0 {
				return fmt.Errorf("escutando em %s: %w", addr, err)
			}
			p.logf("sem escuta em %s: %v", addr, err)
			continue
		}
		p.serveOn(ln)
	}
	return nil
}

func (p *Proxy) serveOn(ln net.Listener) {
	p.lns = append(p.lns, ln)
	p.wg.Add(1)
	go p.accept(ln)
}

func (p *Proxy) Stop() {
	if p.closed.Swap(true) {
		return
	}
	for _, ln := range p.lns {
		ln.Close()
	}
	p.wg.Wait()
}

func (p *Proxy) accept(ln net.Listener) {
	defer p.wg.Done()
	for {
		c, err := ln.Accept()
		if err != nil {
			if p.closed.Load() {
				return
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return
		}
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.serve(c)
		}()
	}
}

func (p *Proxy) serve(client net.Conn) {
	defer client.Close()

	ap, ok := client.RemoteAddr().(*net.TCPAddr)
	if !ok {
		p.Failed.Add(1)
		return
	}
	v6 := ap.IP.To4() == nil
	conn, found := p.red.Lookup(uint16(ap.Port), v6)
	if !found {
		p.Failed.Add(1)
		p.logf("conexao de :%d sem destino conhecido — recusada", ap.Port)
		return
	}

	target := net.JoinHostPort(conn.DstIP.String(), fmt.Sprint(conn.DstPort))
	upstream, err := p.dial("tcp", target)
	if err != nil {
		if !p.FallbackDirect.Load() {
			p.Failed.Add(1)
			p.logf("falha discando %s: %v", target, err)
			return
		}
		direct, derr := net.DialTimeout("tcp", target, 15*time.Second)
		if derr != nil {
			p.Failed.Add(1)
			p.logf("falha discando %s (tor e direto): %v / %v", target, err, derr)
			return
		}
		p.Direct.Add(1)
		p.logf("FORA DO TOR (IP real exposto): %s", target)
		defer direct.Close()
		pipe(client, direct)
		return
	}
	defer upstream.Close()

	p.Served.Add(1)
	p.logf("%s (pid %d)", target, conn.PID)
	pipe(client, upstream)
}

func pipe(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	half := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.SetReadDeadline(time.Now().Add(5 * time.Second))
		}
	}
	go half(a, b)
	go half(b, a)
	wg.Wait()
}
