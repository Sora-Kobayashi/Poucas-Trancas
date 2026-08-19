//go:build !windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package divert

import "errors"

type (
	Layer uint8
	Flags uint64
	Event uint8
)

const (
	LayerNetwork Layer = 0
	LayerSocket  Layer = 3

	FlagSniff    Flags = 0x0001
	FlagRecvOnly Flags = 0x0004

	EventSocketBind    Event = 3
	EventSocketConnect Event = 4
	EventSocketClose   Event = 7
)

type DataNetwork struct{ IfIdx, SubIfIdx uint32 }

type DataSocket struct {
	ProcessID uint32
	LocalPort uint16
	Protocol  uint8
}

type Address struct{ data [80]byte }

func (a *Address) Layer() Layer          { return 0 }
func (a *Address) Event() Event          { return 0 }
func (a *Address) Outbound() bool        { return false }
func (a *Address) IPv6() bool            { return false }
func (a *Address) SetOutbound(bool)      {}
func (a *Address) Network() *DataNetwork { return &DataNetwork{} }
func (a *Address) Socket() *DataSocket   { return &DataSocket{} }

// ErrUnsupported explica por que o desvio nao existe fora do Windows: o
// WinDivert e um driver do kernel do Windows. O equivalente em Linux seria
// nftables com TPROXY ou uma interface TUN, que e outro backend inteiro.
var ErrUnsupported = errors.New("interceptação por PID só existe no Windows (WinDivert); em Linux falta um backend nftables/TPROXY")

type Handle struct{}

func Load(string) error                                 { return ErrUnsupported }
func Open(string, Layer, int16, Flags) (*Handle, error) { return nil, ErrUnsupported }

func (h *Handle) Recv([]byte, *Address) (int, error)   { return 0, ErrUnsupported }
func (h *Handle) Send([]byte, *Address) (int, error)   { return 0, ErrUnsupported }
func (h *Handle) CalcChecksums([]byte, *Address) error { return ErrUnsupported }
func (h *Handle) Shutdown()                            {}
func (h *Handle) Close() error                         { return nil }
