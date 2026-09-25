# Zyron - Suíte de Testes de Carga e Qualidade P2P

Esta pasta contém uma suíte de testes de estresse, concorrência e qualidade de conexão para o **Zyron**, desenvolvida para ser executada de forma **100% externa**, **sem alterar nenhuma linha de código** do servidor (`zyron-server`) ou do cliente desktop (`zyron-client`).

---

## 📐 Arquitetura dos Testes

O Zyron opera em duas camadas independentes com papéis distintos:

```text
┌────────────────────────────────────────────────────────────────────────┐
│ 1. Camada de Sinalização, Presença e Fila (Servidor Central Go)       │
│    - Conexões simultâneas WebSocket (/ws) com Ping/Pong periódico      │
│    - Fila de atendimento atômica no Redis e presença com TTL de 10s    │
│    - Auditoria de sessões encerradas no PostgreSQL                     │
│    -> Testado via: server_load.exe (Clientes e Operadores Virtuais)   │
└────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────┐
│ 2. Camada de Mídia P2P (WebRTC - Tela e Áudio)                        │
│    - Conexão direta entre estações via UDP/SRTP (rede LAN ou VPN)     │
│    - O tráfego pesado de vídeo/áudio NÃO transita pelo servidor Go    │
│    -> Testado via: p2p_benchmark.exe (Medição de RTT, Jitter e Perda) │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 📁 Arquivos da Suíte

| Arquivo | Descrição |
| :--- | :--- |
| [`server_load.go`](server_load.go) / `server_load.exe` | Injetor de carga de conexões simultâneas WebSocket, fila e sinalização |
| [`p2p_benchmark.go`](p2p_benchmark.go) / `p2p_benchmark.exe` | Medidor de qualidade de rede P2P (UDP/RTP), RTT, Jitter RFC 3550 e perda de pacotes |
| [`run_server_load.bat`](run_server_load.bat) | Script de menu rápido para testes de servidor no Windows |
| [`run_p2p_benchmark.bat`](run_p2p_benchmark.bat) | Script facilitador para testes entre máquinas no Windows |

---

## 🚀 1. Testes de Carga no Servidor (`server_load`)

O utilitário `server_load` simula um ecossistema com múltiplos **clientes virtuais** e **operadores virtuais** executando o protocolo real do Zyron:
1. Conexão WebSocket no endpoint `/ws` com controle gradual de entrada (*ramp-up*).
2. Manutenção do TTL de presença no Redis enviando batimentos de `ping` a cada 4–5 segundos.
3. Clientes solicitam atendimento (`request_support`) gerando chamados com dados de hostname e IP.
4. Operadores recebem a atualização da fila (`queue_update`) e disputam o aceite dos chamados de forma atômica (`accept_ticket`).
5. Troca sintética de sinalização WebRTC (oferta SDP, resposta SDP e candidatos ICE).
6. Encerramento da chamada (`session_closed`), acionando a gravação de auditoria no PostgreSQL.
7. Envio de avaliação do atendimento (`rate_session`).

### 1.1 Execução Rápida (Windows)

Dê um duplo-clique no arquivo [`run_server_load.bat`](run_server_load.bat) ou execute no terminal:

```cmd
run_server_load.bat

# Ou informando o IP diretamente na chamada:
run_server_load.bat 192.168.1.100:8080
```

1. **Definição do IP do Servidor:** O script solicita o endereço do servidor logo no início (ex: `localhost`, `192.168.1.100` ou `10.0.0.5:8080`). Se você apenas pressionar **ENTER**, ele assume o padrão `localhost:8080`. Se omitir a porta (ex: `192.168.1.100`), a porta padrão `:8080` é anexada automaticamente.
2. **Seleção do Perfil:** Em seguida, escolha o perfil de teste desejado para rodar contra o IP definido:
   - **[1] Carga Leve**: 50 Clientes, 2 Operadores, 30s (validação rápida).
   - **[2] Carga Média**: 250 Clientes, 10 Operadores, 60s (cenário de dia a dia corporativo).
   - **[3] Carga Alta / Stress**: 1.000 Clientes, 25 Operadores, 2 minutos (teste de concorrência e carga no Redis).
   - **[4] Personalizado**: Ajuste quantidade de clientes, operadores e tempo manualmente.

---

### 1.2 Execução via Linha de Comando (CLI)

Você pode executar o binário compilado diretamente:

```bash
# Teste padrão contra o servidor local (100 clientes, 5 operadores, 1 minuto)
./server_load.exe -server localhost:8080 -clients 100 -ops 5 -duration 1m

