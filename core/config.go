// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config e o que sobrevive entre execucoes.
type Config struct {
	UDPMode        string   `json:"udpMode"`
	Upstream       string   `json:"upstream"`
	SocksURL       string   `json:"socksUrl"`
	FallbackDirect bool     `json:"fallbackDirect"`
	Bridges        []string `json:"bridges"`
	AskOnClose     bool     `json:"askOnClose"`
	AutoConnect    bool     `json:"autoConnect"`
	Notify         bool     `json:"notify"`
}

func defaultConfig() Config {
	return Config{
		UDPMode:    string(UDPDirect),
		Upstream:   string(UpTor),
		AskOnClose: true,
		Notify:     true,
	}
}

var cfgMu sync.Mutex

func configPath() string { return filepath.Join(DataDir(), "config.json") }

// LoadConfig le a configuracao salva. Arquivo ausente ou corrompido volta
// ao padrao em vez de falhar.
func LoadConfig() Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	c := defaultConfig()
	b, err := os.ReadFile(configPath())
	if err != nil {
		return c
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return defaultConfig()
	}
	if c.UDPMode == "" {
		c.UDPMode = string(UDPDirect)
	}
	if c.Upstream == "" {
		c.Upstream = string(UpTor)
	}
	return c
}

func SaveConfig(c Config) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()

	if err := os.MkdirAll(DataDir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o600)
}
