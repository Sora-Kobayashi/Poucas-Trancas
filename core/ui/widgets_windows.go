// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package ui

// Controles desenhados: botao, opcao e caixa de marcar. Cada um pinta e
// registra a propria area clicavel na mesma chamada, entao o que se ve e o
// que responde nunca divergem.

import "golang.org/x/sys/windows"

// button devolve a borda direita, para encadear botoes numa linha.
func (w *Window) button(c *canvas, r rect, label string, font windows.Handle,
	primary, enabled bool, fn func()) int32 {

	idx := -1
	if enabled {
		idx = w.addHit(r, fn)
	}
	hot := idx >= 0 && idx == w.hover
	down := idx >= 0 && idx == w.pressed

	switch {
	case !enabled:
		c.roundRect(r, 8, -1, int64(colLine))
		c.text(label, r, textOpts{font, colFaint, dtCenter | dtVCenter | dtSingle, 0})

	case primary:
		top, bot := colAcc, colAccSoft
		if down {
			top, bot = colAccSoft, colAccSoft
		} else if hot {
			top = rgb(255, 80, 96)
		}
		c.roundRect(r, 8, int64(bot), -1)
		c.vGradient(rect{r.Left + 1, r.Top + 1, r.Right - 1, r.Bottom - 1}, top, bot, 8)
		c.roundRect(r, 8, -1, int64(bot))
		c.text(label, r, textOpts{font, rgb(255, 255, 255), dtCenter | dtVCenter | dtSingle, 0})

	default:
		bg := int64(-1)
		edge := colLine
		if down {
			bg, edge = int64(rgb(20, 20, 24)), colLineHi
		} else if hot {
			bg, edge = int64(rgb(16, 16, 20)), colLineHi
		}
		c.roundRect(r, 8, bg, int64(edge))
		c.text(label, r, textOpts{font, colText, dtCenter | dtVCenter | dtSingle, 0})
	}
	return r.Right
}

// option e o cartao selecionavel. Devolve a base, para empilhar.
func (w *Window) option(c *canvas, r rect, title, desc string,
	selected bool, fTitle, fDesc windows.Handle, fn func()) int32 {

	idx := w.addHit(r, fn)
	hot := idx == w.hover

	switch {
	case selected:
		c.roundRect(r, 8, int64(colAccBG), int64(colAcc))
	case hot:
		c.roundRect(r, 8, int64(rgb(14, 14, 17)), int64(colLineHi))
	default:
		c.roundRect(r, 8, -1, int64(colLine))
	}

	descCol := colDim
	if selected {
		descCol = rgb(224, 182, 187)
	}
	c.text(title, rect{r.Left + 11, r.Top + 6, r.Right - 8, r.Top + 22},
		textOpts{fTitle, colText, dtLeft | dtSingle, 0})
	c.text(desc, rect{r.Left + 11, r.Top + 22, r.Right - 8, r.Bottom - 3},
		textOpts{fDesc, descCol, dtLeft | dtWordBrk, 0})
	return r.Bottom
}

// check e a caixa de marcar. Devolve a base.
func (w *Window) check(c *canvas, r rect, label string, on bool,
	font windows.Handle, fn func()) int32 {

	idx := w.addHit(r, fn)
	hot := idx == w.hover

	box := rect{r.Left, r.Top + 3, r.Left + 16, r.Top + 19}
	if on {
		c.roundRect(box, 4, int64(colAcc), int64(colAcc))
		c.checkMark(box, rgb(255, 255, 255))
	} else {
		edge := colLine
		if hot {
			edge = colLineHi
		}
		c.roundRect(box, 4, -1, int64(edge))
	}

	col := colDim
	if hot {
		col = colText
	}
	h := c.measure(label, r.w()-24, font, dtWordBrk)
	c.text(label, rect{r.Left + 23, r.Top + 2, r.Right, r.Top + 2 + h + 4},
		textOpts{font, col, dtLeft | dtWordBrk, 0})
	if h+8 > r.h() {
		return r.Top + h + 8
	}
	return r.Bottom
}
