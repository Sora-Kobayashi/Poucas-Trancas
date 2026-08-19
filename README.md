# Poucas Trancas

Faz o Discord sair por uma **rede anônima**, sem gambiarra, sem VPN e sem
proxy de estranho.

O nome é honesto: *poucas trancas*. A ferramenta não promete blindagem —
ela troca por onde o seu tráfego sai e diz, o tempo todo, o que continua
exposto.

---

## Por que existe

As soluções comuns para mudar por onde o Discord sai têm todas o mesmo
problema: você troca um risco por outro.

- **VPN grátis / proxy público** — você passa *todo* o seu tráfego por um
  servidor de um desconhecido, que vê seus destinos, pode registrar e pode
  vender. Trocou o seu provedor por um estranho ainda pior.
- **VPN paga** — melhor, mas ainda é uma empresa que sabe quem você é
  (pagou com cartão) e vê tudo que você acessa. Confiança concentrada em um
  ponto.
- **DLL injetada no Discord** — mexe dentro de um processo que guarda seu
  token, some a cada atualização e não é auditável.
- **Mudar região na mão, mexer em firewall, forçar rota** — gambiarra que
  quebra no próximo update e some com voz e tela sem avisar.

Poucas Trancas evita os quatro. O tráfego sai pela **rede Tor** — três
saltos, cada nó sabendo só o anterior e o próximo, nenhum deles sabendo
quem você é *e* para onde vai ao mesmo tempo. Ninguém no caminho tem o
quadro completo, e você não precisa confiar em nenhum operador específico.

E o Discord não é tocado: nada é injetado, nenhum arquivo dele é alterado,
o desvio sobrevive às atualizações.

---

## Como funciona

Um driver de rede (WinDivert) intercepta o tráfego TCP **por processo**: ele
diz qual programa abriu cada conexão, e só as do Discord são desviadas para
um proxy local, que refaz a discagem pela rede anônima.

```
Discord ──TCP──▶ desvio por PID ──▶ proxy local ──▶ Tor ──▶ internet
                 (WinDivert)                         3 saltos
```

Tor e WinDivert vão **dentro do executável**. Um arquivo, nada para
instalar, nada para baixar, funciona sem rede na primeira execução.

Prefere a sua própria saída em vez do Tor? Aponte para um **SOCKS5 seu** — a
mesma arquitetura, com o transporte que você controla.

---

## O limite, dito na cara

**Tor não transporta UDP.** É projeto da rede, não defeito da ferramenta.
Voz e compartilhamento de tela do Discord são UDP e continuam saindo
direto, expondo seu IP real ao servidor de mídia.

| Modo | Voz e tela | IP real |
|---|---|---|
| **Deixar passar** | funcionam | exposto ao servidor de mídia |
| **Bloquear** | param de funcionar | não vaza |

Não há terceira opção com Tor. A interface mostra qual modo está valendo o
tempo todo, e **conta cada conexão que escapa da rota anônima** — o app te
avisa em vez de deixar você descobrir sozinho.

Quem quiser voz *e* IP estrangeiro precisa de um transporte que carregue
UDP (WireGuard) — e para isso o campo SOCKS5 aceita apontar para ele.

---

## Uso

Baixe o `.exe` e **execute como Administrador** — carregar o driver de rede
exige isso (o Windows pede pelo UAC no primeiro clique).

1. Escolha a saída: **Tor** ou um **SOCKS5 próprio**
2. Decida o que fazer com voz e tela
3. **Conectar** e espere a barra chegar a 100%
4. **Reiniciar** o Discord na lista de clientes

O desvio só pega conexões novas, por isso o reinício.

Fechar a janela pergunta se você quer mandar para a **bandeja** (o túnel
continua) ou sair. As preferências ficam em
`%LOCALAPPDATA%\poucastrancas\config.json`.

### Se a rede bloquear o Tor

