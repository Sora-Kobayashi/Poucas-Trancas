// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package ui

// Primitivas de desenho. Tudo em GDI puro sobre um buffer fora da tela: o
// WM_PAINT copia o buffer pronto de uma vez, senao a janela pisca a cada
// atualizacao de estado.

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Paleta AMOLED: preto real economiza pixel em tela OLED, e o vermelho
// aparece so no que exige atencao — botao principal, selecao e alerta.
var (
	colBG      = rgb(0, 0, 0)
	colPanel   = rgb(10, 10, 12)
	colPanel2  = rgb(18, 18, 21)
	colLine    = rgb(30, 30, 34)
	colLineHi  = rgb(46, 46, 52)
	colText    = rgb(242, 242, 244)
	colDim     = rgb(142, 142, 150)
	colFaint   = rgb(90, 90, 98)
	colAcc     = rgb(255, 45, 64)
	colAccSoft = rgb(196, 31, 46)
	colAccBG   = rgb(26, 5, 7)
	colOK      = rgb(49, 214, 122)
	colWarn    = rgb(255, 167, 38)
	colBad     = rgb(255, 82, 82)
)

type canvas struct {
	dc    windows.Handle
	fonts *fontCache

	// sc converte pixel logico (96 dpi) em pixel de tela. Fica aqui e nao
	// no layout para o codigo de desenho continuar legivel: quem escreve a
	// tela pensa em 96 dpi e o canvas resolve o resto.
	sc float64
}

func newCanvas(dc windows.Handle, scale float64, fonts *fontCache) *canvas {
	if scale < 1 {
		scale = 1
	}
	return &canvas{dc: dc, fonts: fonts, sc: scale}
}

func (c *canvas) px(v int32) int32 { return int32(float64(v)*c.sc + 0.5) }

func (c *canvas) scaleRect(r rect) rect {
	return rect{c.px(r.Left), c.px(r.Top), c.px(r.Right), c.px(r.Bottom)}
}

// fontCache guarda as fontes entre frames. Criar fonte a cada onPaint —
// que roda a cada segundo e a cada movimento do mouse — desperdicaria
// dezenas de CreateFontIndirect por segundo. O cache e chaveado por
// (nome, tamanho em px, negrito): muda com o DPI, entao troca de monitor
// gera fontes novas, o que e correto.
type fontCache struct {
	m map[string]windows.Handle
}

func newFontCache() *fontCache { return &fontCache{m: map[string]windows.Handle{}} }

func (fc *fontCache) destroy() {
	for _, h := range fc.m {
		pDeleteObject.Call(uintptr(h))
	}
	fc.m = map[string]windows.Handle{}
}

func (c *canvas) font(name string, size int32, bold bool) windows.Handle {
	px := c.px(size)
	key := fmt.Sprintf("%s|%d|%t", name, px, bold)
	if h, ok := c.fonts.m[key]; ok {
		return h
	}
	lf := logFont{
		Height:  -px,
		Weight:  400,
		CharSet: 1, // DEFAULT_CHARSET: mantem acentuacao
		Quality: 5, // CLEARTYPE_QUALITY
	}
	if bold {
		lf.Weight = 700
	}
	src := windows.StringToUTF16(name)
	copy(lf.FaceName[:], src[:min(len(src), 31)])

	h, _, _ := pCreateFontIndirect.Call(uintptr(unsafe.Pointer(&lf)))
	c.fonts.m[key] = windows.Handle(h)
	return windows.Handle(h)
}

func (c *canvas) fill(r rect, color uint32) {
	dr := c.scaleRect(r)
	br, _, _ := pCreateSolidBrush.Call(uintptr(color))
	pFillRect.Call(uintptr(c.dc), uintptr(unsafe.Pointer(&dr)), br)
	pDeleteObject.Call(br)
}

// fillRaw pinta sem escalar; usado por quem ja esta em pixel de tela.
func (c *canvas) fillRaw(r rect, color uint32) {
	br, _, _ := pCreateSolidBrush.Call(uintptr(color))
	pFillRect.Call(uintptr(c.dc), uintptr(unsafe.Pointer(&r)), br)
	pDeleteObject.Call(br)
}

// roundRect desenha painel com canto arredondado. fill<0 = sem preenchimento,
// stroke<0 = sem borda.
func (c *canvas) roundRect(r rect, radius int32, fill, stroke int64) {
	var oldPen, oldBrush uintptr

	if stroke >= 0 {
		pen, _, _ := pCreatePen.Call(psSolid, 1, uintptr(uint32(stroke)))
		oldPen, _, _ = pSelectObject.Call(uintptr(c.dc), pen)
		defer func() {
			pSelectObject.Call(uintptr(c.dc), oldPen)
			pDeleteObject.Call(pen)
		}()
	} else {
		np, _, _ := pGetStockObject.Call(nullPen)
		oldPen, _, _ = pSelectObject.Call(uintptr(c.dc), np)
		defer pSelectObject.Call(uintptr(c.dc), oldPen)
	}

	if fill >= 0 {
		br, _, _ := pCreateSolidBrush.Call(uintptr(uint32(fill)))
		oldBrush, _, _ = pSelectObject.Call(uintptr(c.dc), br)
		defer func() {
			pSelectObject.Call(uintptr(c.dc), oldBrush)
			pDeleteObject.Call(br)
		}()
	} else {
		nb, _, _ := pGetStockObject.Call(nullBrush)
		oldBrush, _, _ = pSelectObject.Call(uintptr(c.dc), nb)
		defer pSelectObject.Call(uintptr(c.dc), oldBrush)
	}

	dr := c.scaleRect(r)
	rad := c.px(radius) * 2
	pRoundRect.Call(uintptr(c.dc),
		uintptr(dr.Left), uintptr(dr.Top), uintptr(dr.Right), uintptr(dr.Bottom),
		uintptr(rad), uintptr(rad))
}

