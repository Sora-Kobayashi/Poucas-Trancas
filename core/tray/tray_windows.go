//go:build windows

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// Package tray coloca um icone na area de notificacao do Windows e envia
// balloons, usando Win32 direto — sem cgo e sem dependencia externa.
package tray

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmCommand     = 0x0111
	wmTrayIcon    = 0x0400 + 1
	wmLButtonUp   = 0x0202
	wmRButtonUp   = 0x0205
	wmLButtonDbl  = 0x0203
	wmNullMessage = 0x0000

	nimAdd    = 0x0000
	nimModify = 0x0001
	nimDelete = 0x0002

	nifMessage = 0x0001
	nifIcon    = 0x0002
	nifTip     = 0x0004
	nifInfo    = 0x0010

	niifNone    = 0x0000
	niifInfo    = 0x0001
	niifWarning = 0x0002
	niifError   = 0x0003

	mfString    = 0x0000
	mfSeparator = 0x0800
	mfGrayed    = 0x0001

	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020

	imageIcon      = 1
	lrLoadFromFile = 0x0010
	lrDefaultSize  = 0x0040

	idiApplication = 32512
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	pRegisterClassEx  = user32.NewProc("RegisterClassExW")
	pCreateWindowEx   = user32.NewProc("CreateWindowExW")
	pDefWindowProc    = user32.NewProc("DefWindowProcW")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pGetMessage       = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessage  = user32.NewProc("DispatchMessageW")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	pDestroyMenu      = user32.NewProc("DestroyMenu")
	pAppendMenu       = user32.NewProc("AppendMenuW")
	pTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	pSetForegroundWin = user32.NewProc("SetForegroundWindow")
	pGetCursorPos     = user32.NewProc("GetCursorPos")
	pLoadIcon         = user32.NewProc("LoadIconW")
	pLoadImage        = user32.NewProc("LoadImageW")
	pPostMessage      = user32.NewProc("PostMessageW")

	pDestroyIcon     = user32.NewProc("DestroyIcon")
	pShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")
	pGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type point struct{ X, Y int32 }

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type notifyIconData struct {
	size            uint32
	hwnd            windows.Handle
	id              uint32
	flags           uint32
	callbackMessage uint32
	icon            windows.Handle
	tip             [128]uint16
	state           uint32
	stateMask       uint32
	info            [256]uint16
	timeoutVersion  uint32
	infoTitle       [64]uint16
	infoFlags       uint32
	guidItem        windows.GUID
	balloonIcon     windows.Handle
}

// Level classifica o balloon.
type Level int

const (
	Info Level = iota
	Warning
	Error
)

// onPanic recebe falhas da thread da bandeja, que de outro modo morreriam
// caladas por rodar fora do fluxo principal.
var onPanic func(string)

// SetDiagnostic instala o coletor de falhas da bandeja.
func SetDiagnostic(f func(string)) { onPanic = f }

// Item e uma entrada do menu de contexto.
type Item struct {
	Label    string
	Disabled bool
	Sep      bool
	OnClick  func()
}

// Tray controla o icone. Todos os metodos podem ser chamados de qualquer
// goroutine; a fila do Windows fica isolada na thread dona da janela.
type Tray struct {
	mu       sync.Mutex
	hwnd     windows.Handle
	icon     windows.Handle
	items    []Item
	tip      string
	onClick  func()
	ready    chan struct{}
	quitOnce sync.Once
}

func utf16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func copyUTF16(dst []uint16, s string) {
	src, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	if len(src) > len(dst) {
		src = src[:len(dst)]
		src[len(src)-1] = 0
	}
	copy(dst, src)
}

// New cria o icone e roda a fila de mensagens ate Stop.
// iconPath vazio usa o icone padrao de aplicacao.
func New(tip, iconPath string, onClick func(), items []Item) (*Tray, error) {
	t := &Tray{
		tip:     tip,
		onClick: onClick,
		items:   items,
		ready:   make(chan struct{}),
	}
	errCh := make(chan error, 1)
	go t.run(iconPath, errCh)

	select {
	case err := <-errCh:
		return nil, err
	case <-t.ready:
		return t, nil
	}
}

