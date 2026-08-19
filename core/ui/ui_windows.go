// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// Package ui e a janela do Poucas Trancas, desenhada em Win32 puro.
//
// Nao ha framework nem WebView: a interface e pintada em GDI sobre um
// buffer fora da tela e a interacao sai de hit-test em retangulos. Em troca
// do trabalho de desenhar na mao, o executavel perde ~11 MB e o programa
// deixa de exigir WebView2 na maquina de quem usa.
package ui

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"poucastrancas/core"
)

// Callbacks liga a janela ao resto do programa. A UI nao conhece o core
// alem disto.
type Callbacks struct {
	Status      func() core.Status
	Config      func() core.Config
	Start       func(mode string) string
	Stop        func() string
	NewIdentity func() string
	RefreshExit func()
	SetUpstream func(kind, url string)
	SetFallback func(bool)
	SetBridges  func(string) int
	SetPrefs    func(ask, auto, notify bool)
	Restart     func(dir string) string
	OnClose     func() bool // true = esconder em vez de sair
	Log         func(string)
}

const (
	idTimer   = 1
	idSocks   = 100
	idBridges = 101

	winW, winH = 900, 720
	minW, minH = 760, 600
)

type clickable struct {
	r  rect
	fn func()
}

// Window guarda o estado da interface. O estado do tunel vive no core; aqui
// so fica o que e visual.
type Window struct {
	cb   Callbacks
	hwnd windows.Handle

	mu     sync.Mutex
	st     core.Status
	cfg    core.Config
	flash  string
	flashK int // 0 info, 1 ok, 2 erro

	mode string
	up   string

	sc       float64
	hitZones []clickable
	hover    int
	pressed  int

	socksEdit   windows.Handle
	bridgesEdit windows.Handle
	editFont    windows.Handle
	editBrush   windows.Handle
	iconBig     windows.Handle
	iconSmall   windows.Handle
	fonts       *fontCache

	// Ultima posicao aplicada em cada EDIT. SetWindowPos redesenha o
	// controle mesmo quando nada muda, e como a repintura acontece a cada
	// movimento do mouse, isso fazia os campos piscarem sem parar.
	socksRect   rect
	bridgesRect rect
	socksShown  bool

	quit chan struct{}
}

func New(cb Callbacks) *Window {
	return &Window{cb: cb, hover: -1, pressed: -1, fonts: newFontCache(), quit: make(chan struct{})}
}

// Run cria a janela e roda a fila de mensagens ate o fim. Bloqueia.
func (w *Window) Run(iconPath string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w.cfg = w.cb.Config()
	w.mode = w.cfg.UDPMode
	w.up = w.cfg.Upstream
	if w.mode == "" {
		w.mode = string(core.UDPDirect)
	}
	if w.up == "" {
		w.up = string(core.UpTor)
	}

	inst, _, _ := pGetModuleHandle.Call(0)
	cls := utf16ptr("PoucasTrancasMain")
	cursor, _, _ := pLoadCursor.Call(0, idcArrow)

	var iconBigH, iconSmallH windows.Handle
	if iconPath != "" {
		p := uintptr(unsafe.Pointer(utf16ptr(iconPath)))
		lr := uintptr(0x0010) // LR_LOADFROMFILE
		big, _, _ := pLoadImage.Call(0, p, 1, 32, 32, lr)
		sm, _, _ := pLoadImage.Call(0, p, 1, 16, 16, lr)
		iconBigH, iconSmallH = windows.Handle(big), windows.Handle(sm)
	}
	w.iconBig, w.iconSmall = iconBigH, iconSmallH

	// Pincel de fundo preto: com WM_ERASEBKGND devolvendo 1, qualquer area
	// ainda nao pintada (redimensionar, primeiro frame) mostraria lixo da
	// tela por baixo.
	blackBrush, _, _ := pCreateSolidBrush.Call(uintptr(colBG))

	wc := wndClassEx{
		size:       uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:    syscall.NewCallback(w.wndProc),
		instance:   windows.Handle(inst),
		cursor:     windows.Handle(cursor),
		background: windows.Handle(blackBrush),
		className:  cls,
		icon:       iconBigH,
		iconSm:     iconSmallH,
	}
	if r, _, err := pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}

	dpi, _, _ := pGetDpiForSystem.Call()
	scale := 1.0
	if dpi >= 48 {
		scale = float64(dpi) / 96.0
	}
	cw, ch := int32(float64(winW)*scale), int32(float64(winH)*scale)

	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	x := (int32(sw) - cw) / 2
	y := (int32(sh) - ch) / 2

	hwnd, _, err := pCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(utf16ptr("Poucas Trancas"))),
		wsOverlappedWindow,
		uintptr(x), uintptr(y), uintptr(cw), uintptr(ch),
		0, 0, inst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	w.hwnd = windows.Handle(hwnd)

	// A barra de tarefas usa o icone GRANDE da janela; a legenda usa o
	// pequeno. Sem aplicar os dois por WM_SETICON, o Windows mostra um
	// placeholder generico no lugar.
	if iconBigH != 0 {
		pSendMessage.Call(hwnd, wmSetIcon, iconBig, uintptr(iconBigH))
	}
	if iconSmallH != 0 {
		pSendMessage.Call(hwnd, wmSetIcon, iconSmall, uintptr(iconSmallH))
	}

	w.createEdits(windows.Handle(inst))
	pSetTimer.Call(hwnd, idTimer, 1000, 0)
	pShowWindow.Call(hwnd, swShow)
	pUpdateWindow.Call(hwnd)

	var m msg
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	return nil
}

