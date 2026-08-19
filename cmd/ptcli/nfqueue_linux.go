//go:build linux

// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Sora-Kobayashi. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package main

import "fmt"

// runNFQueue é o modo "gêmeo do WinDivert": o pacote sobe pra userspace, é
// reescrito e devolvido, com uma tabela NAT igual à do Windows.
//
// # POR QUE ELE NÃO ESTÁ LIGADO AINDA
//
// Reescrever pacote em userspace pode falhar ABERTA: um erro no caminho de
// volta faz o tráfego sair sem passar pelo Tor, parecendo que funciona. Num
// app de anonimato isso é o pior tipo de bug — vaza calado.
//
// O modo netns, ao contrário, falha FECHADA: se o roteamento não subir, a
// namespace fica sem rota e nada sai. Por isso ele é o recomendado.
//
// Este modo será ligado quando: (1) tiver a dependência de NFQUEUE em Go
// puro (florianl/go-nfqueue, sem cgo), (2) o veredito padrão for DROP
// (falha fechada) e (3) tiver sido testado em hardware real confirmando que
// não vaza. Enquanto isso, ele recusa em vez de rodar meio pronto.
func runNFQueue(args []string) {
	fmt.Println("modo nfqueue: ainda não habilitado — por segurança.")
	fmt.Println()
	fmt.Println("Reescrever pacotes em userspace pode falhar ABERTA (vazar tráfego")
	fmt.Println("sem passar pelo Tor). Num app de anonimato isso é grave, então")
	fmt.Println("prefiro não entregar meio pronto e sem teste em Linux real.")
	fmt.Println()
	fmt.Println("Use  -mode netns  — mesmo efeito, e falha FECHADA (não vaza).")
	fmt.Println("Detalhes e progresso: " + repoURL)
}
