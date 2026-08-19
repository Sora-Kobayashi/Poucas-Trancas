// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package redirect

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

func mkV4(src, dst net.IP, sport, dport uint16) []byte {
	b := make([]byte, 40)
	b[0] = 0x45
	b[9] = protoTCP
	copy(b[12:16], src.To4())
	copy(b[16:20], dst.To4())
	binary.BigEndian.PutUint16(b[20:22], sport)
	binary.BigEndian.PutUint16(b[22:24], dport)
	return b
}

func mkV6(src, dst net.IP, sport, dport uint16) []byte {
	b := make([]byte, v6Header+20)
	b[0] = 0x60
	b[6] = protoTCP
	copy(b[8:24], src.To16())
	copy(b[24:40], dst.To16())
	binary.BigEndian.PutUint16(b[40:42], sport)
	binary.BigEndian.PutUint16(b[42:44], dport)
	return b
}

func TestParsePacket(t *testing.T) {
	p4, ok := parsePacket(mkV4(net.IPv4(192, 168, 1, 15), net.IPv4(162, 159, 128, 233), 51000, 443))
	if !ok || p4.v6 || p4.dstPort() != 443 || p4.src[0] != 192 {
		t.Fatalf("IPv4 mal interpretado: %+v ok=%v", p4, ok)
	}

	src6 := net.ParseIP("2804:104c:8429:cc00::1")
	dst6 := net.ParseIP("2606:4700:7::a29f:8aea")
	p6, ok := parsePacket(mkV6(src6, dst6, 51001, 443))
	if !ok || !p6.v6 || p6.dstPort() != 443 {
		t.Fatalf("IPv6 mal interpretado: %+v ok=%v", p6, ok)
	}
	if !net.IP(p6.dst).Equal(dst6) {
		t.Errorf("destino v6 errado: %v", net.IP(p6.dst))
	}

	udp := mkV4(net.IPv4(1, 2, 3, 4), net.IPv4(5, 6, 7, 8), 1, 2)
	udp[9] = 17
	if _, ok := parsePacket(udp); ok {
		t.Error("UDP nao deveria ser reconhecido")
	}
	if _, ok := parsePacket(mkV6(src6, dst6, 1, 2)[:30]); ok {
		t.Error("IPv6 truncado nao deveria passar")
	}
	if _, ok := parsePacket([]byte{0x00}); ok {
		t.Error("lixo nao deveria passar")
	}
}

// A ida grava o destino; a volta tem de restaura-lo byte a byte. Roda para
// as duas familias porque o bug de misturar v4 com v6 e silencioso.
func TestNATIdaEVolta(t *testing.T) {
	const proxy, clientPort = 9253, 51000

	cases := []struct {
		name           string
		v6             bool
		src, dst, loop net.IP
		mk             func(src, dst net.IP, sp, dp uint16) []byte
	}{
		{"IPv4", false, net.IPv4(192, 168, 1, 15), net.IPv4(162, 159, 128, 233),
			net.IPv4(127, 0, 0, 1), mkV4},
		{"IPv6", true, net.ParseIP("2804:104c:8429:cc00::1"),
			net.ParseIP("2606:4700:7::a29f:8aea"), net.IPv6loopback, mkV6},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := New(proxy, []string{"discord.exe"})
			r.ports[clientPort] = 4242

			out := tc.mk(tc.src, tc.dst, clientPort, 443)
			p, _ := parsePacket(out)

			key := natKey{clientPort, p.v6}
			r.nat[key] = &Conn{DstIP: copyIP(p.dst), DstPort: 443, SrcIP: copyIP(p.src), PID: 4242}
			copy(p.src, p.loopbackAddr())
			copy(p.dst, p.loopbackAddr())
			p.setDstPort(proxy)

			if !net.IP(p.dst).Equal(tc.loop) || p.dstPort() != proxy {
				t.Fatalf("ida nao virou loopback: %v:%d", net.IP(p.dst), p.dstPort())
			}

			c, ok := r.Lookup(clientPort, tc.v6)
			if !ok || !c.DstIP.Equal(tc.dst) {
				t.Fatalf("tabela nao guardou o destino: %+v", c)
			}
			if _, wrong := r.Lookup(clientPort, !tc.v6); wrong {
				t.Error("familia trocada devolveu entrada — chave nao esta separando v4/v6")
			}

			back := tc.mk(tc.loop, tc.loop, proxy, clientPort)
			bp, _ := parsePacket(back)
			copy(bp.src, c.DstIP)
			copy(bp.dst, c.SrcIP)
			bp.setSrcPort(c.DstPort)

			if !net.IP(bp.src).Equal(tc.dst) {
				t.Errorf("origem nao restaurada: %v", net.IP(bp.src))
			}
			if !net.IP(bp.dst).Equal(tc.src) {
				t.Errorf("destino nao restaurado: %v", net.IP(bp.dst))
			}
			if bp.srcPort() != 443 {
				t.Errorf("porta nao restaurada: %d", bp.srcPort())
			}
		})
	}
}

func TestProxyRecusaSemNAT(t *testing.T) {
	r := New(0, []string{"discord.exe"})
	dialed := make(chan string, 1)
	p := NewProxy(0, r, func(network, addr string) (net.Conn, error) {
		dialed <- addr
		return nil, errNoDial
	}, nil)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p.serveOn(ln)
	defer p.Stop()

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.Close()

	select {
	case addr := <-dialed:
		t.Fatalf("nao deveria ter discado nada, discou %s", addr)
	case <-time.After(300 * time.Millisecond):
	}
	if p.Failed.Load() == 0 {
		t.Error("deveria ter contado a recusa")
	}
}

var errNoDial = errors.New("nao deveria discar")
