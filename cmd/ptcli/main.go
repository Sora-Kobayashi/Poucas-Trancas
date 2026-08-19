//go:build linux

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Sora-Kobayashi. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// ptcli é a alternativa Linux, por linha de comando.
//
// # POR QUE É DIFERENTE DO WINDOWS
//
// A versão Windows desvia o tráfego do Discord por PID com um driver de
// kernel (WinDivert). Em Linux não existe WinDivert; há vários caminhos, e
// a CLI expõe cada um explicitamente pelo -mode:
//
//	-mode netns    (RECOMENDADO) roda o app numa network namespace cuja
//	               única saída é o Tor. Pega TODO o tráfego do app, inclusive
//	               o do Chromium/Electron. Falha FECHADA: se algo der errado,
//	               a namespace fica sem rota e nada vaza. Precisa de root.
//
//	-mode nfqueue  (EXPERIMENTAL) o gêmeo do WinDivert: o pacote sobe pra
//	               userspace, é reescrito e devolvido. Reescrever em userspace
//	               pode falhar ABERTA (vazar) se houver bug — perigoso num app
//	               de anonimato. Veja o aviso ao selecionar.
//
//	-mode socks    só sobe o Tor e mostra o SOCKS5; você aponta o app. Mais
//	               simples, mas o Chromium do Discord pode escapar.
//
// Em QUALQUER modo: voz e tela são UDP e o Tor não os carrega — continuam
// expondo o IP real, igual no Windows.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"poucastrancas/core"
)

const repoURL = "https://github.com/Sora-Kobayashi/Poucas-Trancas"

func main() {
	mode := flag.String("mode", "", "netns | nfqueue | socks  (escolha explícita)")
	flag.Usage = usage
	flag.Parse()

	switch *mode {
	case "netns":
		runNetns(flag.Args())
	case "socks":
		runSocks()
	case "nfqueue":
		runNFQueue(flag.Args())
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Poucas Trancas (Linux) — %s\n\n", repoURL)
	fmt.Fprintln(os.Stderr, "escolha o modo explicitamente com -mode:")
	fmt.Fprintln(os.Stderr, "  -mode netns    -- discord     app roda numa netns só com saída Tor (recomendado, root)")
	fmt.Fprintln(os.Stderr, "  -mode socks                   sobe o Tor e mostra o SOCKS5 para você apontar o app")
	fmt.Fprintln(os.Stderr, "  -mode nfqueue  -- discord     reescreve pacotes em userspace (experimental, root)")
	fmt.Fprintln(os.Stderr, "\nem qualquer modo: voz e tela são UDP e continuam expondo seu IP real.")
}

// startTor sobe o Tor do sistema (Linux não embute binário) e devolve o
// cliente pronto. transparent = também abrir TransPort/DNSPort (modo netns).
func startTor(transparent bool) *core.Tor {
	torPath, err := exec.LookPath("tor")
	if err != nil {
		fatal("tor não encontrado. Instale: sudo apt install tor")
	}
	dir := filepath.Join(os.TempDir(), "poucas-trancas-cli")
	t := core.NewTor(torPath, dir, 0, 0)
	t.TransparentPorts = transparent

	fmt.Println("subindo Tor…")
	last := -1
	if err := t.StartAuto(func(pct int, _ string) {
		if pct/20 != last/20 {
			fmt.Printf("  bootstrap %d%%\n", pct)
			last = pct
		}
	}); err != nil {
		fatal(fmt.Sprintf("Tor falhou: %v", err))
	}
	return t
}

func runSocks() {
	t := startTor(false)
	defer t.Stop()
	fmt.Printf("\nTor pronto.  SOCKS5: socks5://%s\n", t.SocksAddr)
	fmt.Println("Aponte o app para esse SOCKS (Discord: --proxy-server=socks5://…).")
	fmt.Println("Lembre: voz e tela são UDP e continuam expondo seu IP real.")
	fmt.Println("Ctrl+C encerra.")
	waitSignal()
	fmt.Println("\nencerrando…")
}

func mustArgs(args []string, mode string) {
	if len(args) == 0 {
		fatal(fmt.Sprintf("modo %s precisa de um programa: ptcli -mode %s -- discord", mode, mode))
	}
}

func waitSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "erro:", msg)
	os.Exit(1)
}

// run executa um comando externo herdando stdio; fatal se falhar.
func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		fatal(fmt.Sprintf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out))
	}
}

// tryRun executa e ignora o erro (usado na limpeza best-effort).
func tryRun(name string, args ...string) {
	_ = exec.Command(name, args...).Run()
}
