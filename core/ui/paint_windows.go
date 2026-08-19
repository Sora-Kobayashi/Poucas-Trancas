// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package ui

// Layout e pintura. Uma passada so: calcula posicao, desenha e registra as
// areas clicaveis — assim o que se ve e o que responde ao clique nunca
// divergem.

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"poucastrancas/core"
)

const (
	pad     = 16
	gap     = 10
	cardPad = 13
)

func (w *Window) onPaint() {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&ps)))

	var cr rect
	pGetClientRect.Call(uintptr(w.hwnd), uintptr(unsafe.Pointer(&cr)))

	// Buffer fora da tela: desenhar direto no DC da janela pisca.
	memDC, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(cr.w()), uintptr(cr.h()))
	old, _, _ := pSelectObject.Call(memDC, bmp)
	defer func() {
		pSelectObject.Call(memDC, old)
		pDeleteObject.Call(bmp)
		pDeleteDC.Call(memDC)
	}()

	sc := w.dpiScale()
	w.sc = sc
	c := newCanvas(windows.Handle(memDC), sc, w.fonts)

	w.hitZones = w.hitZones[:0]
	// O layout pensa em 96 dpi; a area util e convertida na entrada.
	logical := rect{0, 0, int32(float64(cr.Right)/sc + 0.5), int32(float64(cr.Bottom)/sc + 0.5)}
	w.draw(c, logical)

	pBitBlt.Call(hdc, 0, 0, uintptr(cr.w()), uintptr(cr.h()), memDC, 0, 0, 0x00CC0020)
}

// addHit converte para pixel de tela: o mouse chega em coordenada real, e
// o layout e escrito em pixel logico.
func (w *Window) addHit(r rect, fn func()) int {
	w.hitZones = append(w.hitZones, clickable{w.canvasScale(r), fn})
	return len(w.hitZones) - 1
}