func (t *Tray) run(iconPath string, errCh chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		if r := recover(); r != nil && onPanic != nil {
			onPanic(fmt.Sprintf("panico na bandeja: %v", r))
		}
	}()

	inst, _, _ := pGetModuleHandle.Call(0)
	className := utf16("PoucasTrancasTray")

	wc := wndClassEx{
		size:      uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:   syscall.NewCallback(t.wndProc),
		instance:  windows.Handle(inst),
		className: className,
	}
	if r, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		errCh <- fmt.Errorf("RegisterClassEx: %w", err)
		return
	}

	hwnd, _, err := pCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16(t.tip))),
		0, 0, 0, 0, 0, 0, 0, inst, 0,
	)
	if hwnd == 0 {
		errCh <- fmt.Errorf("CreateWindowEx: %w", err)
		return
	}
	t.hwnd = windows.Handle(hwnd)
	t.icon = loadIcon(iconPath, windows.Handle(inst))

	if err := t.notify(nimAdd, nil); err != nil {
		errCh <- err
		return
	}
	close(t.ready)

	var m msg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			if onPanic != nil {
				onPanic(fmt.Sprintf("fila da bandeja encerrou (GetMessage=%d)", int32(r)))
			}
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func loadIcon(path string, inst windows.Handle) windows.Handle {
	if path != "" {
		h, _, _ := pLoadImage.Call(0, uintptr(unsafe.Pointer(utf16(path))),
			imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
		if h != 0 {
			return windows.Handle(h)
		}
	}
	h, _, _ := pLoadIcon.Call(0, idiApplication)
	return windows.Handle(h)
}

func (t *Tray) baseData() notifyIconData {
	d := notifyIconData{
		size:            uint32(unsafe.Sizeof(notifyIconData{})),
		hwnd:            t.hwnd,
		id:              1,
		flags:           nifMessage | nifIcon | nifTip,
		callbackMessage: wmTrayIcon,
		icon:            t.icon,
	}
	t.mu.Lock()
	copyUTF16(d.tip[:], t.tip)
	t.mu.Unlock()
	return d
}

func (t *Tray) notify(action uintptr, d *notifyIconData) error {
	if d == nil {
		base := t.baseData()
		d = &base
	}
	if r, _, err := pShellNotifyIcon.Call(action, uintptr(unsafe.Pointer(d))); r == 0 {
		return fmt.Errorf("Shell_NotifyIcon: %w", err)
	}
	return nil
}

// SetTooltip troca o texto exibido ao passar o mouse.
func (t *Tray) SetTooltip(s string) {
	t.mu.Lock()
	t.tip = s
	t.mu.Unlock()
	_ = t.notify(nimModify, nil)
}

// SetItems substitui o menu de contexto.
func (t *Tray) SetItems(items []Item) {
	t.mu.Lock()
	t.items = items
	t.mu.Unlock()
}

// Notify exibe um balloon.
func (t *Tray) Notify(title, body string, level Level) {
	d := t.baseData()
	d.flags |= nifInfo
	copyUTF16(d.infoTitle[:], title)
	copyUTF16(d.info[:], body)
	switch level {
	case Warning:
		d.infoFlags = niifWarning
	case Error:
		d.infoFlags = niifError
	default:
		d.infoFlags = niifInfo
	}
	_ = t.notify(nimModify, &d)
}

// Stop remove o icone e encerra a fila.
func (t *Tray) Stop() {
	t.quitOnce.Do(func() {
		_ = t.notify(nimDelete, nil)
		pPostMessage.Call(uintptr(t.hwnd), wmClose, 0, 0)
		if t.icon != 0 {
			pDestroyIcon.Call(uintptr(t.icon))
		}
	})
}

// wndProc precisa de TODOS os parametros do tamanho de uintptr: o
// syscall.NewCallback monta a chamada assumindo isso, e um uint32 no meio
// desloca a leitura dos argumentos seguintes. O sintoma foi a fila receber
// WM_QUIT sozinha e a bandeja morrer em segundos.
func (t *Tray) wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmTrayIcon:
		switch lParam {
		case wmLButtonUp, wmLButtonDbl:
			if t.onClick != nil {
				go t.onClick()
			}
		case wmRButtonUp:
			t.showMenu()
		}
		return 0

	case wmCommand:
		idx := int(wParam&0xFFFF) - 1
		t.mu.Lock()
		var fn func()
		if idx >= 0 && idx < len(t.items) {
			fn = t.items[idx].OnClick
		}
		t.mu.Unlock()
		if fn != nil {
			go fn()
		}
		return 0

	case wmClose:
		pDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(hwnd, message, wParam, lParam)
	return r
}

func (t *Tray) showMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer pDestroyMenu.Call(menu)

	t.mu.Lock()
	items := append([]Item(nil), t.items...)
	t.mu.Unlock()

	for i, it := range items {
		if it.Sep {
			pAppendMenu.Call(menu, mfSeparator, 0, 0)
			continue
		}
		flags := uintptr(mfString)
		if it.Disabled {
			flags |= mfGrayed
		}
		pAppendMenu.Call(menu, flags, uintptr(i+1), uintptr(unsafe.Pointer(utf16(it.Label))))
	}

	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWin.Call(uintptr(t.hwnd))
	pTrackPopupMenu.Call(menu, tpmRightButton|tpmBottomAlign,
		uintptr(pt.X), uintptr(pt.Y), 0, uintptr(t.hwnd), 0)
	pPostMessage.Call(uintptr(t.hwnd), wmNullMessage, 0, 0)
}
