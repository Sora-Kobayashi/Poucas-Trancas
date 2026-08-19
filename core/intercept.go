// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"fmt"
	"path/filepath"
	"runtime"

	"poucastrancas/core/divert"
	"poucastrancas/core/redirect"
)

// ProxyPort e a porta local onde o proxy transparente recebe o que o
// driver desviou. Fixa e alta para nao colidir com servico comum.
const ProxyPort = 9253

// InterceptStats junta o que a interface precisa mostrar.
type InterceptStats struct {
	redirect.Stats
	Served uint64 `json:"served"`
	Failed uint64 `json:"failed"`
	Direct uint64 `json:"direct"`
}

type interceptor struct {
	red   *redirect.Redirector
	proxy *redirect.Proxy
}

func divertDir() string { return filepath.Join(DataDir(), "windivert") }

// EnsureWinDivert extrai o driver embutido e carrega a DLL. Sem rede, sem
// download: os binarios vem de dentro do executavel.
func EnsureWinDivert(progress func(string)) error {
	if runtime.GOOS != "windows" {
		return divert.ErrUnsupported
	}
	if progress != nil {
		progress("extraindo WinDivert embutido...")
	}
	dllPath, err := divert.Extract(divertDir())
	if err != nil {
		return err
	}
	if progress != nil {
		progress("carregando driver...")
	}
	return divert.Load(dllPath)
}

func (m *Manager) startIntercept(dial redirect.DialFunc, log func(string), fallback, forceV4 bool) (*interceptor, error) {
	red := redirect.New(ProxyPort, ExeNames())
	red.ForceIPv4.Store(forceV4)
	proxy := redirect.NewProxy(ProxyPort, red, dial, log)
	proxy.FallbackDirect.Store(fallback)

	if err := proxy.Start(); err != nil {
		return nil, err
	}
	if err := red.Start(); err != nil {
		proxy.Stop()
		return nil, fmt.Errorf("iniciando o desvio: %w", err)
	}
	return &interceptor{red: red, proxy: proxy}, nil
}

func (i *interceptor) stop() {
	if i == nil {
		return
	}
	if i.red != nil {
		i.red.Stop()
	}
	if i.proxy != nil {
		i.proxy.Stop()
	}
}

func (i *interceptor) stats() InterceptStats {
	if i == nil {
		return InterceptStats{}
	}
	return InterceptStats{
		Stats:  i.red.Stats(),
		Served: i.proxy.Served.Load(),
		Failed: i.proxy.Failed.Load(),
		Direct: i.proxy.Direct.Load(),
	}
}
