// Poucas Trancas — Discord por rede anônima.
// Copyright 2026 Poucas Trancas. Licenciado sob a Apache License 2.0.
// Veja LICENSE e NOTICE na raiz do projeto.

package core

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Tor struct {
	Exe       string
	DataDir   string
	SocksAddr string
	CtrlAddr  string

	Bridges  []string
	Lyrebird string

	cmd  *exec.Cmd
	done chan struct{}

	mu    sync.Mutex
	warns []string
}

var bootstrapRe = regexp.MustCompile(`Bootstrapped (\d+)%`)

func NewTor(exe, dataDir string, socksPort, ctrlPort int) *Tor {
	return &Tor{
		Exe:       exe,
		DataDir:   dataDir,
		SocksAddr: fmt.Sprintf("127.0.0.1:%d", socksPort),
		CtrlAddr:  fmt.Sprintf("127.0.0.1:%d", ctrlPort),
		done:      make(chan struct{}),
	}
}

func (t *Tor) writeTorrc() (string, error) {
	if err := os.MkdirAll(t.DataDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(t.DataDir, "torrc")
	_, socksPort, _ := net.SplitHostPort(t.SocksAddr)
	_, ctrlPort, _ := net.SplitHostPort(t.CtrlAddr)

	var b strings.Builder
	fmt.Fprintf(&b, "SocksPort 127.0.0.1:%s\n", socksPort)
	fmt.Fprintf(&b, "ControlPort 127.0.0.1:%s\n", ctrlPort)
	b.WriteString("CookieAuthentication 1\n")
	fmt.Fprintf(&b, "DataDirectory %s\n", fwd(t.DataDir))
	b.WriteString("ClientOnly 1\nAvoidDiskWrites 1\nLog notice stdout\n")

	if len(t.Bridges) > 0 && t.Lyrebird != "" {
		fmt.Fprintf(&b, "ClientTransportPlugin obfs4,meek_lite,webtunnel exec %s\n", fwd(t.Lyrebird))
		b.WriteString("UseBridges 1\n")
		for _, br := range t.Bridges {
			if br = strings.TrimSpace(br); br != "" {
				fmt.Fprintf(&b, "Bridge %s\n", strings.TrimPrefix(br, "Bridge "))
			}
		}
	}

	return path, os.WriteFile(path, []byte(b.String()), 0o600)
}

func fwd(p string) string { return strings.ReplaceAll(p, `\`, `/`) }

const bootstrapTimeout = 6 * time.Minute

const stallAfter = 75 * time.Second

// Start sobe o tor e so retorna quando o bootstrap chega a 100%.
func (t *Tor) Start(progress func(pct int, msg string)) error {
	if _, err := os.Stat(t.Exe); err != nil {
		return fmt.Errorf("tor.exe nao encontrado em %s: %w", t.Exe, err)
	}
	torrc, err := t.writeTorrc()
	if err != nil {
		return fmt.Errorf("gerando torrc: %w", err)
	}

	t.cmd = hiddenCmd(t.Exe, "-f", torrc)
	t.cmd.Dir = filepath.Dir(t.Exe)
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	t.cmd.Stderr = t.cmd.Stdout

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("iniciando tor: %w", err)
	}

	ready := make(chan error, 1)
	progressed := make(chan int, 64)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if m := bootstrapRe.FindStringSubmatch(line); m != nil {
				pct, _ := strconv.Atoi(m[1])
				select {
				case progressed <- pct:
				default:
				}
				if progress != nil {
					progress(pct, line)
				}
				if pct >= 100 {
					select {
					case ready <- nil:
					default:
					}
				}
			}
			if strings.Contains(line, "[warn]") || strings.Contains(line, "[err]") {
				t.mu.Lock()
				t.warns = append(t.warns, strings.TrimSpace(line))
				if len(t.warns) > 12 {
					t.warns = t.warns[len(t.warns)-12:]
				}
				t.mu.Unlock()
			}
			if strings.Contains(line, "[err]") {
				select {
				case ready <- errors.New(t.explain(line)):
				default:
				}
			}
		}
		close(t.done)
	}()

	deadline := time.After(bootstrapTimeout)
	stall := time.NewTimer(stallAfter)
	defer stall.Stop()
	last := 0

	for {
		select {
		case err := <-ready:
			return err
		case pct := <-progressed:
			if pct > last {
				last = pct
				if !stall.Stop() {
					select {
					case <-stall.C:
					default:
					}
				}
				stall.Reset(stallAfter)
			}
		case <-stall.C:
			t.Stop()
			if len(t.Bridges) > 0 {
				return fmt.Errorf("bootstrap parou em %d%% mesmo com pontes — as pontes podem estar bloqueadas; pegue outras em https://bridges.torproject.org", last)
			}
			return fmt.Errorf("bootstrap parou em %d%% (sem avanco por %s) — sinal de rede filtrando o Tor.\n"+
				"Ative as pontes obfs4 e cole linhas de https://bridges.torproject.org", last, stallAfter)
		case <-deadline:
			t.Stop()
			return fmt.Errorf("bootstrap nao terminou em %s (parou em %d%%)", bootstrapTimeout, last)
		case <-t.done:
			return errors.New("tor encerrou antes de ficar pronto")
		}
	}
}

func (t *Tor) explain(errLine string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var ctx []string
	for _, w := range t.warns {
		if strings.Contains(w, "[warn]") {
			ctx = append(ctx, w)
		}
	}
	if len(ctx) == 0 {
		return errLine
	}
	return errLine + "\n" + strings.Join(ctx, "\n")
}

func (t *Tor) Stop() {
	if t.cmd != nil && t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
		_, _ = t.cmd.Process.Wait()
	}
}

// NewIdentity pede um circuito novo pela porta de controle — util quando
// o no de saida caiu numa lista de bloqueio.
func (t *Tor) NewIdentity() error {
	conn, err := net.DialTimeout("tcp", t.CtrlAddr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	cookie, err := t.authCookie()
	if err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	for _, cmd := range []string{"AUTHENTICATE " + cookie + "\r\n", "SIGNAL NEWNYM\r\n"} {
		if _, err := io.WriteString(conn, cmd); err != nil {
			return err
		}
		resp, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if !strings.HasPrefix(resp, "250") {
			return fmt.Errorf("controle recusou %q: %s", strings.TrimSpace(cmd), strings.TrimSpace(resp))
		}
	}
	return nil
}

func (t *Tor) authCookie() (string, error) {
	b, err := os.ReadFile(filepath.Join(t.DataDir, "control_auth_cookie"))
	if err != nil {
		return "", fmt.Errorf("lendo cookie de controle: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (t *Tor) Dial(network, addr string) (net.Conn, error) {
	if !strings.HasPrefix(network, "tcp") {
		return nil, fmt.Errorf("tor so transporta TCP, pediram %q", network)
	}
	conn, err := net.DialTimeout("tcp", t.SocksAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("SOCKS do tor inacessivel: %w", err)
	}
	if err := socks5connect(conn, addr); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func socks5connect(conn net.Conn, addr string) error {
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer conn.SetDeadline(time.Time{})

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("porta invalida %q", portStr)
	}
	if len(host) > 255 {
		return errors.New("host longo demais para SOCKS5")
	}

	if _, err := conn.Write([]byte{0x05, 0x01, 0x00}); err != nil {
		return err
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x05 || resp[1] != 0x00 {
		return fmt.Errorf("SOCKS5 recusou o handshake (%02x %02x)", resp[0], resp[1])
	}

	req := []byte{0x05, 0x01, 0x00, 0x03, byte(len(host))}
	req = append(req, host...)
	req = binary.BigEndian.AppendUint16(req, uint16(port))
	if _, err := conn.Write(req); err != nil {
		return err
	}

	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return err
	}
	if head[1] != 0x00 {
		return fmt.Errorf("SOCKS5 CONNECT falhou (codigo %d)", head[1])
	}

	var skip int
	switch head[3] {
	case 0x01:
		skip = 4
	case 0x04:
		skip = 16
	case 0x03:
		b := make([]byte, 1)
		if _, err := io.ReadFull(conn, b); err != nil {
			return err
		}
		skip = int(b[0])
	default:
		return fmt.Errorf("tipo de endereco desconhecido: %d", head[3])
	}
	_, err = io.CopyN(io.Discard, conn, int64(skip+2))
	return err
}

func (t *Tor) StartAuto(progress func(int, string)) error {
	killOrphanTor(t.Exe)
	sp, cp, err := freePortPair()
	if err != nil {
		return err
	}
	t.SocksAddr = fmt.Sprintf("127.0.0.1:%d", sp)
	t.CtrlAddr = fmt.Sprintf("127.0.0.1:%d", cp)
	return t.Start(progress)
}