// Edits nativos para entrada de texto: reimplementar cursor, selecao e
// area de transferencia na mao nao valeria o esforco.
func (w *Window) createEdits(inst windows.Handle) {
	lf := logFont{Height: -int32(12*w.dpiScale() + 0.5), Weight: 400, CharSet: 1, Quality: 5}
	src := windows.StringToUTF16("Consolas")
	copy(lf.FaceName[:], src[:min(len(src), 31)])
	f, _, _ := pCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	w.editFont = windows.Handle(f)

	mk := func(id uintptr, style uintptr, visible bool) windows.Handle {
		base := uintptr(wsChild | wsBorder | wsTabstop)
		if visible {
			base |= wsVisible
		}
		h, _, _ := pCreateWindowEx.Call(0,
			uintptr(unsafe.Pointer(utf16ptr("EDIT"))),
			uintptr(unsafe.Pointer(utf16ptr(""))),
			base|style,
			0, 0, 10, 10, uintptr(w.hwnd), id, uintptr(inst), 0)
		pSendMessage.Call(h, wmSetFont, uintptr(w.editFont), 1)
		return windows.Handle(h)
	}
	// O campo do SOCKS nasce oculto para bater com socksShown=false. Criado
	// visivel, a comparacao "mudou?" nunca divergia e ele ficava na tela
	// mesmo com o Tor selecionado.
	w.socksEdit = mk(idSocks, esAutoHScroll, false)
	w.bridgesEdit = mk(idBridges, esMultiline|esAutoVScroll|wsVScroll, true)

	if w.cfg.SocksURL != "" {
		pSetWindowText.Call(uintptr(w.socksEdit), uintptr(unsafe.Pointer(utf16ptr(w.cfg.SocksURL))))
	}
	if len(w.cfg.Bridges) > 0 {
		txt := strings.Join(w.cfg.Bridges, "\r\n")
		pSetWindowText.Call(uintptr(w.bridgesEdit), uintptr(unsafe.Pointer(utf16ptr(txt))))
	}
}

// dpiScale converte medidas de layout (pensadas em 96 dpi) para a tela
// atual. O manifesto declara per-monitor, entao o escalonamento e nosso.
func (w *Window) dpiScale() float64 {
	if w.hwnd == 0 {
		return 1
	}
	d, _, _ := pGetDpiForWindow.Call(uintptr(w.hwnd))
	if d < 48 {
		return 1
	}
	return float64(d) / 96.0
}

// invalidateZone marca so a area de um elemento como suja.
func (w *Window) invalidateZone(i int) {
	if i < 0 || i >= len(w.hitZones) || w.hwnd == 0 {
		return
	}
	r := w.hitZones[i].r
	r.Left -= 2
	r.Top -= 2
	r.Right += 2
	r.Bottom += 2
	pInvalidateRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&r)), 0)
}

