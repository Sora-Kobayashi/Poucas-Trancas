// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"fmt"
	"net"
)

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func freePortPair() (int, int, error) {
	a, err := freePort()
	if err != nil {
		return 0, 0, fmt.Errorf("sem porta livre para o SOCKS: %w", err)
	}
	for i := 0; i < 10; i++ {
		b, err := freePort()
		if err != nil {
			return 0, 0, fmt.Errorf("sem porta livre para o controle: %w", err)
		}
		if b != a {
			return a, b, nil
		}
	}
	return 0, 0, fmt.Errorf("nao consegui duas portas distintas")
}
