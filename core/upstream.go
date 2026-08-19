// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"fmt"
	"net"
	"strings"
	"time"

	"poucastrancas/core/redirect"
)

type Upstream string

const (
	UpTor   Upstream = "tor"
	UpSocks Upstream = "socks"
)

func socksDialer(addr string) redirect.DialFunc {
	return func(network, target string) (net.Conn, error) {
		if !strings.HasPrefix(network, "tcp") {
			return nil, fmt.Errorf("SOCKS5 so transporta TCP")
		}
		c, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("SOCKS5 %s inacessivel: %w", addr, err)
		}
		if err := socks5connect(c, target); err != nil {
			c.Close()
			return nil, err
		}
		return c, nil
	}
}
