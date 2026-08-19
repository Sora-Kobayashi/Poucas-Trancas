//go:build windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// Package divert liga o WinDivert 2.2 sem cgo.
//
// A DLL e carregada por LazyDLL e as chamadas passam por syscall, entao o
// build continua sendo Go puro — sem gcc, sem toolchain C. Isso importa
// porque o resto do projeto ja compila assim e nao vale perder isso por
// uma unica dependencia.
//
// O que o WinDivert resolve aqui: interceptar o trafego do Discord POR
// PROCESSO, sem injetar DLL nenhuma nele. A camada SOCKET diz qual PID
// abriu cada conexao; a camada NETWORK entrega os pacotes para reescrever.
//
// Precisa de Administrador: o driver e carregado no primeiro Open.
package divert

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Layer uint8

const (
	LayerNetwork        Layer = 0
	LayerNetworkForward Layer = 1
	LayerFlow           Layer = 2
	LayerSocket         Layer = 3
	LayerReflect        Layer = 4
)

type Flags uint64

const (
	FlagSniff     Flags = 0x0001
	FlagDrop      Flags = 0x0002
	FlagRecvOnly  Flags = 0x0004
	FlagSendOnly  Flags = 0x0008
	FlagNoInstall Flags = 0x0010
	FlagFragments Flags = 0x0020
)

type Event uint8

const (
	EventNetworkPacket Event = 0
	EventFlowEstablish Event = 1
	EventFlowDelete    Event = 2
	EventSocketBind    Event = 3
	EventSocketConnect Event = 4
	EventSocketListen  Event = 5
	EventSocketAccept  Event = 6
	EventSocketClose   Event = 7
	EventReflectOpen   Event = 8
	EventReflectClose  Event = 9
)

// Address espelha WINDIVERT_ADDRESS (80 bytes). O layout e fixo pelo
// driver; mudar a ordem ou o tamanho aqui corrompe tudo silenciosamente.
type Address struct {
	Timestamp int64
	bits      uint32
	Reserved2 uint32
	Data      [64]byte
}

func (a *Address) Layer() Layer   { return Layer(a.bits & 0xFF) }
func (a *Address) Event() Event   { return Event((a.bits >> 8) & 0xFF) }
func (a *Address) Outbound() bool { return a.bits&(1<<17) != 0 }
func (a *Address) Loopback() bool { return a.bits&(1<<18) != 0 }
func (a *Address) IPv6() bool     { return a.bits&(1<<20) != 0 }

func (a *Address) SetOutbound(v bool) {
	if v {
		a.bits |= 1 << 17
	} else {
		a.bits &^= 1 << 17
	}
}

// DataNetwork e a visao da camada NETWORK.
type DataNetwork struct {
	IfIdx    uint32
	SubIfIdx uint32
}

// DataSocket e a visao da camada SOCKET — aqui vem o ProcessId, que e o
// motivo de existir este pacote.
type DataSocket struct {
	EndpointID       uint64
	ParentEndpointID uint64
	ProcessID        uint32
	LocalAddr        [4]uint32
	RemoteAddr       [4]uint32
	LocalPort        uint16
	RemotePort       uint16
	Protocol         uint8
}

func (a *Address) Network() *DataNetwork {
	return (*DataNetwork)(unsafe.Pointer(&a.Data[0]))
}

func (a *Address) Socket() *DataSocket {
	return (*DataSocket)(unsafe.Pointer(&a.Data[0]))
}

var (
	dll     *windows.LazyDLL
	pOpen   *windows.LazyProc
	pRecv   *windows.LazyProc
	pSend   *windows.LazyProc
	pClose  *windows.LazyProc
	pCalc   *windows.LazyProc
	pShut   *windows.LazyProc
	loadErr error
)

