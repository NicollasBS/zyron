# Stage 1: Build
FROM golang:alpine AS builder

WORKDIR /app

# Instala ferramentas necessárias
RUN apk add --no-cache git ca-certificates

# Cache de dependências do Go
COPY go.mod go.sum ./
RUN go mod download

# Copia o código fonte do projeto
COPY . .

# Compilação estática em binário Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /app/zyron-server ./cmd/server

# Stage 2: Imagem final leve e segura
FROM alpine:3.20

WORKDIR /app

# Instala certificados CA e wget para healthcheck
RUN apk --no-cache add ca-certificates tzdata wget

# Cria usuário não-root por segurança
RUN adduser -D -g '' appuser

# Copia o binário compilado
COPY --from=builder /app/zyron-server /app/zyron-server

# Define permissões
USER appuser

EXPOSE 8080

CMD ["/app/zyron-server"]
