// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package divert

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

const (
	zipURL = "https://github.com/basil00/WinDivert/releases/download/v2.2.2/WinDivert-2.2.2-A.zip"
	zipSum = "63cb41763bb4b20f600b6de04e991a9c2be73279e317d4d82f237b150c5f3f15"
)

var wanted = map[string]string{
	"WinDivert.dll":   "c1e060ee19444a259b2162f8af0f3fe8c4428a1c6f694dce20de194ac8d7d9a2",
	"WinDivert64.sys": "8da085332782708d8767bcace5327a6ec7283c17cfb85e40b03cd2323a90ddc2",
}

// Installed diz se os dois arquivos ja estao no lugar e com hash correto.
func Installed(dir string) bool {
	for name, want := range wanted {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || sum256(b) != want {
			return false
		}
	}
	return true
}

func Install(dir string, progress func(string)) error {
	if Installed(dir) {
		if progress != nil {
			progress("WinDivert ja instalado e verificado")
		}
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	cache := filepath.Join(os.TempDir(), "WinDivert-2.2.2-A.zip")
	blob, err := os.ReadFile(cache)
	if err != nil || sum256(blob) != zipSum {
		if progress != nil {
			progress("baixando WinDivert do release oficial...")
		}
		blob, err = download(zipURL)
		if err != nil {
			return err
		}
		if got := sum256(blob); got != zipSum {
			return fmt.Errorf("sha256 do WinDivert nao confere\n  esperado %s\n  obtido   %s", zipSum, got)
		}
		_ = os.WriteFile(cache, blob, 0o600)
	}
	if progress != nil {
		progress("pacote verificado")
	}

	zr, err := zip.NewReader(bytes.NewReader(blob), int64(len(blob)))
	if err != nil {
		return err
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !contains(f.Name, "/x64/") {
			continue
		}
		name := path.Base(f.Name)
		want, ok := wanted[name]
		if !ok {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if h := sum256(data); h != want {
			return fmt.Errorf("%s com hash inesperado (%s)", name, h)
		}
		if err := writeGz(filepath.Join(dir, name+".gz"), data); err != nil {
			return fmt.Errorf("gravando %s: %w", name, err)
		}
		got[name] = true
		if progress != nil {
			progress("instalado " + name)
		}
	}
	for name := range wanted {
		if !got[name] {
			return fmt.Errorf("%s nao veio no pacote", name)
		}
	}
	return nil
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && bytes.Contains([]byte(s), []byte(sub))
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
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
