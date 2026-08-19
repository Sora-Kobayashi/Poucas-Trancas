// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package divert

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed all:dist
var dist embed.FS

// Embedded diz se os binarios foram incluidos neste build.
func Embedded() bool {
	for name := range wanted {
		if _, err := readEmbedded(name); err != nil {
			return false
		}
	}
	return true
}

func Extract(destDir string) (string, error) {
	if !Embedded() {
		return "", errors.New("WinDivert nao foi embutido neste build — rode: go run ./cmd/fetchdeps")
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}

	for name, want := range wanted {
		data, err := readEmbedded(name)
		if err != nil {
			return "", err
		}
		if got := sum256(data); got != want {
			return "", fmt.Errorf("%s embutido com hash inesperado (%s)", name, got)
		}
		target := filepath.Join(destDir, name)
		if b, err := os.ReadFile(target); err == nil && sum256(b) == want {
			continue
		}
		if err := os.WriteFile(target, data, 0o700); err != nil {
			return "", fmt.Errorf("gravando %s: %w", name, err)
		}
	}
	return filepath.Join(destDir, "WinDivert.dll"), nil
}

// readEmbedded aceita o arquivo com ou sem .gz. Os binarios sao guardados
// comprimidos para nao inchar o executavel.
func readEmbedded(name string) ([]byte, error) {
	data, err := dist.ReadFile("dist/" + name + ".gz")
	if err != nil {
		return dist.ReadFile("dist/" + name)
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func sum256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