// canvasScale leva um retangulo do layout para pixel de tela.
func (w *Window) canvasScale(r rect) rect {
	s := w.sc
	if s < 1 {
		s = 1
	}
	f := func(v int32) int32 { return int32(float64(v)*s + 0.5) }
	return rect{f(r.Left), f(r.Top), f(r.Right), f(r.Bottom)}
}

// placeEdit move um controle nativo so quando a posicao mudou.
func placeEdit(h windows.Handle, last *rect, r rect) {
	if *last == r {
		return
	}
	*last = r
	pSetWindowPos.Call(uintptr(h), 0,
		uintptr(r.Left), uintptr(r.Top), uintptr(r.w()), uintptr(r.h()), 0x0004)
}

func (w *Window) invalidate() {
	if w.hwnd != 0 {
		pInvalidateRect.Call(uintptr(w.hwnd), 0, 0)
	}
}

// Show traz a janela de volta (usado pela bandeja).
func (w *Window) Show() {
	pShowWindow.Call(uintptr(w.hwnd), swRestore)
	pSetForegroundWin.Call(uintptr(w.hwnd))
}

func (w *Window) Hide() { pShowWindow.Call(uintptr(w.hwnd), swHide) }

func (w *Window) Visible() bool {
	r, _, _ := pIsWindowVisible.Call(uintptr(w.hwnd))
	return r != 0
}

func (w *Window) Close() { pSendMessage.Call(uintptr(w.hwnd), wmClose, 0, 0) }

// Flash mostra uma mensagem temporaria no rodape.
func (w *Window) Flash(text string, kind int) {
	w.mu.Lock()
	w.flash, w.flashK = text, kind
	w.mu.Unlock()
	w.invalidate()
}

func (w *Window) wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmPaint:
		w.onPaint()
		return 0

	case wmEraseBkgnd:
		return 1 // o buffer proprio ja pinta o fundo; deixar o Windows apagar pisca

	case wmTimer:
		w.refresh()
		return 0

	case wmSize:
		w.invalidate()
		return 0

	case wmGetMinMaxInfo:
		// lParam e um ponteiro real que o Windows nos passa. O go vet marca
		// uintptr->unsafe.Pointer, mas neste caso e correto: a MINMAXINFO
		// vive na pilha do chamador enquanto tratamos a mensagem.
		mmi := (*minMaxInfo)(unsafe.Pointer(lParam)) //nolint:govet
		sc := w.dpiScale()
		mmi.ptMinTrackSize = point{int32(float64(minW) * sc), int32(float64(minH) * sc)}
		return 0

	case wmMouseMove:
		w.onMouseMove(loWord(lParam), hiWord(lParam))
		return 0

	case wmMouseLeave:
		if w.hover != -1 {
			w.hover = -1
			w.invalidate()
		}
		return 0

	case wmLButtonDown:
		w.pressed = w.hitAt(loWord(lParam), hiWord(lParam))
		w.invalidate()
		return 0

	case wmLButtonUp:
		idx := w.hitAt(loWord(lParam), hiWord(lParam))
		if idx >= 0 && idx == w.pressed && idx < len(w.hitZones) {
			fn := w.hitZones[idx].fn
			w.pressed = -1
			w.invalidate()
			if fn != nil {
				go fn()
			}
			return 0
		}
		w.pressed = -1
		w.invalidate()
		return 0

	case wmCtlColorEdit:
		// O brush do fundo do EDIT e criado uma vez e reusado. Criar um por
		// mensagem vazava ~1 handle/segundo (o EDIT se repinta a cada tick e
		// a cada movimento do mouse) — o classico "trava o Windows".
		if w.editBrush == 0 {
			h, _, _ := pCreateSolidBrush.Call(uintptr(rgb(5, 5, 6)))
			w.editBrush = windows.Handle(h)
		}
		pSetTextColor.Call(wParam, uintptr(colText))
		pSetBkMode.Call(wParam, transparentBk)
		return uintptr(w.editBrush)

	case wmCommand:
		// EN_KILLFOCUS: grava o que foi digitado ao sair do campo
		if hiWord(wParam) == 0x0200 {
			w.commitEdits()
		}
		return 0

	case wmClose:
		if w.cb.OnClose != nil && w.cb.OnClose() {
			w.Hide()
			return 0
		}
		pDestroyWindow.Call(hwnd)
		return 0

	case wmDestroy:
		pKillTimer.Call(hwnd, idTimer)
		w.fonts.destroy()
		for _, h := range []windows.Handle{w.editFont, w.editBrush, w.iconBig, w.iconSmall} {
			if h != 0 {
				pDeleteObject.Call(uintptr(h))
			}
		}
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProc.Call(hwnd, message, wParam, lParam)
	return r
}