func (c *canvas) circle(cx, cy, r int32, color uint32) {
	cx, cy, r = c.px(cx), c.px(cy), c.px(r)
	br, _, _ := pCreateSolidBrush.Call(uintptr(color))
	np, _, _ := pGetStockObject.Call(nullPen)
	ob, _, _ := pSelectObject.Call(uintptr(c.dc), br)
	op, _, _ := pSelectObject.Call(uintptr(c.dc), np)
	pEllipse.Call(uintptr(c.dc), uintptr(cx-r), uintptr(cy-r), uintptr(cx+r), uintptr(cy+r))
	pSelectObject.Call(uintptr(c.dc), ob)
	pSelectObject.Call(uintptr(c.dc), op)
	pDeleteObject.Call(br)
}

type textOpts struct {
	font   windows.Handle
	color  uint32
	flags  uint32
	indent int32
}

func (c *canvas) text(s string, r rect, o textOpts) int32 {
	if s == "" {
		return 0
	}
	old, _, _ := pSelectObject.Call(uintptr(c.dc), uintptr(o.font))
	pSetTextColor.Call(uintptr(c.dc), uintptr(o.color))
	pSetBkMode.Call(uintptr(c.dc), transparentBk)

	rr := c.scaleRect(r)
	rr.Left += c.px(o.indent)
	u := windows.StringToUTF16(s)
	h, _, _ := pDrawTextEx.Call(uintptr(c.dc), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&rr)), uintptr(o.flags|dtNoPrefix), 0)
	runtime.KeepAlive(u)

	pSelectObject.Call(uintptr(c.dc), old)
	return int32(h)
}

// measure descobre a altura que o texto ocupara, sem desenhar.
func (c *canvas) measure(s string, width int32, font windows.Handle, flags uint32) int32 {
	if s == "" {
		return 0
	}
	old, _, _ := pSelectObject.Call(uintptr(c.dc), uintptr(font))
	r := rect{0, 0, c.px(width), 0}
	u := windows.StringToUTF16(s)
	pDrawTextEx.Call(uintptr(c.dc), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&r)), uintptr(flags|dtCalcRect|dtNoPrefix), 0)
	runtime.KeepAlive(u)
	pSelectObject.Call(uintptr(c.dc), old)
	return int32(float64(r.Bottom)/c.sc + 0.5)
}

// textWidth devolve a largura do texto em pixel logico, para posicionar
// elementos em sequencia sem offset chutado.
func (c *canvas) textWidth(s string, font windows.Handle) int32 {
	if s == "" {
		return 0
	}
	old, _, _ := pSelectObject.Call(uintptr(c.dc), uintptr(font))
	r := rect{0, 0, 4000, 0}
	u := windows.StringToUTF16(s)
	pDrawTextEx.Call(uintptr(c.dc), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&r)), uintptr(dtSingle|dtCalcRect|dtNoPrefix), 0)
	runtime.KeepAlive(u)
	pSelectObject.Call(uintptr(c.dc), old)
	return int32(float64(r.Right)/c.sc + 0.5)
}

// vGradient aproxima gradiente vertical por faixas. Poucas linhas em uma
// altura pequena ja ficam continuas, e evita depender do msimg32.
func (c *canvas) vGradient(r rect, top, bottom uint32, radius int32) {
	r = c.scaleRect(r)
	radius = c.px(radius)
	steps := r.h()
	if steps <= 0 {
		return
	}
	tr, tg, tb := byte(top&0xFF), byte((top>>8)&0xFF), byte((top>>16)&0xFF)
	br_, bg, bb := byte(bottom&0xFF), byte((bottom>>8)&0xFF), byte((bottom>>16)&0xFF)

	for i := int32(0); i < steps; i++ {
		t := float64(i) / float64(steps)
		col := rgb(
			byte(float64(tr)+(float64(br_)-float64(tr))*t),
			byte(float64(tg)+(float64(bg)-float64(tg))*t),
			byte(float64(tb)+(float64(bb)-float64(tb))*t),
		)
		line := rect{r.Left, r.Top + i, r.Right, r.Top + i + 1}
		// encolhe as pontas para acompanhar o canto arredondado
		if radius > 0 {
			d := radius - i
			if i > steps-radius {
				d = radius - (steps - i)
			}
			if d > 0 {
				inset := radius - int32(sqrt(float64(radius*radius-(radius-d)*(radius-d))))
				line.Left += inset
				line.Right -= inset
			}
		}
		c.fillRaw(line, col)
	}
}

// checkMark desenha o tique dentro da caixa. Feito com passos diagonais
// porque dois retangulos em L nao leem como confirmacao.
func (c *canvas) checkMark(box rect, color uint32) {
	b := c.scaleRect(box)
	w, h := b.w(), b.h()
	th := h / 6
	if th < 2 {
		th = 2
	}
	// desce da esquerda ate o vertice
	x, y := b.Left+w*30/100, b.Top+h*50/100
	for i := int32(0); i < h*22/100; i++ {
		c.fillRaw(rect{x + i, y + i, x + i + th, y + i + th}, color)
	}
	// sobe do vertice ate a direita
	x, y = x+h*22/100, y+h*22/100
	for i := int32(0); i < h*40/100; i++ {
		c.fillRaw(rect{x + i, y - i, x + i + th, y - i + th}, color)
	}
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 12; i++ {
		z = (z + x/z) / 2
	}
	return z
}
