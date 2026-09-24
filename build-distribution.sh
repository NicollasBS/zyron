#!/usr/bin/env bash
# ==============================================================================
# Zyron - Script de Build e Distribuição de Atualizações (Ambiente Linux)
# ==============================================================================
# Este script automatiza o processo no servidor Linux após executar 'git pull':
#   1. Detecta a versão da aplicação (ou aceita uma personalizada).
#   2. Compila o cliente Windows (zyron-client.exe) nativamente ou via Docker.
#   3. Copia o binário para zyron-server/downloads/.
#   4. Gera o arquivo version.json com a versão e notas de lançamento.
#   5. Sincroniza os arquivos com o container zyron_api em execução.
#   6. Opcionalmente reinicia os containers do servidor.
#
# Uso:
#   ./build-distribution.sh
#   ./build-distribution.sh --version 1.0.2 --notes "Correções e melhorias"
#   ./build-distribution.sh --skip-build
#   ./build-distribution.sh --restart-server
# ==============================================================================

set -e

# Cores para terminal
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Localização das pastas do projeto
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -d "$SCRIPT_DIR/zyron-client" ] && [ -d "$SCRIPT_DIR/zyron-server" ]; then
    ROOT_DIR="$SCRIPT_DIR"
elif [ -d "$SCRIPT_DIR/../zyron-client" ] && [ -d "$SCRIPT_DIR/../zyron-server" ]; then
    ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
else
    echo -e "${RED}[ERRO] Não foi possível localizar a raiz do projeto Zyron.${NC}"
    exit 1
fi

CLIENT_DIR="$ROOT_DIR/zyron-client"
SERVER_DIR="$ROOT_DIR/zyron-server"
DOWNLOADS_DIR="$SERVER_DIR/downloads"

CUSTOM_VERSION=""
CUSTOM_NOTES=""
SKIP_BUILD=false
RESTART_SERVER=false

# Processa parâmetros de linha de comando
while [[ $# -gt 0 ]]; do
    case "$1" in
        -v|--version)
            CUSTOM_VERSION="$2"
            shift 2
            ;;
        -n|--notes)
            CUSTOM_NOTES="$2"
            shift 2
            ;;
        -s|--skip-build)
            SKIP_BUILD=true
            shift
            ;;
        -r|--restart-server)
            RESTART_SERVER=true
            shift
            ;;
        -h|--help)
            echo "Uso: $0 [OPÇÕES]"
            echo ""
            echo "Opções:"
            echo "  -v, --version <versão>   Define a versão manualmente (ex: 1.0.2)"
            echo "  -n, --notes <texto>      Define as notas de lançamento"
            echo "  -s, --skip-build         Pula a compilação e usa o zyron-client.exe existente"
            echo "  -r, --restart-server     Reinicia o container zyron_api após publicação"
            echo "  -h, --help               Exibe esta mensagem de ajuda"
            exit 0
            ;;
        *)
            echo -e "${RED}[ERRO] Opção desconhecida: $1${NC}"
            echo "Use '$0 --help' para ver as opções disponíveis."
            exit 1
            ;;
    esac
done

echo -e "${BLUE}========================================================${NC}"
echo -e "${BLUE}      Zyron - Build & Distribuição de Atualizações      ${NC}"
echo -e "${BLUE}========================================================${NC}"
echo ""

# 1. Determinação da Versão
VERSION_FILE="$CLIENT_DIR/internal/version/version.go"
if [ -n "$CUSTOM_VERSION" ]; then
    VERSION="$CUSTOM_VERSION"
    if [ -f "$VERSION_FILE" ]; then
        echo -e "${YELLOW}[1/5] Atualizando versão no código para v$VERSION...${NC}"
        sed -i -E "s/const CurrentVersion = \".*\"/const CurrentVersion = \"$VERSION\"/" "$VERSION_FILE"
    fi
