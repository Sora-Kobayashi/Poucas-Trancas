// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package ui

// Ligacao com o Win32 usada pela janela: constantes, structs e ponteiros
// para as funcoes. Tudo por syscall, sem cgo.

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsVScroll          = 0x00200000
	wsBorder           = 0x00800000
	wsTabstop          = 0x00010000
	esMultiline        = 0x0004
	esAutoVScroll      = 0x0040
	esAutoHScroll      = 0x0080

	wmDestroy       = 0x0002
	wmSize          = 0x0005
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmGetMinMaxInfo = 0x0024
	wmCommand       = 0x0111
	wmTimer         = 0x0113
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmMouseLeave    = 0x02A3
	wmCtlColorEdit  = 0x0133
	wmSetFont       = 0x0030
	wmSetIcon       = 0x0080

	iconSmall = 0
	iconBig   = 1

	swShow    = 5
	swHide    = 0
	swRestore = 9

	idcArrow = 32512
	idcHand  = 32649

	dtLeft     = 0x0000
	dtCenter   = 0x0001
	dtVCenter  = 0x0004
	dtSingle   = 0x0020
	dtWordBrk  = 0x0010
	dtEndEllip = 0x8000
	dtCalcRect = 0x0400
	dtNoPrefix = 0x0800

	transparentBk = 1
	psSolid       = 0
	nullPen       = 8
	nullBrush     = 5

	tmeLeave = 0x00000002
)

// Autor e projeto. Ficam num lugar so; o rodape da janela e o menu da
// bandeja apontam para ca.
const (
	authorName = "Sora-Kobayashi"
	repoURL    = "https://github.com/Sora-Kobayashi/Poucas-Trancas"
)

// openURL abre um link no navegador padrao.
func openURL(u string) {
	verb := utf16ptr("open")
	target := utf16ptr(u)
	pShellExecute.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(target)),
		0, 0, 1 /* SW_SHOWNORMAL */)
}

type rect struct{ Left, Top, Right, Bottom int32 }

func (r rect) w() int32 { return r.Right - r.Left }
func (r rect) h() int32 { return r.Bottom - r.Top }

func (r rect) contains(x, y int32) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
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

type paintStruct struct {
	hdc         windows.Handle
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

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

type minMaxInfo struct {
	ptReserved     point
	ptMaxSize      point
	ptMaxPosition  point
	ptMinTrackSize point
	ptMaxTrackSize point
}

type trackMouseEvent struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   windows.Handle
	dwHoverTime uint32
}

type logFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [32]uint16
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	pRegisterClassEx  = user32.NewProc("RegisterClassExW")
	pCreateWindowEx   = user32.NewProc("CreateWindowExW")
	pDefWindowProc    = user32.NewProc("DefWindowProcW")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pShowWindow       = user32.NewProc("ShowWindow")
	pUpdateWindow     = user32.NewProc("UpdateWindow")
	pGetMessage       = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessage  = user32.NewProc("DispatchMessageW")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pBeginPaint       = user32.NewProc("BeginPaint")
	pEndPaint         = user32.NewProc("EndPaint")
	pGetClientRect    = user32.NewProc("GetClientRect")
	pInvalidateRect   = user32.NewProc("InvalidateRect")
	pFillRect         = user32.NewProc("FillRect")
	pDrawTextEx       = user32.NewProc("DrawTextExW")
	pSetTimer         = user32.NewProc("SetTimer")
	pKillTimer        = user32.NewProc("KillTimer")
	pLoadCursor       = user32.NewProc("LoadCursorW")
	pSetCursor        = user32.NewProc("SetCursor")
	pTrackMouseEvent  = user32.NewProc("TrackMouseEvent")
	pSetWindowPos     = user32.NewProc("SetWindowPos")
	pGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	pSendMessage      = user32.NewProc("SendMessageW")
	pSetWindowText    = user32.NewProc("SetWindowTextW")
	pGetWindowText    = user32.NewProc("GetWindowTextW")
	pGetWindowTextLen = user32.NewProc("GetWindowTextLengthW")
	pMessageBox       = user32.NewProc("MessageBoxW")
	pLoadImage        = user32.NewProc("LoadImageW")
	pIsWindowVisible  = user32.NewProc("IsWindowVisible")
	pSetForegroundWin = user32.NewProc("SetForegroundWindow")
	pGetDpiForWindow  = user32.NewProc("GetDpiForWindow")
	pGetDpiForSystem  = user32.NewProc("GetDpiForSystem")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pRoundRect              = gdi32.NewProc("RoundRect")
	pEllipse                = gdi32.NewProc("Ellipse")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pCreateFontIndirect     = gdi32.NewProc("CreateFontIndirectW")
	pGetStockObject         = gdi32.NewProc("GetStockObject")

	pGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	pShellExecute    = shell32.NewProc("ShellExecuteW")
)

func utf16ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func rgb(r, g, b byte) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

func loWord(v uintptr) int32 { return int32(int16(v & 0xFFFF)) }
func hiWord(v uintptr) int32 { return int32(int16((v >> 16) & 0xFFFF)) }

func editText(h windows.Handle) string {
	n, _, _ := pGetWindowTextLen.Call(uintptr(h))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	pGetWindowText.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}