Barra travada em `loading_descriptors` = sua rede filtra o Tor. Pegue
pontes em [bridges.torproject.org](https://bridges.torproject.org), cole no
campo de pontes e reconecte. O obfs4proxy já vai embutido.

### Liberar o compartilhamento de tela

Se o compartilhamento de tela aparecer como **Not Eligible** / bloqueado, dá
para liberar por um experiment do Discord:

1. Instale o **Vencord** — repositório oficial
   [github.com/Vendicated/Vencord](https://github.com/Vendicated/Vencord) ou
   site [vencord.dev](https://vencord.dev). (Não achou? procura "Vencord" no
   Google e cai num dos dois.)
2. Nas configurações **do próprio Discord**, vá na seção do **Vencord** →
   **Plugins** e ative o **Experiments**.
3. **Reinicie o cliente do Discord.**
4. Volte nas configurações do Discord e procure a seção **Experiments**.
5. Ache o **`2026-08-video-guard`** — ou clique direto em
   `dev://experiment/2026-08-video-guard` — e deixe em **Not Eligible**.

> Isso mexe em flags internas do Discord via Vencord — contraria o ToS dele,
> igual ao aviso lá embaixo. Use por sua conta.

---

## Privacidade, sem letra miúda

- **A ferramenta não escreve log de diagnóstico.** O build de release não
  grava histórico do que você fez. O que fica em disco é só o necessário
  para funcionar: sua configuração e os binários do Tor/WinDivert.
- **Sem telemetria, sem "casa" para ligar.** Não há servidor deste projeto.
  O tráfego vai para a rede Tor pública, e nada volta para ninguém.
- **O código está todo aqui.** Não confie na palavra do README — leia.

---

## Detalhes técnicos

<details>
<summary>Como o desvio é feito</summary>

A camada `SOCKET` do WinDivert informa qual PID abriu cada porta local. A
camada `NETWORK` entrega os pacotes desses fluxos, reescritos para
`127.0.0.1:<proxy>`, guardando o destino original numa tabela indexada por
`(porta, família)`. O proxy consulta a tabela e disca pela saída escolhida.
Na volta, a reescrita é desfeita. O índice da interface é preservado e
restaurado junto — um pacote com IPs reais entregue com índice de loopback
é descartado pela pilha do Windows sem erro nenhum.
</details>

<details>
<summary>Por que IPv4 é forçado com Tor</summary>

Pouquíssimos nós de saída do Tor têm IPv6, então um destino v6 falha mesmo
em porta permitida. O SYN IPv6 dos alvos é descartado e o cliente cai em
IPv4 pelo próprio Happy Eyeballs — e por IPv4 as portas de mídia do Discord
(2082–2096) estão na política de saída padrão do Tor. Com SOCKS5 próprio
isso fica desligado.
</details>

<details>
<summary>Sem framework de UI</summary>

A janela é Win32 puro, desenhada em GDI. Sem Electron, sem WebView, sem
Wails. A única dependência é `golang.org/x/sys`, biblioteca oficial do Go. O
executável não exige runtime nenhum instalado na máquina.
</details>

---

## Compilar

Requer [Go](https://go.dev) 1.26+. Sem cgo, sem toolchain C.

```bash
go run ./cmd/fetchdeps    # baixa Tor + WinDivert e popula os embeds
go build -trimpath -ldflags="-H windowsgui -s -w -buildid=" -o poucas-trancas.exe .
```

O `fetchdeps` aceita `-sha256 <hash>` para conferir o Tor Expert Bundle
contra o hash publicado em torproject.org. Sem ele o download é aceito só
com a garantia do HTTPS — em uma ferramenta de anonimato, vale conferir. Os
hashes do WinDivert são fixos no código: se o release for republicado, o
download para em vez de aceitar binário diferente.

```
core/divert/     ligação com o WinDivert, sem cgo
core/redirect/   desvio por PID, tabela NAT, proxy transparente
core/tor.go      cliente Tor gerenciado, SOCKS5 próprio
core/ui/         janela Win32 pura, desenhada em GDI
cmd/fetchdeps/   popula os binários embutidos (só no build)
```

---

## Avisos

Modificar o comportamento de rede do Discord contraria os Termos de Serviço
dele. A ferramenta não altera o cliente nem automatiza nada dentro dele, mas
a decisão de usar é sua.

O Tor protege *quem* você é, não *o que* você faz. Sua conta continua sendo
sua conta.

Projeto pessoal, sem auditoria externa. O código está aberto para ser lido.

**A partir do momento em que você baixa e usa, é por sua conta e risco.** A
ferramenta é fornecida como está, sem garantia de nenhum tipo (ver seções 7
e 8 da licença). O que você faz com ela, e as consequências disso, são sua
responsabilidade — não do autor.

Boa parte do código foi feita com a IA **Claude**, da Anthropic. Se você não
curte isso, pouco me importa — usa outra solução, ou faz um fork.

## Licença

**Apache 2.0** — veja [LICENSE](LICENSE).

Você pode usar, modificar, redistribuir e até vender. O que a licença
**obriga**: manter o arquivo [NOTICE](NOTICE) com o crédito ao autor em
qualquer fork ou redistribuição (seção 4). Quem publicar sem o crédito está
violando a licença.

Inclui binários de terceiros redistribuídos sem modificação:
[Tor](https://www.torproject.org) (BSD-3-Clause) e
[WinDivert](https://github.com/basil00/WinDivert) (LGPLv3) — que mantêm as
próprias licenças.
