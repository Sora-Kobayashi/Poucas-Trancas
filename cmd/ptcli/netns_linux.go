//go:build linux

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Sora-Kobayashi. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
)

// Recorte da rede: nomes e IPs da network namespace. Escolhidos altos para
// não colidir com redes comuns.
const (
	nsName   = "poucastrancas"
	vethHost = "pt-veth0"
	vethNS   = "pt-veth1"
	hostIP   = "10.66.66.1"
	nsIP     = "10.66.66.2"
	nftTable = "poucastrancas"
)

// runNetns roda um programa numa network namespace cuja ÚNICA saída é o Tor.
//
// Como funciona: a namespace recebe uma rota default para o host; no host, o
// nftables redireciona TODO o TCP que vem dela para o TransPort do Tor (e o
// DNS para o DNSPort). Como o Tor não faz UDP, voz/tela simplesmente não têm
// rota — falha FECHADA, sem vazar. Precisa de root.
func runNetns(args []string) {
	mustArgs(args, "netns")
	requireRoot()
	requireTools("ip", "nft", "sysctl")

	t := startTor(true) // abre TransPort/DNSPort
	defer t.Stop()
	fmt.Printf("Tor pronto (TransPort %d, DNSPort %d).\n", t.TransPort, t.DNSPort)

	// Limpa restos de execução anterior antes de montar.
	teardownNetns()
	setupNetns(t.TransPort, t.DNSPort)
	defer teardownNetns()

	fmt.Println("\nnamespace pronta. Lançando o app com saída só pelo Tor…")
	fmt.Println("Lembre: voz e tela (UDP) NÃO passam pelo Tor — aqui ficam sem rota.")

	cmd := launchInNS(args)
	if err := cmd.Start(); err != nil {
		fatal(fmt.Sprintf("não consegui lançar na namespace: %v", err))
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-sig:
		_ = cmd.Process.Signal(syscall.SIGTERM)
		<-done
	case <-done:
	}
	fmt.Println("\nencerrando e desmontando a namespace…")
}

func setupNetns(transPort, dnsPort int) {
	// namespace + par veth
	run("ip", "netns", "add", nsName)
	run("ip", "link", "add", vethHost, "type", "veth", "peer", "name", vethNS)
	run("ip", "link", "set", vethNS, "netns", nsName)

	// endereços e rota
	run("ip", "addr", "add", hostIP+"/24", "dev", vethHost)
	run("ip", "link", "set", vethHost, "up")
	run("ip", "-n", nsName, "addr", "add", nsIP+"/24", "dev", vethNS)
	run("ip", "-n", nsName, "link", "set", vethNS, "up")
	run("ip", "-n", nsName, "link", "set", "lo", "up")
	run("ip", "-n", nsName, "route", "add", "default", "via", hostIP)

	// REDIRECT para 127.0.0.1 a partir de uma interface não-loopback exige
	// route_localnet ligado nessa interface.
	run("sysctl", "-w", "net.ipv4.conf."+vethHost+".route_localnet=1")

	// nftables: tudo que vem da namespace vira TransPort/DNSPort do Tor.
	// TCP → TransPort, DNS (udp/53) → DNSPort. UDP restante não é tratado,
	// então não tem rota — falha fechada.
	//
	// Usa "dnat to 127.0.0.1:porta" e NÃO "redirect": o redirect do nft manda
	// o pacote para o IP da interface de entrada (10.66.66.1), mas o Tor
	// escuta no loopback (127.0.0.1) — não bateria e daria "connection
	// refused". O dnat explícito para 127.0.0.1, junto do route_localnet
	// ligado acima, entrega no TransPort/DNSPort do Tor. (Validado em WSL2:
	// DNS resolve pelo Tor e o HTTPS sai por nó Tor, tudo dentro da netns.)
	ruleset := fmt.Sprintf(`table ip %[1]s {
  chain prerouting {
    type nat hook prerouting priority dstnat; policy accept;
    iifname "%[2]s" udp dport 53 dnat to 127.0.0.1:%[4]d
    iifname "%[2]s" meta l4proto tcp dnat to 127.0.0.1:%[3]d
  }
}`, nftTable, vethHost, transPort, dnsPort)

	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(ruleset)
	if out, err := cmd.CombinedOutput(); err != nil {
		teardownNetns()
		fatal(fmt.Sprintf("nft falhou: %v\n%s", err, out))
	}
}

func teardownNetns() {
	tryRun("nft", "delete", "table", "ip", nftTable)
	tryRun("ip", "netns", "del", nsName)
	tryRun("ip", "link", "del", vethHost) // remove o par se sobrou
}

// launchInNS monta o comando `ip netns exec … <app>` rodando como o usuário
// real (não root), preservando o ambiente gráfico. GUI funciona porque
// X/Wayland/PulseAudio/D-Bus usam sockets de arquivo, visíveis entre
// namespaces de rede.
func launchInNS(args []string) *exec.Cmd {
	inner := args
	if user := os.Getenv("SUDO_USER"); user != "" {
		// roda como o usuário que chamou o sudo, mantendo o ambiente gráfico
		keep := "DISPLAY,XAUTHORITY,WAYLAND_DISPLAY,XDG_RUNTIME_DIR,DBUS_SESSION_BUS_ADDRESS,PULSE_SERVER,HOME"
		inner = append([]string{"sudo", "--preserve-env=" + keep, "-u", user, "--"}, args...)
	} else {
		fmt.Fprintln(os.Stderr, "aviso: rode com sudo (não como root direto) para o app abrir como seu usuário.")
	}
	full := append([]string{"netns", "exec", nsName}, inner...)
	cmd := exec.Command("ip", full...)
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd
}

func requireRoot() {
	if os.Geteuid() != 0 {
		fatal("este modo precisa de root: rode com sudo")
	}
}

func requireTools(tools ...string) {
	var missing []string
	for _, b := range tools {
		if _, err := exec.LookPath(b); err != nil {
			missing = append(missing, b)
		}
	}
	if len(missing) > 0 {
		fatal("faltam ferramentas: " + strings.Join(missing, ", ") +
			"  (ex.: sudo apt install iproute2 nftables)")
	}
}