func (w *Window) commitEdits() {
	if w.cb.SetUpstream != nil {
		w.cb.SetUpstream(w.up, editText(w.socksEdit))
	}
	if w.cb.SetBridges != nil {
		w.cb.SetBridges(editText(w.bridgesEdit))
	}
}

func (w *Window) refresh() {
	if w.cb.Status == nil {
		return
	}
	st := w.cb.Status()
	w.mu.Lock()
	changed := st.Running != w.st.Running || st.Bootstrap != w.st.Bootstrap ||
		st.ExitIP != w.st.ExitIP || st.BootstrapMsg != w.st.BootstrapMsg ||
		st.Err != w.st.Err || len(st.Installs) != len(w.st.Installs) ||
		st.Intercept != w.st.Intercept
	w.st = st
	w.mu.Unlock()
	if changed {
		w.invalidate()
	}
}

func (w *Window) hitAt(x, y int32) int {
	for i, z := range w.hitZones {
		if z.r.contains(x, y) {
			return i
		}
	}
	return -1
}

func (w *Window) onMouseMove(x, y int32) {
	tme := trackMouseEvent{
		cbSize:    uint32(unsafe.Sizeof(trackMouseEvent{})),
		dwFlags:   tmeLeave,
		hwndTrack: w.hwnd,
	}
	pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))

	idx := w.hitAt(x, y)
	if idx == w.hover {
		return
	}
	prev := w.hover
	w.hover = idx

	cur := uintptr(idcArrow)
	if idx >= 0 {
		cur = idcHand
	}
	h, _, _ := pLoadCursor.Call(0, cur)
	pSetCursor.Call(h)

	// Invalida so os dois retangulos envolvidos: repintar a janela inteira
	// a cada movimento do mouse e desperdicio e fica visivel.
	w.invalidateZone(prev)
	w.invalidateZone(idx)
}

// ---------------------------------------------------------------- acoes

func (w *Window) toggleConnect() {
	st := w.cb.Status()
	if st.Running {
		if msg := w.cb.Stop(); msg != "" {
			w.Flash(msg, 2)
		}
		return
	}
	w.commitEdits()
	if msg := w.cb.Start(w.mode); msg != "" {
		w.Flash(msg, 2)
	}
	w.refresh()
}

func (w *Window) setUp(v string) {
	w.mu.Lock()
	w.up = v
	w.mu.Unlock()
	w.cb.SetUpstream(v, editText(w.socksEdit))
	w.invalidate()
}

func (w *Window) setMode(v string) {
	w.mu.Lock()
	w.mode = v
	w.mu.Unlock()
	w.invalidate()
}

func (w *Window) togglePref(which int) {
	w.mu.Lock()
	switch which {
	case 0:
		w.cfg.AskOnClose = !w.cfg.AskOnClose
	case 1:
		w.cfg.AutoConnect = !w.cfg.AutoConnect
	case 2:
		w.cfg.Notify = !w.cfg.Notify
	case 3:
		w.cfg.FallbackDirect = !w.cfg.FallbackDirect
	}
	c := w.cfg
	w.mu.Unlock()

	if which == 3 {
		w.cb.SetFallback(c.FallbackDirect)
	} else {
		w.cb.SetPrefs(c.AskOnClose, c.AutoConnect, c.Notify)
	}
	w.invalidate()
}
