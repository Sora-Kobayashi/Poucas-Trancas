// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

// fetchdeps baixa o Tor Expert Bundle oficial e popula torbin/dist/, para
// que o go:embed coloque o Tor dentro do executavel final.
//
// Roda uma vez, na maquina de build — o usuario final nunca executa isso.
//
//	go run ./cmd/fetchdeps                 # versao padrao
//	go run ./cmd/fetchdeps -version 14.5.6
//	go run ./cmd/fetchdeps -sha256 <hash>  # confere a integridade
//
// Pega o tor.exe, as DLLs ao lado dele e o lyrebird.exe (obfs4proxy), que
// e o que permite usar pontes quando a rede filtra o Tor. Os geoip (20 MB
// somados) ficam de fora: sao opcionais e so servem para estatistica por
// pais, nao para conectar.
package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"poucastrancas/core/divert"
)

const urlTemplate = "https://archive.torproject.org/tor-package-archive/torbrowser/%s/tor-expert-bundle-windows-x86_64-%s.tar.gz"

// torSHA256 e o hash OFICIAL do bundle da versao padrao, conferido contra o
// sha256sums-signed-build.txt do torproject. Fixado para que o build (e o
// CI) so aceitem o binario legitimo — se o torproject republicar ou o
// download vier adulterado, o fetchdeps para em vez de embutir outra coisa.
// Ao trocar -version, passe tambem o -sha256 correspondente.
const torSHA256 = "4b225152ee8c79de6fe2e0015f82a2e7a4909390f02e2f6f41ac96bbcf779421"

func wanted(name string) bool {
	base := path.Base(name)
	if !strings.HasPrefix(name, "tor/") {
		return false
	}
	if base == "lyrebird.exe" {
		return true
	}
	if strings.Contains(name, "pluggable_transports") {
		return false
	}
	return base == "tor.exe" || strings.HasSuffix(base, ".dll")
}

func main() {
	version := flag.String("version", "14.5.6", "versao do Tor Expert Bundle")
	wantSum := flag.String("sha256", torSHA256, "sha256 esperado do .tar.gz")
	out := flag.String("out", "", "pasta de destino (padrao: torbin/dist junto ao modulo)")
	flag.Parse()

	dest := *out
	if dest == "" {
		wd, err := os.Getwd()
		if err != nil {
			fail(err)
		}
		dest = filepath.Join(wd, "torbin", "dist")
	}

	url := fmt.Sprintf(urlTemplate, *version, *version)
	fmt.Printf("Baixando %s\n", url)

	blob, err := download(url)
	if err != nil {
		fail(fmt.Errorf("download: %w", err))
	}
	sum := sha256.Sum256(blob)
	got := hex.EncodeToString(sum[:])
	fmt.Printf("sha256: %s  (%.1f MiB)\n", got, float64(len(blob))/(1<<20))

	switch {
	case *wantSum == "":
		fmt.Println("AVISO: -sha256 vazio (trocou de versao?). Sem verificacao de integridade.")
		fmt.Println("       Pegue o hash em https://www.torproject.org/download/tor/")
	case !strings.EqualFold(got, *wantSum):
		fail(fmt.Errorf("sha256 nao confere — download recusado:\n  esperado %s\n  obtido   %s", *wantSum, got))
	default:
		fmt.Println("integridade do Tor verificada contra o hash oficial")
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		fail(err)
	}
	n, err := extract(blob, dest)
	if err != nil {
		fail(fmt.Errorf("extraindo: %w", err))
	}
	if n == 0 {
		fail(fmt.Errorf("nenhum arquivo util no bundle — o layout mudou?"))
	}
	fmt.Printf("\n%d arquivo(s) do Tor em %s\n", n, dest)

	dvDest := filepath.Join(filepath.Dir(filepath.Dir(dest)), "core", "divert", "dist")
	fmt.Printf("\nWinDivert -> %s\n", dvDest)
	if err := divert.Install(dvDest, func(m string) { fmt.Println("  " + m) }); err != nil {
		fail(fmt.Errorf("WinDivert: %w", err))
	}

	fmt.Println("\nAgora: go build ./...")
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extract(blob []byte, dest string) (int, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(blob)))
	if err != nil {
		return 0, err
	}
	defer gz.Close()

	count := 0
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		if hdr.Typeflag != tar.TypeReg || !wanted(hdr.Name) {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return count, err
		}
		target := filepath.Join(dest, path.Base(hdr.Name)+".gz")
		if err := writeGz(target, data); err != nil {
			return count, err
		}
		fmt.Printf("  %-24s %6.1f MiB\n", path.Base(hdr.Name), float64(len(data))/(1<<20))
		count++
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "erro:", err)
	os.Exit(1)
}
