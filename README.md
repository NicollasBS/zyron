# Zyron - Sistema de Acesso e Suporte Remoto

O **Zyron** é uma plataforma de gerenciamento e acesso remoto composta por um servidor central (**Zyron Server**) e uma aplicação cliente desktop (**Zyron Client**) desenvolvida com suporte a auto-atualização silenciosa e segura no Windows.

---

## 📁 Estrutura do Repositório

```text
zyron/
├── build-distribution.sh          # Script de automação e distribuição no servidor Linux
├── README.md                      # Documentação dos comandos e operação
├── zyron-client/                  # Aplicação Cliente (Desktop - Wails + Go + Vue 3)
│   ├── config.json                # Configuração de conexão do cliente (IP e Porta)
│   ├── build.bat                  # Script de build para Windows
│   ├── Dockerfile.build-windows   # Build do cliente Windows em hosts Linux via Docker
│   ├── internal/version/          # Definição da versão do executável
│   └── internal/updater/          # Mecanismo de auto-update atômico
└── zyron-server/                  # Servidor Central (API Go + WebSockets + Docker)
    ├── docker-compose.yml         # Orquestração dos containers (API + PostgreSQL)
    ├── build-distribution.sh      # Atalho para o script de distribuição raiz
    └── downloads/                 # Diretório onde ficam o instalador e version.json
```

---

## 🚀 Script de Build e Distribuição (`build-distribution.sh`)

O script [`build-distribution.sh`](build-distribution.sh) automatiza todo o ciclo no servidor Linux após executar `git pull`:
1. Atualiza o IP e a Porta no [`config.json`](zyron-client/config.json).
2. Atualiza a versão semântica e gera as notas de lançamento.
3. Compila o executável do Windows (`zyron-client.exe`) via Docker (sem necessidade de ter Wails ou MinGW instalados no host).
4. Publica o executável e o `config.json` no diretório público do servidor (`zyron-server/downloads/`).
5. Gera o manifesto de atualização [`version.json`](zyron-server/downloads/version.json) com checksum SHA-256.
6. Sincroniza os arquivos automaticamente com o container Docker `zyron_api` em execução.

### Permissão de Execução (Linux / Git Bash)

Antes de executar pela primeira vez:
```bash
chmod +x build-distribution.sh
```

---

### Opções e Parâmetros

| Flag Curta | Flag Longa | Argumento | Descrição |
| :--- | :--- | :--- | :--- |
| `-i` | `--ip` | `<endereço>` | Define o IP ou Host do servidor no `config.json` |
| `-p` | `--port` | `<porta>` | Define a porta do servidor no `config.json` |
| `-c` | `--config` | *Nenhum* | Abre o assistente interativo para editar IP e Porta |
| `-v` | `--version` | `<versão>` | Define a nova versão manualmente (ex: `1.0.3`) |
| `-n` | `--notes` | `"<texto>"` | Define a mensagem/notas de lançamento da versão |
| `-s` | `--skip-build` | *Nenhum* | Pula a compilação e reutiliza o binário existente |
| `-r` | `--restart-server`| *Nenhum* | Reinicia o container `zyron_api` após a publicação |
| `-h` | `--help` | *Nenhum* | Exibe a lista completa de comandos e ajuda |

---

### Exemplos Práticos de Uso

#### 1. Build e distribuição padrão (detecta versão do código automaticamente)
```bash
./build-distribution.sh
```

#### 2. Alterar o IP e a Porta do servidor de conexão
```bash
./build-distribution.sh --ip 192.168.1.100 --port 8080
```

#### 3. Modo interativo (pergunta IP e Porta no terminal)
```bash
./build-distribution.sh --config
```

#### 4. Lançar uma nova versão com número e notas personalizadas
```bash
./build-distribution.sh -v 1.0.3 -n "Correções na tela de visualização remota"
```

#### 5. Alterar apenas o IP/versão sem recompilar o executável (rápido)
```bash
./build-distribution.sh --ip 10.0.0.50 --skip-build
```

#### 6. Atualizar a distribuição e reiniciar o container da API
```bash
./build-distribution.sh -v 1.0.4 --restart-server
```

---

## 🐳 Servidor Central (`zyron-server`)

### Iniciar o Servidor com Docker Compose

Na pasta `zyron-server/`:

```bash
cd zyron-server

# Subir os containers em background
docker compose up -d

# Visualizar logs em tempo real
docker compose logs -f zyron_api

# Reiniciar o container da API
docker compose restart zyron_api

# Parar os serviços
docker compose down
```

### Endpoints de Atualização do Servidor

- **Verificação de versão**: `GET /api/update/version`
- **Download do executável**: `GET /api/update/download`
- **Download da configuração**: `GET /downloads/config.json`

---

## 💻 Cliente Desktop (`zyron-client`)

### Build local no Windows

No ambiente Windows de desenvolvimento, você pode compilar tanto via Prompt de Comando (.bat) quanto via PowerShell (.ps1):

#### Via Batch Script:
```cmd
cd zyron-client
build.bat
```

#### Via PowerShell Script:
```powershell
cd zyron-client
.\build.ps1
```

> **Nota de Segurança:** Todos os scripts de build (`build.bat`, `build.ps1` e `build-distribution.sh`), assim como a própria inicialização da aplicação, definem automaticamente o arquivo `config.json` como **oculto** (`attrib +h` / `FILE_ATTRIBUTE_HIDDEN`).

Ou usando a CLI oficial do Wails:

```bash
# Modo de desenvolvimento (Live Reload)
wails dev

# Build de produção do executável
wails build
```

O executável gerado ficará em `zyron-client/build/bin/zyron-client.exe`.

---

## ⚙️ Onde Configurar as Versões

Caso deseje alterar as versões manualmente no código-fonte:

1. **Versão interna no Go**:
   [`zyron-client/internal/version/version.go`](zyron-client/internal/version/version.go)  
   `const CurrentVersion = "1.0.0"`

2. **Versão do projeto Wails**:
   [`zyron-client/wails.json`](zyron-client/wails.json)  
   `"info": { "productVersion": "1.0.0" }`

3. **Versão do Frontend**:
   [`zyron-client/frontend/package.json`](zyron-client/frontend/package.json)  
   `"version": "1.0.0"`

4. **Manifesto publicado no servidor**:
   [`zyron-server/downloads/version.json`](zyron-server/downloads/version.json)  
   *(Gerado automaticamente pelo `build-distribution.sh`)*

---

## 🛡️ Mecanismo de Atualização Automática

O cliente verifica periodicamente ou na inicialização se há uma versão mais recente no servidor (`/api/update/version`).  
Ao detectar uma versão superior:
- Realiza o download seguro e valida a integridade com hash SHA-256.
- Utiliza **substituição atômica no sistema de arquivos NTFS** (`.exe` -> `.exe.old`), dispensando a criação de scripts `.bat` em `%TEMP%`.
- 100% compatível com o Windows Defender e políticas de bloqueio de scripts/firewall corporativo.
- Reinicia o aplicativo diretamente na nova versão instalada.