func (w *Window) draw(c *canvas, cr rect) {
	w.mu.Lock()
	st, cfg, flash, flashK := w.st, w.cfg, w.flash, w.flashK
	w.mu.Unlock()

	c.fill(cr, colBG)

	fTitle := c.font("Segoe UI", 17, true)
	fBody := c.font("Segoe UI", 13, false)
	fSmall := c.font("Segoe UI", 12, false)
	fTiny := c.font("Segoe UI", 11, false)
	fLabel := c.font("Segoe UI", 11, true)
	fMonoBig := c.font("Consolas", 24, true)
	fMono := c.font("Consolas", 13, true)
	fMonoS := c.font("Consolas", 11, false)

	x0, x1 := int32(pad), cr.Right-pad
	y := int32(pad)

	// ── cabecalho: "Poucas" branco + "Trancas" vermelho, medindo a largura
	//    real para o vermelho e o subtitulo nao dependerem de offset magico.
	c.text("Poucas ", rect{x0, y, x1, y + 24}, textOpts{fTitle, colText, dtLeft | dtSingle, 0})
	w1 := c.textWidth("Poucas ", fTitle)
	c.text("Trancas", rect{x0 + w1, y, x1, y + 24}, textOpts{fTitle, colAcc, dtLeft | dtSingle, 0})
	w2 := c.textWidth("Trancas", fTitle)
	c.text("Discord pelo Tor, sem tocar no Discord",
		rect{x0 + w1 + w2 + 12, y + 4, x1, y + 24}, textOpts{fSmall, colDim, dtLeft | dtSingle, 0})
	y += 32

	// ── painel de estado ─────────────────────────────────────────────
	heroTop := y
	heroH := int32(146)
	if st.Running && (st.Intercept.Redirected > 0 || st.Intercept.Returned > 0) {
		heroH += 44
	}
	hero := rect{x0, heroTop, x1, heroTop + heroH}
	c.roundRect(hero, 12, int64(colPanel), int64(colLine))
	// Brilho no topo por gradiente. Antes eram dois preenchimentos chapados
	// e a divisa entre eles aparecia como um risco atravessando o painel.
	c.vGradient(rect{hero.Left + 1, hero.Top + 1, hero.Right - 1, hero.Top + 46},
		colPanel2, colPanel, 12)
	c.roundRect(hero, 12, -1, int64(colLine))

	inX := x0 + cardPad
	inW := x1 - cardPad
	cy := heroTop + 14

	dotCol := colFaint
	switch {
	case st.Err != "":
		dotCol = colBad
	case st.Running:
		dotCol = colOK
	case st.Bootstrap > 0:
		dotCol = colWarn
	}
	c.circle(inX+5, cy+13, 5, dotCol)

	ipTxt, ipFont, ipCol := "desconectado", fMono, colFaint
	if st.Running && st.ExitIP != "" {
		ipTxt, ipFont, ipCol = st.ExitIP, fMonoBig, colText
	} else if st.Bootstrap > 0 {
		ipTxt = "conectando"
	}
	c.text(ipTxt, rect{inX + 20, cy, inW - 60, cy + 30}, textOpts{ipFont, ipCol, dtLeft | dtSingle, 0})
	if st.ExitLoc != "" {
		lr := rect{inW - 46, cy + 4, inW, cy + 22}
		c.roundRect(lr, 5, int64(rgb(23, 23, 27)), -1)
		c.text(st.ExitLoc, lr, textOpts{fTiny, colDim, dtCenter | dtVCenter | dtSingle, 0})
	}
	cy += 36

	// barra de progresso
	barBG := rect{inX, cy, inW, cy + 4}
	c.roundRect(barBG, 2, int64(rgb(23, 23, 27)), -1)
	if st.Bootstrap > 0 {
		fw := (inW - inX) * int32(st.Bootstrap) / 100
		if fw > 0 {
			c.vGradient(rect{inX, cy, inX + fw, cy + 4}, colAccSoft, colAcc, 0)
		}
	}
	cy += 12

	phase := st.BootstrapMsg
	if phase == "" {
		if st.Running {
			phase = st.Proxy
		} else {
			phase = "parado"
		}
	}
	c.text(phase, rect{inX, cy, inW, cy + 18}, textOpts{fSmall, colDim, dtLeft | dtSingle | dtEndEllip, 0})
	cy += 24

	// botoes
	label := "Conectar"
	if st.Running {
		label = "Desconectar"
	}
	bx := inX
	bx = w.button(c, rect{bx, cy, bx + 118, cy + 34}, label, fBody, true, true, w.toggleConnect)
	bx = w.button(c, rect{bx + 8, cy, bx + 8 + 128, cy + 34}, "Trocar circuito", fBody, false, st.Running,
		func() { w.cb.NewIdentity() })
	w.button(c, rect{bx + 8, cy, bx + 8 + 128, cy + 34}, "Verificar saída", fBody, false, st.Running,
		func() { w.cb.RefreshExit() })
	cy += 42

	if st.Running && (st.Intercept.Redirected > 0 || st.Intercept.Returned > 0) {
		c.fill(rect{inX, cy, inW, cy + 1}, colLine)
		cy += 10
		it := st.Intercept
		sx := inX
		stats := []struct {
			v   uint64
			lbl string
			col uint32
		}{
			{it.Redirected, "DESVIADOS", colAcc},
			{it.Returned, "RETORNADOS", colText},
			{uint64(it.Active), "ATIVOS", colText},
			{it.Served, "VIA SAÍDA", colText},
		}
		if it.DroppedV6 > 0 {
			stats = append(stats, struct {
				v   uint64
				lbl string
				col uint32
			}{it.DroppedV6, "V6→V4", colText})
		}
		if it.Direct > 0 {
			stats = append(stats, struct {
				v   uint64
				lbl string
				col uint32
			}{it.Direct, "POR FORA", colBad})
		}
		if it.Failed > 0 {
			stats = append(stats, struct {
				v   uint64
				lbl string
				col uint32
			}{it.Failed, "FALHAS", colWarn})
		}
		for _, s := range stats {
			c.text(fmt.Sprint(s.v), rect{sx, cy, sx + 90, cy + 18},
				textOpts{fMono, s.col, dtLeft | dtSingle, 0})
			c.text(s.lbl, rect{sx, cy + 18, sx + 90, cy + 32},
				textOpts{fTiny, colFaint, dtLeft | dtSingle, 0})
			sx += 92
		}
	}
	y = hero.Bottom + gap

	// ── dois cartoes lado a lado ─────────────────────────────────────
	colW := (x1 - x0 - gap) / 2
	cardH := int32(152)

	left := rect{x0, y, x0 + colW, y + cardH}
	c.roundRect(left, 10, int64(colPanel), int64(colLine))
	c.text("SAÍDA DO TRÁFEGO", rect{left.Left + cardPad, left.Top + 11, left.Right, left.Top + 26},
		textOpts{fLabel, colFaint, dtLeft | dtSingle, 0})

	oy := left.Top + 32
	oy = w.option(c, rect{left.Left + cardPad, oy, left.Right - cardPad, oy + 42},
		"Tor", "Anônimo. Força IPv4: nó de saída com IPv6 é raro.",
		w.up == "tor", fBody, fTiny, func() { w.setUp("tor") })
	oy = w.option(c, rect{left.Left + cardPad, oy + 6, left.Right - cardPad, oy + 6 + 42},
		"SOCKS5 próprio", "Sem política de saída: qualquer porta passa.",
		w.up == "socks", fBody, fTiny, func() { w.setUp("socks") })

	// campo do SOCKS: controle nativo, posicionado sobre o layout
	sr := w.canvasScale(rect{left.Left + cardPad, oy + 6, left.Right - cardPad, oy + 6 + 24})
	placeEdit(w.socksEdit, &w.socksRect, sr)
	if w.up != "socks" {
		c.text("O tráfego sai por nós do Tor, escolhidos a cada circuito.",
			rect{left.Left + cardPad, oy + 10, left.Right - cardPad, oy + 34},
			textOpts{fTiny, colFaint, dtLeft | dtWordBrk, 0})
	}
	if show := w.up == "socks"; show != w.socksShown {
		w.socksShown = show
		pShowWindow.Call(uintptr(w.socksEdit), uintptr(map[bool]int{true: swShow, false: swHide}[show]))
	}

	right := rect{x1 - colW, y, x1, y + cardH}
	c.roundRect(right, 10, int64(colPanel), int64(colLine))
	c.text("VOZ E TELA", rect{right.Left + cardPad, right.Top + 11, right.Right, right.Top + 26},
		textOpts{fLabel, colFaint, dtLeft | dtSingle, 0})

	my := right.Top + 32
	my = w.option(c, rect{right.Left + cardPad, my, right.Right - cardPad, my + 42},
		"Deixar passar", "Funcionam. São UDP: o servidor de mídia vê seu IP real.",
		w.mode == "direct", fBody, fTiny, func() { w.setMode("direct") })
	my = w.option(c, rect{right.Left + cardPad, my + 6, right.Right - cardPad, my + 6 + 42},
		"Bloquear", "Nada vaza por UDP. Voz e tela param.",
		w.mode == "block", fBody, fTiny, func() { w.setMode("block") })

	w.check(c, rect{right.Left + cardPad, my + 8, right.Right - cardPad, my + 8 + 30},
		"Se a saída recusar, discar por fora (expõe seu IP)",
		cfg.FallbackDirect, fTiny, func() { w.togglePref(3) })
	y = left.Bottom + gap

	// ── clientes do Discord ──────────────────────────────────────────
	rows := int32(len(st.Installs))
	if rows == 0 {
		rows = 1
	}
	listH := 34 + rows*30
	list := rect{x0, y, x1, y + listH}
	c.roundRect(list, 10, int64(colPanel), int64(colLine))
	c.text("CLIENTES ENCONTRADOS", rect{list.Left + cardPad, list.Top + 11, list.Right, list.Top + 26},
		textOpts{fLabel, colFaint, dtLeft | dtSingle, 0})

	ry := list.Top + 32
	if len(st.Installs) == 0 {
		c.text("nenhum cliente Discord encontrado",
			rect{list.Left + cardPad, ry, list.Right - cardPad, ry + 20},
			textOpts{fSmall, colFaint, dtLeft | dtSingle, 0})
	}
	for _, in := range st.Installs {
		pill := rect{list.Left + cardPad, ry + 5, list.Left + cardPad + 58, ry + 21}
		pc, pt := colFaint, "FECHADO"
		pb := rgb(22, 22, 26)
		if in.Running {
			pc, pt, pb = colOK, "ABERTO", rgb(10, 36, 22)
		}
		c.roundRect(pill, 4, int64(pb), -1)
		c.text(pt, pill, textOpts{fTiny, pc, dtCenter | dtVCenter | dtSingle, 0})

		c.text(in.Flavor, rect{pill.Right + 10, ry + 2, pill.Right + 130, ry + 20},
			textOpts{fSmall, colText, dtLeft | dtSingle, 0})
		c.text(in.Dir, rect{pill.Right + 10, ry + 15, list.Right - 110, ry + 30},
			textOpts{fMonoS, colFaint, dtLeft | dtSingle | dtEndEllip, 0})

		dir := in.Dir
		w.button(c, rect{list.Right - cardPad - 84, ry + 3, list.Right - cardPad, ry + 27},
			"Reiniciar", fTiny, false, true, func() {
				if e := w.cb.Restart(dir); e != "" {
					w.Flash(e, 2)
				} else {
					w.Flash("Discord reiniciado — conexões novas passam pelo desvio.", 1)
				}
			})
		ry += 30
	}
	y = list.Bottom + gap

	// ── pontes + preferencias ────────────────────────────────────────
	advH := int32(150)
	adv := rect{x0, y, x1, y + advH}
	c.roundRect(adv, 10, int64(colPanel), int64(colLine))
	c.text("PONTES OBFS4  ·  só se a rede bloquear o Tor",
		rect{adv.Left + cardPad, adv.Top + 11, adv.Right, adv.Top + 26},
		textOpts{fLabel, colFaint, dtLeft | dtSingle, 0})

	brl := rect{adv.Left + cardPad, adv.Top + 32, adv.Left + colW, adv.Top + 96}
	placeEdit(w.bridgesEdit, &w.bridgesRect, w.canvasScale(brl))

	c.text("Cole linhas de bridges.torproject.org. Valem na próxima conexão.",
		rect{adv.Left + cardPad, brl.Bottom + 6, adv.Left + colW, brl.Bottom + 40},
		textOpts{fTiny, colFaint, dtLeft | dtWordBrk, 0})

	px := adv.Left + colW + 20
	py := adv.Top + 32
	py = w.check(c, rect{px, py, adv.Right - cardPad, py + 26},
		"Perguntar ao fechar a janela", cfg.AskOnClose, fTiny, func() { w.togglePref(0) })
	py = w.check(c, rect{px, py + 2, adv.Right - cardPad, py + 28},
		"Conectar sozinho ao abrir", cfg.AutoConnect, fTiny, func() { w.togglePref(1) })
	w.check(c, rect{px, py + 2, adv.Right - cardPad, py + 28},
		"Notificar conexão, falha e vazamento", cfg.Notify, fTiny, func() { w.togglePref(2) })
	y = adv.Bottom + gap

	// ── avisos ───────────────────────────────────────────────────────
	notes := w.notes(st, cfg)
	if flash != "" {
		notes = append([]note{{flash, flashK}}, notes...)
	}
	for _, n := range notes {
		if y > cr.Bottom-30 {
			break
		}
		bg, fg, edge := rgb(26, 20, 8), rgb(233, 217, 182), colWarn
		switch n.kind {
		case 1:
			bg, fg, edge = rgb(8, 25, 15), rgb(191, 230, 205), colOK
		case 2:
			bg, fg, edge = rgb(28, 10, 12), rgb(245, 196, 196), colBad
		}
		h := c.measure(n.text, x1-x0-24, fSmall, dtWordBrk) + 18
		nr := rect{x0, y, x1, y + h}
		c.roundRect(nr, 6, int64(bg), -1)
		c.fill(rect{x0, y, x0 + 3, y + h}, edge)
		c.text(n.text, rect{x0 + 13, y + 8, x1 - 10, y + h}, textOpts{fSmall, fg, dtLeft | dtWordBrk, 0})
		y += h + 6
	}
}