// Load aponta para a WinDivert.dll extraida. Precisa ser chamado antes de
// Open; o .sys tem de estar na MESMA pasta, senao o driver nao sobe.
func Load(dllPath string) error {
	dll = windows.NewLazyDLL(dllPath)
	if err := dll.Load(); err != nil {
		loadErr = fmt.Errorf("carregando %s: %w", dllPath, err)
		return loadErr
	}
	pOpen = dll.NewProc("WinDivertOpen")
	pRecv = dll.NewProc("WinDivertRecv")
	pSend = dll.NewProc("WinDivertSend")
	pClose = dll.NewProc("WinDivertClose")
	pCalc = dll.NewProc("WinDivertHelperCalcChecksums")
	pShut = dll.NewProc("WinDivertShutdown")
	loadErr = nil
	return nil
}

type Handle struct {
	h windows.Handle
}

// Open abre um canal com o filtro dado. A sintaxe do filtro e a do
// WinDivert, ex: "outbound and tcp and ip.DstAddr != 127.0.0.1".
func Open(filter string, layer Layer, priority int16, flags Flags) (*Handle, error) {
	if dll == nil {
		return nil, fmt.Errorf("divert.Load nao foi chamado")
	}
	if loadErr != nil {
		return nil, loadErr
	}
	f, err := syscall.BytePtrFromString(filter)
	if err != nil {
		return nil, err
	}
	r, _, e := pOpen.Call(
		uintptr(unsafe.Pointer(f)),
		uintptr(layer),
		uintptr(priority),
		uintptr(flags),
	)
	h := windows.Handle(r)
	if h == windows.InvalidHandle {
		if e == windows.ERROR_ACCESS_DENIED {
			return nil, fmt.Errorf("acesso negado — o WinDivert precisa de Administrador")
		}
		return nil, fmt.Errorf("WinDivertOpen falhou: %w", e)
	}
	return &Handle{h: h}, nil
}

// Recv bloqueia ate chegar um pacote. Devolve quantos bytes vieram.
func (h *Handle) Recv(packet []byte, addr *Address) (int, error) {
	var n uint32
	var p unsafe.Pointer
	if len(packet) > 0 {
		p = unsafe.Pointer(&packet[0])
	}
	r, _, e := pRecv.Call(
		uintptr(h.h),
		uintptr(p),
		uintptr(len(packet)),
		uintptr(unsafe.Pointer(&n)),
		uintptr(unsafe.Pointer(addr)),
	)
	if r == 0 {
		return 0, fmt.Errorf("WinDivertRecv: %w", e)
	}
	return int(n), nil
}

// Send devolve o pacote (possivelmente reescrito) para a pilha de rede.
// Sem isso o pacote e descartado — o WinDivert desvia, nao copia.
func (h *Handle) Send(packet []byte, addr *Address) (int, error) {
	var n uint32
	var p unsafe.Pointer
	if len(packet) > 0 {
		p = unsafe.Pointer(&packet[0])
	}
	r, _, e := pSend.Call(
		uintptr(h.h),
		uintptr(p),
		uintptr(len(packet)),
		uintptr(unsafe.Pointer(&n)),
		uintptr(unsafe.Pointer(addr)),
	)
	if r == 0 {
		return 0, fmt.Errorf("WinDivertSend: %w", e)
	}
	return int(n), nil
}

func (h *Handle) CalcChecksums(packet []byte, addr *Address) error {
	var p unsafe.Pointer
	if len(packet) > 0 {
		p = unsafe.Pointer(&packet[0])
	}
	r, _, e := pCalc.Call(
		uintptr(p),
		uintptr(len(packet)),
		uintptr(unsafe.Pointer(addr)),
		0,
	)
	if r == 0 {
		return fmt.Errorf("WinDivertHelperCalcChecksums: %w", e)
	}
	return nil
}

// Shutdown destrava um Recv pendente para o Close nao ficar preso.
func (h *Handle) Shutdown() {
	if pShut != nil {
		pShut.Call(uintptr(h.h), 2 /* WINDIVERT_SHUTDOWN_BOTH */)
	}
}

func (h *Handle) Close() error {
	h.Shutdown()
	r, _, e := pClose.Call(uintptr(h.h))
	if r == 0 {
		return fmt.Errorf("WinDivertClose: %w", e)
	}
	return nil
}