else
    if [ -f "$VERSION_FILE" ]; then
        VERSION=$(grep -E 'CurrentVersion\s*=' "$VERSION_FILE" | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    if [ -z "$VERSION" ]; then
        VERSION="1.0.0"
    fi
    echo -e "${GREEN}[1/5] Versão detectada no código: v$VERSION${NC}"
fi

RELEASE_NOTES="${CUSTOM_NOTES:-Atualização automática oficial do Zyron Client v$VERSION}"

# 2. Compilação do Cliente Windows x64
if [ "$SKIP_BUILD" = true ]; then
    echo -e "${YELLOW}[2/5] Compilação ignorada (--skip-build). Utilizando binário existente.${NC}"
else
    echo -e "${YELLOW}[2/5] Compilando Zyron Client para Windows x64...${NC}"
    mkdir -p "$CLIENT_DIR/build/bin"

    # Verifica se ferramentas nativas estão disponíveis no host Linux
    if command -v wails >/dev/null 2>&1 && command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then
        echo -e "${BLUE}  -> Compilando nativamente com Wails e MinGW-w64...${NC}"
        (cd "$CLIENT_DIR" && wails build -platform windows/amd64 -windowsconsole=false -clean)
    elif command -v docker >/dev/null 2>&1; then
        echo -e "${BLUE}  -> Wails/MinGW nativos não encontrados. Usando builder Docker...${NC}"
        BUILDER_IMAGE="zyron-client-builder"
        (cd "$CLIENT_DIR" && docker build -f Dockerfile.build-windows -t "$BUILDER_IMAGE" .)
        docker run --rm -v "$CLIENT_DIR:/app" "$BUILDER_IMAGE"
    else
        echo -e "${RED}[ERRO] Nenhuma ferramenta de build encontrada.${NC}"
        echo "Instale 'wails' + 'gcc-mingw-w64-x86-64' ou instale o 'docker'."
        exit 1
    fi
fi

# 3. Distribuição do Executável para a pasta de downloads do servidor
echo -e "${YELLOW}[3/5] Publicando binário na pasta de downloads do servidor...${NC}"
mkdir -p "$DOWNLOADS_DIR"

if [ -f "$CLIENT_DIR/build/bin/zyron-client.exe" ]; then
    cp -f "$CLIENT_DIR/build/bin/zyron-client.exe" "$DOWNLOADS_DIR/zyron-client.exe"
    echo -e "${GREEN}  -> Copiado de build/bin para $DOWNLOADS_DIR/zyron-client.exe${NC}"
fi

if [ ! -f "$DOWNLOADS_DIR/zyron-client.exe" ]; then
    echo -e "${RED}[ERRO] O arquivo $DOWNLOADS_DIR/zyron-client.exe não foi encontrado!${NC}"
    exit 1
fi

EXE_SIZE=$(wc -c < "$DOWNLOADS_DIR/zyron-client.exe" | tr -d ' ')
if command -v sha256sum >/dev/null 2>&1; then
    EXE_SHA256=$(sha256sum "$DOWNLOADS_DIR/zyron-client.exe" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    EXE_SHA256=$(shasum -a 256 "$DOWNLOADS_DIR/zyron-client.exe" | awk '{print $1}')
else
    EXE_SHA256=""
fi

echo -e "${GREEN}  -> Tamanho do executável: $EXE_SIZE bytes${NC}"
if [ -n "$EXE_SHA256" ]; then
    echo -e "${GREEN}  -> SHA-256: $EXE_SHA256${NC}"
fi

# 4. Geração do version.json
echo -e "${YELLOW}[4/5] Gerando arquivo version.json...${NC}"
cat <<EOF > "$DOWNLOADS_DIR/version.json"
{
  "version": "$VERSION",
  "release_notes": "$RELEASE_NOTES"
}
EOF
echo -e "${GREEN}  -> Arquivo $DOWNLOADS_DIR/version.json atualizado com sucesso!${NC}"

# 5. Sincronização com o Docker Container do Servidor
echo -e "${YELLOW}[5/5] Sincronizando com o container do servidor...${NC}"
if command -v docker >/dev/null 2>&1; then
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^zyron_api$"; then
        docker exec -u 0 zyron_api mkdir -p /app/downloads >/dev/null 2>&1 || true
        docker cp "$DOWNLOADS_DIR/zyron-client.exe" zyron_api:/app/downloads/zyron-client.exe
        docker cp "$DOWNLOADS_DIR/version.json" zyron_api:/app/downloads/version.json
        docker exec -u 0 zyron_api chown -R appuser:appuser /app/downloads >/dev/null 2>&1 || true
        echo -e "${GREEN}  -> Sincronizado com o container 'zyron_api' em execução!${NC}"
    else
        echo -e "${BLUE}  -> Container 'zyron_api' não está rodando no momento.${NC}"
    fi

    if [ "$RESTART_SERVER" = true ]; then
        echo -e "${YELLOW}Reiniciando o container zyron_api com docker compose...${NC}"
        cd "$SERVER_DIR"
        if [ -f "docker-compose.yml" ]; then
            docker compose restart zyron_api 2>/dev/null || docker-compose restart zyron_api 2>/dev/null || docker compose up -d
        elif [ -f "docker-compose-dev.yml" ]; then
            docker compose -f docker-compose-dev.yml restart zyron_api 2>/dev/null || docker compose -f docker-compose-dev.yml up -d
        fi
        echo -e "${GREEN}  -> Servidor reiniciado!${NC}"
    fi
fi

echo ""
echo -e "${GREEN}========================================================${NC}"
echo -e "${GREEN}  [SUCESSO] Atualização publicada com sucesso! v$VERSION ${NC}"
echo -e "${GREEN}========================================================${NC}"
echo ""

# Validação do Endpoint se o servidor estiver acessível
if command -v curl >/dev/null 2>&1; then
    echo -e "${BLUE}Verificando resposta do endpoint /api/update/version local:${NC}"
    curl -s http://localhost:8080/api/update/version || echo "(Servidor local na porta 8080 não respondeu, verifique se está ativo)"
    echo ""
fi