# Teste contra servidor remoto com 500 clientes simultâneos
./server_load.exe -server 192.168.1.100:8080 -clients 500 -ops 20 -duration 2m -ramp-up 10s

# Teste focado exclusivamente em manter presença no Redis (sem abrir chamados)
./server_load.exe -server localhost:8080 -clients 1000 -ops 10 -req-ratio 0.0 -duration 5m
```

Também é possível compilar ou rodar via código-fonte Go:

```bash
go run server_load.go -server localhost:8080 -clients 200 -ops 10 -duration 1m
```

---

### 1.3 Parâmetros Disponíveis (`server_load`)

| Flag | Padrão | Descrição |
| :--- | :--- | :--- |
| `-server` | `localhost:8080` | Endereço `host:porta` do Zyron Server |
| `-clients` | `100` | Quantidade de clientes simultâneos a serem conectados |
| `-ops` | `5` | Quantidade de operadores de suporte simultâneos |
| `-duration`| `1m0s` | Duração do teste (ex: `30s`, `2m`, `10m`) |
| `-req-ratio`| `0.6` | Proporção de clientes que abrirão chamado na fila (ex: `0.6` = 60%) |
| `-ramp-up` | `5s` | Tempo para diluir a entrada das conexões (evita sobrecarga de porta local) |
| `-ping-interval` | `5s` | Intervalo do ping de presença |
| `-simulate-signaling`| `true` | Simula fluxo completo (oferta, resposta SDP e encerramento auditado no Postgres) |
| `-rate-sessions` | `true` | Envia nota e feedback de satisfação ao encerrar cada chamada |

---

### 1.4 O que monitorar no Servidor durante o teste

Durante a execução do teste, abra os seguintes canais de monitoramento:

1. **Painel Web do Gerenciador:**
   Acesse no navegador: `http://localhost:8080/manager`  
   Acesse a seção de **Tempo Real** para ver:
   - Conexões ativas no Hub
   - Tamanho da fila dinâmica no Redis
   - Operadores disponíveis vs ocupados
   - Duração das chamadas em andamento

2. **Consumo de Recursos dos Containers Docker:**
   ```bash
   docker stats zyron_api zyron_redis zyron_db
   ```
   - O uso de CPU e memória do `zyron_api` deve permanecer estável.
   - O `zyron_redis` deve manter operações por segundo altas com consumo de memória baixo (kB/MB).
   - O `zyron_db` deve receber os `INSERT` das sessões fechadas sem travar conexões.

3. **Logs do Servidor em Tempo Real:**
   ```bash
   docker compose logs -f --tail=50 zyron_api
   ```

---

## 🌐 2. Testes de Qualidade de Conexão Entre Clientes (P2P)

Como o tráfego de tela e voz do WebRTC trafega ponto a ponto diretamente entre as máquinas via **UDP**, a qualidade do atendimento remoto depende da estabilidade, latência e ausência de perda de pacotes na rota de rede entre o Cliente e o Operador.

O utilitário `p2p_benchmark` simula pacotes RTP reais de 1200 bytes com carimbo de data/hora de alta precisão e calcula:
- **RTT (Round-Trip Time)**: Latência de ida e volta (Mínima, Média, Máxima, p95 e p99).
- **Jitter Estatístico (RFC 3550)**: A mesma métrica padrão utilizada internacionalmente pelo protocolo RTP/WebRTC para medir a estabilidade do áudio.
- **Perda de Pacotes (% e total)**: Indicador de congelamento de tela e engasgos de áudio.
- **Throughput Efetivo**: Taxa real de transferência mantida em Kbps.

---

### 2.1 Como Executar o Teste P2P entre duas máquinas

#### Passo 1: Na Máquina do OPERADOR (Receptor / Servidor UDP)
Execute o script [`run_p2p_benchmark.bat`](run_p2p_benchmark.bat) e escolha a opção `[1]`, ou execute via CLI:

```bash
./p2p_benchmark.exe -mode server -port 19800
```
> O servidor abrirá a porta UDP 19800 e ficará em modo *echo*, refletindo os pacotes instantaneamente com o carimbo de resposta.

