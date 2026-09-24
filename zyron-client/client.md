# 📱 Especificação Técnica do ClientApp (Zyron Client)
## Aplicação Desktop P2P em Go com Wails (v2)

Este documento descreve as diretrizes arquiteturais, fluxos e especificações para o desenvolvimento do **ClientApp (Zyron Client)** — aplicativo desktop corporativo construído com **Golang** e o framework **Wails v2** ([wails.io](https://wails.io)).

---

## 1. Visão Geral e Princípios Arquiteturais

O ClientApp é um binário único executado nas estações Windows de clientes e operadores.

```
       ┌────────────────────────────────────────────────────────┐
       │               Servidor Central (zyron-server)          │
       │       - Autenticação / Perfil (REST)                   │
       │       - Sinalização WebRTC SDP/ICE (WebSocket)         │
       │       - Fila de Espera em Memória (Redis)              │
       │       - Auditoria de Atendimentos (PostgreSQL)         │
       └──────────────┬──────────────────────────┬──────────────┘
         WebSocket/   │                          │  WebSocket/
         Sinalização  │                          │  Sinalização
                      ▼                          ▼
       ┌────────────────────────┐      ┌────────────────────────┐
       │   ClientApp (Usuário)  │      │  ClientApp (Operador)  │
       │   - Autenticação whoami│      │   - Autenticação whoami│
       │   - Compartilha Tela   │      │   - Visualiza Fila     │
       │   - Áudio / Voz (Mic)  │      │   - Recebe Tela Remota │
       └───────────┬────────────┘      └───────────▲────────────┘
                   │                               │
                   │   CONEXÃO DIRETA P2P (LAN)    │
                   └───► Vídeo (Tela) WebRTC ──────┘
                   ◄───► Voz Bidirecional WebRTC ──►
```

### Regras Críticas:
1. **P2P LAN**: O tráfego de **áudio (voz)** e **vídeo (compartilhamento de tela)** transita **diretamente entre as máquinas** na rede interna via WebRTC. O servidor em Go **nunca** recebe pacotes de mídia.
2. **Login Transparente**: O usuário não digita login ou senha. A aplicação detecta a identidade automaticamente através do comando/usuário do sistema operacional (`whoami`).
3. **Pilha Tecnológica**:
   - **Backend**: Go 1.22+ com Wails v2.
   - **Frontend**: Vue.js 3, React ou Svelte com TailwindCSS (executado sobre o Microsoft Edge WebView2 nativo do Windows).
   - **Sinalização**: WebSocket conectando ao `zyron-server` na porta `:8080`.
   - **Mídia P2P**: WebRTC via WebView2 (`RTCPeerConnection` nativo do navegador) ou via biblioteca Go `pion/webrtc`.

---

## 2. Identificação e Autenticação Automática (`whoami`)

Ao inicializar, o aplicativo Go deve:
1. Identificar o usuário da máquina atual (`DOMINIO\usuario` ou `usuario`).
2. Identificar o Hostname da máquina.
3. Identificar o endereço IPv4 ativo na rede LAN.
4. Consultar o servidor para saber se o usuário logado possui privilégios de **Operador de Suporte** ou de **Cliente Comum**.

### 2.1. Obtenção do Usuário no Go

```go
package system

import (
	"net"
	"os"
	"os/exec"
	"strings"
)

// GetCurrentUser executa whoami para obter o formato DOMINIO\usuario no Windows
func GetCurrentUser() string {
	cmd := exec.Command("whoami")
	out, err := cmd.Output()
	if err == nil {
		user := strings.TrimSpace(string(out))
		if user != "" {
			return user
		}
	}
	// Fallback para variáveis de ambiente
	domain := os.Getenv("USERDOMAIN")
	username := os.Getenv("USERNAME")
	if domain != "" && username != "" {
		return domain + "\\" + username
	}
	return username
}

// GetLocalIPv4 retorna o IP local ativo da máquina na rede LAN
func GetLocalIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
```

### 2.2. Verificação de Perfil no Servidor Central

O ClientApp faz uma requisição HTTP REST para identificar o papel do usuário:

- **Endpoint**: `POST http://<server-ip>:8080/api/auth/check-profile`
- **Request Headers**: `Content-Type: application/json`
- **Request Body**:
  ```json
  {
    "username": "CORP\\185033"
  }
  ```
- **Response**:
  ```json
  {
    "is_operator": true
  }
  ```
- Se `is_operator: true`: O Wails renderiza a **Tela de Atendimento do Operador** (fila de chamados, visualizador de tela remota, controles de chamada).
- Se `is_operator: false`: O Wails renderiza a **Tela do Solicitante** (botão de pedir suporte, status na fila, painel de sessão ativa).

---

## 3. Conexão WebSocket e Ciclo de Vida da Sinalização

### 3.1. Conexão e Heartbeat
- **URL**: `ws://<server-ip>:8080/ws?username=DOMINIO\usuario&is_operator=true|false`
- **Heartbeat (Manutenção de Presença)**:
  - O servidor envia um frame WebSocket `Ping` a cada 5 segundos.
  - O cliente deve responder automaticamente com `Pong`.
  - Essa resposta renova a chave de presença de 10s no Redis (`presence:DOMINIO\usuario`).

### 3.2. Formato Padrão das Mensagens (`WsMessage`)
Todas as mensagens trocadas via WebSocket seguem esta estrutura JSON:
```json
{
  "type": "string",
  "from": "DOMINIO\\remetente",
  "to": "DOMINIO\\destinatario",
  "payload": {}
}
```

---

## 4. Fluxo do Cliente Solicitante (Usuário Comum)

### 4.1. Solicitar Suporte (`request_support`)
Quando o usuário clica em "Solicitar Suporte":
```json
{
  "type": "request_support",
  "from": "CORP\\185033",
  "payload": {
    "hostname": "MEU-PC",
    "ip": "192.168.1.150"
  }
}
```

### 4.2. Confirmação do Chamado (`ticket_created`)
O servidor responde com o ticket e a posição do usuário na fila:
```json
{
  "type": "ticket_created",
  "from": "server",
  "to": "CORP\\185033",
  "payload": {
    "id": "86dfe123-b4bb-4116-8eb1-59952b2c6332",
    "ticket_id": "86dfe123-b4bb-4116-8eb1-59952b2c6332",
    "client_user": "CORP\\185033",
    "hostname": "MEU-PC",
    "ip": "192.168.1.150",
    "status": "waiting",
    "position": 2,
    "queue_position": 2
  }
}
```
> **Nota de Validação**: Se o usuário solicitar novamente enquanto já estiver na fila, o servidor não cria chamado duplicado e mantém rigorosamente a sua posição.

### 4.3. Atualização de Posição (`queue_position`)
Sempre que a fila andar (outro chamado atendido ou cancelado), o servidor notifica os clientes que avançaram:
```json
{
  "type": "queue_position",
  "from": "server",
  "to": "CORP\\185033",
  "payload": {
    "ticket_id": "86dfe123-b4bb-4116-8eb1-59952b2c6332",
    "position": 1,
    "queue_position": 1,
    "status": "waiting"
  }
}
```
*A interface do usuário deve atualizar exibindo: "Você é o 1º da fila. Aguarde o operador aceitar."*

### 4.4. Cancelamento Voluntário (`cancel_ticket`)
Caso o usuário desista de aguardar:
```json
{
  "type": "cancel_ticket",
  "from": "CORP\\185033"
}
```

### 4.5. Chamado Aceito pelo Operador (`ticket_accepted`)
Quando o operador aceita o chamado, o cliente recebe:
```json
{
  "type": "ticket_accepted",
  "from": "CORP\\operador_ti",
  "to": "CORP\\185033",
  "payload": {
    "ticket_id": "86dfe123-b4bb-4116-8eb1-59952b2c6332",
    "operator": "CORP\\operador_ti"
  }
}
```
**Ação do Cliente**:
1. Captura a tela e o microfone local.
2. Inicia o handshake WebRTC criando a oferta (`createOffer`).
3. Envia a mensagem com `type: "offer"` para o operador via WebSocket.

---

## 5. Fluxo do Operador de Suporte

### 5.1. Recepção da Fila em Tempo Real (`queue_update`)
Ao se conectar e a cada alteração na fila, o operador recebe a lista ordenada de chamados:
```json
{
  "type": "queue_update",
  "payload": [
    {
      "id": "86dfe123-...",
      "ticket_id": "86dfe123-...",
      "client_user": "CORP\\185033",
      "hostname": "FINANCEIRO-01",
      "ip": "192.168.1.150",
      "position": 1,
      "status": "waiting"
    },
    {
      "id": "97acf234-...",
      "ticket_id": "97acf234-...",
      "client_user": "CORP\\outro_usuario",
      "hostname": "RH-NOTEBOOK",
      "ip": "192.168.1.162",
      "position": 2,
      "status": "waiting"
    }
  ]
}
```

### 5.2. Aceitar Chamado (`accept_ticket`)
O operador escolhe um ticket para atender e envia:
```json
{
  "type": "accept_ticket",
  "from": "CORP\\operador_ti",
  "payload": {
    "ticket_id": "86dfe123-...",
    "client_user": "CORP\\185033"
  }
}
```

### 5.3. Confirmação do Aceite (`ticket_accepted`)
O servidor confirma a associação e entrega o IP direto da máquina do cliente:
```json
{
  "type": "ticket_accepted",
  "from": "server",
  "to": "CORP\\operador_ti",
  "payload": {
    "ticket_id": "86dfe123-...",
    "client_user": "CORP\\185033",
    "client_ip": "192.168.1.150"
  }
}
```

---

## 6. Implementação de Mídia P2P (WebRTC: Tela + Voz)

Como o Wails executa sobre o motor **Edge WebView2 (Chromium)**, a forma mais eficiente e performática de implementar a transmissão de tela e voz é utilizando a **API WebRTC nativa do navegador (`RTCPeerConnection`)** no código JavaScript/TypeScript do frontend.

### 6.1. Configuração do PeerConnection
Na rede local corporativa (LAN), servidores STUN/TURN públicos não são necessários, pois a conexão ocorre diretamente entre os IPs internos (Host Candidates):

```javascript
const configuration = {
  iceServers: [] // Na LAN corporativa, os candidatos diretos do tipo "host" são utilizados
};
const peerConnection = new RTCPeerConnection(configuration);
```

### 6.2. No Solicitante (Compartilha Tela + Microfone)

```javascript
async function startSharing(targetOperator) {
  // 1. Captura de Tela (Vídeo)
  const screenStream = await navigator.mediaDevices.getDisplayMedia({
    video: {
      cursor: "always",
      frameRate: { ideal: 30, max: 60 }
    },
    audio: false
  });

  // 2. Captura de Microfone (Áudio / Voz)
  const voiceStream = await navigator.mediaDevices.getUserMedia({
    audio: {
      echoCancellation: true,
      noiseSuppression: true,
      autoGainControl: true
    },
    video: false
  });

  // 3. Adiciona os tracks ao WebRTC
  screenStream.getVideoTracks().forEach(track => {
    peerConnection.addTrack(track, screenStream);
  });
  voiceStream.getAudioTracks().forEach(track => {
    peerConnection.addTrack(track, voiceStream);
  });

  // 4. Recebe a voz do Operador (Reprodução de Áudio Remoto)
  peerConnection.ontrack = (event) => {
    if (event.track.kind === 'audio') {
      const remoteAudio = document.getElementById('remoteOperatorAudio');
      remoteAudio.srcObject = event.streams[0];
      remoteAudio.play();
    }
  };

  // 5. Envia ICE Candidates para o Operador via WebSocket
  peerConnection.onicecandidate = (event) => {
    if (event.candidate) {
      wsSend({
        type: "candidate",
        to: targetOperator,
        payload: event.candidate
      });
    }
  };

  // 6. Cria a Oferta SDP (Offer)
  const offer = await peerConnection.createOffer();
  await peerConnection.setLocalDescription(offer);

  wsSend({
    type: "offer",
    to: targetOperator,
    payload: offer
  });
}
```

### 6.3. No Operador (Exibe Tela Remota + Microfone para Voz)

```javascript
async function handleIncomingCall(targetClient) {
  // 1. Captura o microfone do operador para conversa bidirecional
  const operatorVoice = await navigator.mediaDevices.getUserMedia({
    audio: {
      echoCancellation: true,
      noiseSuppression: true
    }
  });

  operatorVoice.getAudioTracks().forEach(track => {
    peerConnection.addTrack(track, operatorVoice);
  });

  // 2. Recebe os streams de tela e áudio do cliente
  peerConnection.ontrack = (event) => {
    if (event.track.kind === 'video') {
      // Exibe a tela do cliente no elemento <video>
      const screenVideo = document.getElementById('clientScreenViewer');
      screenVideo.srcObject = event.streams[0];
      screenVideo.play();
    } else if (event.track.kind === 'audio') {
      // Toca a voz do cliente nas caixas de som/fone
      const clientAudio = document.getElementById('clientVoiceAudio');
      clientAudio.srcObject = event.streams[0];
      clientAudio.play();
    }
  };

  // 3. Envia ICE Candidates do operador para o cliente
  peerConnection.onicecandidate = (event) => {
    if (event.candidate) {
      wsSend({
        type: "candidate",
        to: targetClient,
        payload: event.candidate
      });
    }
  };
}

// Ao receber a oferta (type: "offer") do cliente:
async function onReceiveOffer(senderUser, offer) {
  await peerConnection.setRemoteDescription(new RTCSessionDescription(offer));
  const answer = await peerConnection.createAnswer();
  await peerConnection.setLocalDescription(answer);

  wsSend({
    type: "answer",
    to: senderUser,
    payload: answer
  });
}

// Ao receber resposta (type: "answer"):
async function onReceiveAnswer(answer) {
  await peerConnection.setRemoteDescription(new RTCSessionDescription(answer));
}

// Ao receber ICE candidate (type: "candidate"):
async function onReceiveCandidate(candidate) {
  await peerConnection.addIceCandidate(new RTCIceCandidate(candidate));
}
```

---

## 7. Encerramento de Sessão e Auditoria

Quando o operador ou o cliente clica em "Encerrar Atendimento":
1. As faixas de mídia (áudio e tela) são encerradas (`track.stop()`) e `peerConnection.close()`.
2. O aplicativo envia a mensagem `session_closed`:
   ```json
   {
     "type": "session_closed",
     "from": "CORP\\operador_ti",
     "to": "CORP\\185033",
     "payload": {
       "ticket_id": "86dfe123-..."
     }
   }
   ```
3. O servidor central grava a auditoria no **PostgreSQL** com data de início, fim, duração em segundos, IPs e usuários envolvidos.
4. O cliente volta para o estado inicial pronto para novas solicitações.

---

## 8. Estrutura de Arquivos Recomendada para o Projeto Wails

```
zyron-client/
├── build/
│   ├── appicon.png
│   └── windows/
│       └── info.plist
├── frontend/
│   ├── src/
│   │   ├── assets/
│   │   ├── components/
│   │   │   ├── HeaderUser.vue         # Identificação DOMINIO\usuario e status
│   │   │   ├── ClientWaitingQueue.vue # Card com a posição na fila (1º, 2º...)
│   │   │   ├── OperatorQueueTable.vue # Tabela de chamados pendentes
│   │   │   ├── ScreenViewer.vue       # Player <video> da tela do cliente
│   │   │   └── VoiceControls.vue      # Botão de mudo e volume de voz
│   │   ├── services/
│   │   │   ├── ws.js                  # Gerenciador da conexão WebSocket
│   │   │   └── webrtc.js              # Gerenciador do RTCPeerConnection
│   │   ├── App.vue                    # Roteador de tela (Cliente vs Operador)
│   │   └── main.js
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── internal/
│   ├── system/
│   │   └── identity.go                # whoami, hostname, IP local
│   └── config/
│       └── config.go                  # URL do servidor zyron-server
├── app.go                             # Struct App com métodos expostos ao Wails
├── main.go                            # Inicialização do Wails Desktop
├── wails.json
├── go.mod
└── go.sum
```

---

## 9. Comandos para Setup e Compilação do ClientApp

### 9.1. Pré-requisitos
- **Go 1.22+**: [golang.org](https://golang.org)
- **Node.js 18+ e NPM**: [nodejs.org](https://nodejs.org)
- **Wails CLI v2**:
  ```powershell
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

### 9.2. Criando o Projeto
```powershell
wails init -n zyron-client -t vue
cd zyron-client
```

### 9.3. Execução em Desenvolvimento (Hot-reload)
```powershell
wails dev
```

### 9.4. Geração do Executável Windows (Produção)
```powershell
wails build -windowsconsole=false -clean
```
O executável final `.exe` será gerado em `build/bin/zyron-client.exe`, pronto para distribuição corporativa via GPO, SCCM ou execução direta.