type note struct {
	text string
	kind int // 0 aviso, 1 ok, 2 erro
}

func (w *Window) notes(st core.Status, cfg core.Config) []note {
	var out []note
	if !st.Elevated {
		out = append(out, note{"Sem privilégio de administrador — o driver de rede não carrega. Feche e abra aceitando o aviso do Windows.", 2})
	}
	if !st.TorEmbedded {
		out = append(out, note{"Este build não tem o Tor embutido. Rode: go run ./cmd/fetchdeps", 2})
	}
	if st.Err != "" {
		out = append(out, note{st.Err, 2})
	}
	if st.Running && st.Intercept.Direct > 0 {
		out = append(out, note{fmt.Sprintf("%d conexão(ões) saíram por fora da rota anônima. Seu IP real ficou exposto nelas.", st.Intercept.Direct), 2})
	}
	if st.Running && st.Intercept.SendErr > 0 {
		out = append(out, note{"Driver recusou pacotes: " + st.Intercept.LastErr, 2})
	}
	if st.Running && st.Intercept.Failed > 0 && !cfg.FallbackDirect {
		out = append(out, note{fmt.Sprintf("%d conexão(ões) recusadas pela saída. Ligue o fallback se preferir funcionalidade.", st.Intercept.Failed), 0})
	}
	if st.Running && st.UDPMode == "direct" {
		out = append(out, note{"Voz e tela saem direto: o servidor de mídia do Discord vê seu IP real.", 0})
	}
	return out
}
