// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// Poucas Trancas — manda o Discord pelo Tor sem tocar no Discord.
//
// # COMO FUNCIONA
//
// O driver WinDivert intercepta o trafego TCP por PID: a camada SOCKET diz
// qual processo abriu cada porta, a camada NETWORK entrega os pacotes desses
// fluxos para reescrever. Os pacotes do Discord sao redirecionados para um
// proxy local, que refaz a discagem pelo Tor.
//
// Nada e injetado no Discord, nenhum arquivo dele e alterado, e o desvio
// sobrevive as atualizacoes automaticas do cliente.
//
// Tor e WinDivert vao embutidos no executavel. A janela e Win32 puro: sem
// framework, sem WebView, sem dependencia externa alem da x/sys oficial.
//
// # O QUE ELA NAO FAZ
//
// Tor nao transporta UDP — e projeto da rede, nao limitacao daqui. Voz e
// compartilhamento de tela do Discord sao UDP e continuam saindo direto,
// expondo o IP real ao servidor de midia. O modo "bloquear" impede esse
// vazamento ao custo de voz e tela pararem. A escolha e do usuario; a
// interface diz qual esta valendo o tempo todo.
package main

import "poucastrancas/core"

func main() {
	app := NewApp()
	if err := app.Run(); err != nil {
		core.Logf("saida com erro: %v", err)
	}
}
