**English** · [Português](README.pt-BR.md)

# Poucas Trancas

Routes Discord through an **anonymity network** — no hacks, no VPN, no
stranger's proxy.

The name is honest: *poucas trancas* ("few locks", in Portuguese). This tool
doesn't promise armor — it changes where your traffic exits and tells you,
at all times, what's still exposed.

---

## Why it exists

The usual ways to change where Discord exits all share one problem: you
trade one risk for another.

- **Free VPN / public proxy** — you route *all* your traffic through a
  stranger's server, which sees your destinations, can log them, and can
  sell them. You swapped your ISP for someone even worse.
- **Paid VPN** — better, but still a company that knows who you are (you
  paid with a card) and sees everything you access. Trust concentrated in
  one point.
- **DLL injected into Discord** — runs inside a process that holds your
  token, breaks on every update, and isn't auditable.
- **Manually changing region, firewall tricks, forcing routes** — hacks
  that break on the next update and kill voice/screen without warning.

Poucas Trancas avoids all four. Traffic exits through the **Tor network** —
three hops, each node knowing only the previous and next one, none of them
knowing who you are *and* where you're going at the same time. Nobody on the
path has the full picture, and you don't have to trust any single operator.

And Discord itself is untouched: nothing is injected, no file of it is
altered, and the redirect survives updates.

---

## How it works

A network driver (WinDivert) intercepts TCP traffic **per process**: it
tells which program opened each connection, and only Discord's are diverted
to a local proxy, which re-dials through the anonymity network.

```
Discord ──TCP──▶ per-PID divert ──▶ local proxy ──▶ Tor ──▶ internet
                 (WinDivert)                         3 hops
```

Tor and WinDivert ship **inside the executable**. One file, nothing to
install, nothing to download, works offline on first run.

Prefer your own exit instead of Tor? Point it at your own **SOCKS5** — same
architecture, with a transport you control.

---

## The limit, stated plainly

**Tor doesn't carry UDP.** That's a design property of the network, not a
flaw in this tool. Discord's voice and screen sharing are UDP and keep going
out directly, exposing your real IP to the media server.

| Mode | Voice & screen | Real IP |
|---|---|---|
| **Let through** | work | exposed to the media server |
| **Block** | stop working | not leaked |

There's no third option with Tor. The UI shows which mode is active at all
times, and **counts every connection that escapes the anonymous route** —
the app warns you instead of letting you find out on your own.

If you want voice *and* a foreign IP, you need a transport that carries UDP
(WireGuard) — and for that the SOCKS5 field lets you point at it.

---

## Usage

Download the `.exe` and **run as Administrator** — loading the network
driver requires it (Windows asks via UAC on the first click).

1. Choose the exit: **Tor** or your own **SOCKS5**
2. Decide what to do with voice & screen
3. **Connect** and wait for the bar to reach 100%
4. **Restart** Discord from the client list

The divert only catches new connections, hence the restart.

Closing the window asks whether to send it to the **tray** (the tunnel stays
up) or quit. Preferences live in
`%LOCALAPPDATA%\poucastrancas\config.json`.

### If the network blocks Tor

Bar stuck at `loading_descriptors` = your network is filtering Tor. Get
bridges at [bridges.torproject.org](https://bridges.torproject.org), paste
them in the bridges field, and reconnect. obfs4proxy is already embedded.

### Unlocking screen sharing

If screen sharing shows up as **Not Eligible** / blocked, you can unlock it
through a Discord experiment:

1. Install **Vencord** — official repo
   [github.com/Vendicated/Vencord](https://github.com/Vendicated/Vencord) or
   site [vencord.dev](https://vencord.dev). (Can't find it? search "Vencord"
   on Google and you'll land on one of the two.)
2. In **Discord's own settings**, go to the **Vencord** section →
   **Plugins** and enable **Experiments**.
3. **Restart the Discord client.**
4. Back in Discord's settings, find the **Experiments** section.
5. Find **`2026-08-video-guard`** — or click
   `dev://experiment/2026-08-video-guard` directly — and set it to
   **Not Eligible**.

> This touches Discord's internal flags via Vencord — against its ToS, same
> as the disclaimer below. Use at your own risk.

---

## Privacy, no fine print

- **The tool writes no diagnostic log.** The release build keeps no history
  of what you did. What lands on disk is only what's needed to run: your
  config and the Tor/WinDivert binaries.
- **No telemetry, no "phone home".** There is no server for this project.
  Traffic goes to the public Tor network, and nothing comes back to anyone.
- **The code is all here.** Don't take the README's word — read it.

---

## Technical notes

<details>
<summary>How the divert is done</summary>

WinDivert's `SOCKET` layer reports which PID opened each local port. The
`NETWORK` layer hands over the packets of those flows, rewritten to
`127.0.0.1:<proxy>`, storing the original destination in a table keyed by
`(port, family)`. The proxy looks up the table and dials through the chosen
exit. On the way back, the rewrite is undone. The interface index is
preserved and restored along with it — a packet with real IPs delivered with
a loopback index is dropped by the Windows stack with no error at all.
</details>

<details>
<summary>Why IPv4 is forced with Tor</summary>

Very few Tor exit nodes have IPv6, so a v6 destination fails even on an
allowed port. The targets' IPv6 SYN is dropped and the client falls back to
IPv4 via its own Happy Eyeballs — and over IPv4 Discord's media ports
(2082–2096) are in Tor's default exit policy. With your own SOCKS5 this is
turned off.
</details>

<details>
<summary>No UI framework</summary>

The window is pure Win32, drawn in GDI. No Electron, no WebView, no Wails.
The only dependency is `golang.org/x/sys`, Go's official library. The
executable requires no runtime installed on the machine.
</details>

---

## Building

Requires [Go](https://go.dev) 1.26+. No cgo, no C toolchain.

```bash
go run ./cmd/fetchdeps    # downloads Tor + WinDivert and populates the embeds
go build -trimpath -ldflags="-H windowsgui -s -w -buildid=" -o poucas-trancas.exe .
```

`fetchdeps` pins the official SHA-256 of the Tor Expert Bundle (verified
against torproject.org): the build and CI refuse any different binary. The
WinDivert hashes are fixed in code — if the release is republished, the
download stops instead of accepting a different binary.

```
core/divert/     WinDivert binding, no cgo
core/redirect/   per-PID divert, NAT table, transparent proxy
core/tor.go      managed Tor client, own SOCKS5
core/ui/         pure Win32 window, drawn in GDI
cmd/fetchdeps/   populates the embedded binaries (build only)
```

---

## Disclaimers

Modifying Discord's network behavior goes against its Terms of Service. This
tool doesn't alter the client or automate anything inside it, but the choice
to use it is yours.

Tor protects *who* you are, not *what* you do. Your account is still your
account.

Personal project, no external audit. The code is open to be read.

**The moment you download and use it, it's at your own risk.** The tool is
provided as is, with no warranty of any kind (see sections 7 and 8 of the
license). What you do with it, and the consequences, are your
responsibility — not the author's.

Much of the code was written with the help of the AI **Claude**, by
Anthropic. If you don't like that, I couldn't care less — use another tool,
or fork it.

## License

**Apache 2.0** — see [LICENSE](LICENSE).

You may use, modify, redistribute, and even sell it. What the license
**requires**: keep the [NOTICE](NOTICE) file with credit to the author in
any fork or redistribution (section 4). Publishing without the credit
violates the license.

Includes third-party binaries redistributed unmodified:
[Tor](https://www.torproject.org) (BSD-3-Clause) and
[WinDivert](https://github.com/basil00/WinDivert) (LGPLv3) — which keep their
own licenses.
