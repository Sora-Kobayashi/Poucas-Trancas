// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package redirect

import (
	"encoding/binary"
	"net"
)

const (
	protoTCP = 6

	v4HeaderMin = 20
	v6Header    = 40
)

var (
	loopback4 = net.IPv4(127, 0, 0, 1).To4()
	loopback6 = net.IPv6loopback.To16()
)

type pkt struct {
	v6  bool
	src []byte
	dst []byte
	tcp []byte
}

func (p pkt) srcPort() uint16 { return binary.BigEndian.Uint16(p.tcp[0:2]) }
func (p pkt) dstPort() uint16 { return binary.BigEndian.Uint16(p.tcp[2:4]) }

func (p pkt) setSrcPort(v uint16) { binary.BigEndian.PutUint16(p.tcp[0:2], v) }
func (p pkt) setDstPort(v uint16) { binary.BigEndian.PutUint16(p.tcp[2:4], v) }

func (p pkt) loopbackAddr() []byte {
	if p.v6 {
		return loopback6
	}
	return loopback4
}

func (p pkt) srcIsLoopback() bool { return ipEqual(p.src, p.loopbackAddr()) }

func copyIP(b []byte) net.IP { return append(net.IP(nil), b...) }

func ipEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func parsePacket(b []byte) (pkt, bool) {
	if len(b) < 1 {
		return pkt{}, false
	}
	switch b[0] >> 4 {
	case 4:
		return parseV4(b)
	case 6:
		return parseV6(b)
	}
	return pkt{}, false
}

func parseV4(b []byte) (pkt, bool) {
	if len(b) < v4HeaderMin {
		return pkt{}, false
	}
	ihl := int(b[0]&0x0F) * 4
	if ihl < v4HeaderMin || len(b) < ihl+20 || b[9] != protoTCP {
		return pkt{}, false
	}
	return pkt{src: b[12:16], dst: b[16:20], tcp: b[ihl:]}, true
}

func parseV6(b []byte) (pkt, bool) {
	if len(b) < v6Header+20 {
		return pkt{}, false
	}
	if b[6] != protoTCP {
		return pkt{}, false
	}
	return pkt{v6: true, src: b[8:24], dst: b[24:40], tcp: b[v6Header:]}, true
}