#### Passo 2: Na Máquina do CLIENTE (Emissor)
Execute o script [`run_p2p_benchmark.bat`](run_p2p_benchmark.bat), selecione a opção `[2]` e informe o IP da máquina do Operador, ou via CLI:

```bash
# Teste de 30 segundos simulando fluxo de tela (~600 Kbps)
./p2p_benchmark.exe -mode client -target 192.168.1.150 -port 19800 -rate 60 -size 1200 -duration 30s

# Teste de alta definição (simulando 60 FPS / ~1.2 Mbps)
./p2p_benchmark.exe -mode client -target 192.168.1.150 -port 19800 -rate 120 -size 1200 -duration 30s
```

Ao final, um relatório completo com o **Diagnóstico de Qualidade** será gerado automaticamente no terminal.

---

### 2.2 Simulação de Redes Degradadas com Clumsy (Windows)

Para testar a resiliência do WebRTC sob condições de redes corporativas ruins, Wi-Fi instável ou links de VPN sem alterar o código:

1. Baixe o utilitário portátil e open-source **[Clumsy](https://jagt.github.io/clumsy/)**.
2. Abra o Clumsy como Administrador.
3. No campo de filtro, digite:
   ```text
   udp or tcp port 8080
   ```
4. Ative os testes de estresse:
   - **Lag (Atraso)**: Adicione `100ms` a `200ms` (avalia o atraso do mouse e voz).
   - **Drop (Perda de Pacote)**: Ative `2%` a `5%` (avalia recuperação de quadros no vídeo).
   - **Jitter**: Ative variação de `20ms` a `40ms` (avalia a nitidez do codec de áudio Opus).
   - **Throttle**: Limite a vazão para forçar a adaptação de bitrate da tela.
5. Inicie um atendimento real ou execute o `p2p_benchmark.exe` com o Clumsy ativo para ver o impacto exato nas métricas.

---

### 2.3 Métricas Nativas do Chromium / WebView2

Durante um atendimento com o `zyron-client` ativo:
- Abra qualquer navegador baseado em Chromium (Google Chrome ou Microsoft Edge).
- Acesse o endereço interno:
  ```text
  chrome://webrtc-internals   (no Google Chrome)
  edge://webrtc-internals     (no Microsoft Edge)
  ```
- Nessa interface você poderá visualizar:
  - Gráficos de `packetsLost` e `fractionLost`.
  - Curva de `roundTripTime` da conexão P2P.
  - Taxa de quadros transmitidos (`framesPerSecond`) e resolução ativa.
  - Taxa de bits de áudio e cancelamento de eco.

---

## 📊 3. Tabela de Referência para Diagnóstico de Qualidade

Utilize os parâmetros abaixo como critério de aceitação nos testes:

| Métrica | Excelente | Aceitável | Crítico / Reprovado |
| :--- | :--- | :--- | :--- |
| **Handshake WebSocket** | < 30 ms | < 150 ms | > 500 ms (Sobrecarga de CPU na API) |
| **Ping/Pong Heartbeat** | < 15 ms | < 80 ms | > 200 ms (Risco de expiração de TTL no Redis) |
| **Latência RTT P2P** | < 40 ms | < 120 ms | > 250 ms (Atraso de mouse e voz perceptível) |
| **Jitter UDP (RFC 3550)** | < 10 ms | < 30 ms | > 50 ms (Voz robotizada e picotada) |
| **Perda de Pacotes UDP** | 0.0% | < 1.5% | > 4% (Congelamento de tela e perda de sílabas) |
| **Quedas Inesperadas de WS** | 0 | < 0.5% | > 2% (Exaustão de descritores de arquivo ou RAM) |

---

## ⚙️ 4. Recomendações para Testes em Altíssima Escala (2.000+ Conexões)

Ao realizar testes com milhares de conexões a partir de uma única máquina:
- **Portas Efêmeras no Windows:** O Windows possui um limite padrão de portas TCP efêmeras. Para testes acima de 2.000 conexões de um mesmo host emissor, aumente o intervalo executando no PowerShell (como Administrador):
  ```powershell
  netsh int ipv4 set dynamicport tcp start=10000 num=55000
  ```
- **Parâmetro `-ramp-up`:** Utilize sempre um valor adequado de `-ramp-up` (ex: `-ramp-up 10s` ou `15s`) para que o sistema operacional consiga negociar as portas TCP sem esgotar a fila de backlog do kernel.
