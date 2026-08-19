// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// Package torbin carrega o Tor dentro do proprio executavel.
//
// O usuario nao instala Tor, nao baixa nada e nao precisa do Tor Browser:
// os binarios ficam embutidos via go:embed e sao extraidos para o cache
// local na primeira execucao.
//
// Por que extrair em vez de rodar da memoria: o Windows exige um arquivo
// real no disco para CreateProcess, e o tor precisa de DataDirectory
// gravavel de qualquer forma. A extracao e idempotente — se o hash bate,
// nao reescreve.
//
// PARA POPULAR OS BINARIOS (uma vez, antes do build):
//
//	go run ./cmd/fetchdeps
//
// Isso baixa o Tor Expert Bundle oficial e preenche torbin/dist/. Sem esse
// passo o pacote compila mesmo assim, mas Available() devolve false e a
// aplicacao avisa em vez de quebrar.
package torbin

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Available diz se ha binario embutido de verdade.
func Available() bool {
	entries, err := fs.ReadDir(dist, "dist")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tor.exe") {
			return true
		}
	}
	return false
}

// Extract materializa os binarios em destDir e devolve o caminho do
// tor.exe. Reescreve so o que mudou.
func Extract(destDir string) (string, error) {
	if !Available() {
		return "", errors.New("nenhum binario do Tor embutido — rode: go run ./cmd/fetchdeps")
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", err
	}

	err := fs.WalkDir(dist, "dist", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(p), ".gz")
		if name == ".gitkeep" {
			return nil
		}
		data, err := readEmbedded(p)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, name)
		if same, _ := sameContent(target, data); same {
			return nil
		}
		return os.WriteFile(target, data, 0o700)
	})
	if err != nil {
		return "", err
	}

	torExe := filepath.Join(destDir, "tor.exe")
	if _, err := os.Stat(torExe); err != nil {
		return "", fmt.Errorf("tor.exe nao apareceu em %s: %w", destDir, err)
	}
	return torExe, nil
}

// readEmbedded devolve o conteudo, descomprimindo se o arquivo terminar em
// .gz. Os binarios sao guardados comprimidos: economizam 63% no executavel
// e a descompressao acontece uma vez, na primeira execucao.
func readEmbedded(name string) ([]byte, error) {
	data, err := dist.ReadFile(name)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(name, ".gz") {
		return data, nil
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func sameContent(path string, want []byte) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	w := sha256.Sum256(want)
	return hex.EncodeToString(h.Sum(nil)) == hex.EncodeToString(w[:]), nil
}
