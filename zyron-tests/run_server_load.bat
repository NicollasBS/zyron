@echo off
chcp 65001 >nul
title Zyron - Teste de Carga no Servidor
echo ===============================================================================
echo                 ZYRON SERVER - TESTE DE CARGA WEBSOCKET
echo ===============================================================================

:: Permite passar o IP diretamente como argumento (ex: run_server_load.bat 192.168.1.100)
set S_HOST=%~1

if "%S_HOST%"=="" (
    echo.
    echo Informe o endereco do servidor (ex: localhost, 192.168.1.100 ou 10.0.0.5:8080)
    set /p S_HOST="Host/IP do Servidor [Pressione ENTER para localhost:8080]: "
)

if "%S_HOST%"=="" set S_HOST=localhost:8080

echo.
echo [*] Servidor Alvo Definido: %S_HOST%
echo.
echo Escolha o perfil de teste desejado:
echo.
echo [1] Carga Leve    (50 Clientes,  2 Operadores, 30s) - Validacao Rapida
echo [2] Carga Media   (250 Clientes, 10 Operadores, 60s) - Cenario Corporativo Padrao
echo [3] Carga Alta    (1000 Clientes, 25 Operadores, 2m) - Teste de Estresse e Concorrencia
echo [4] Personalizado (Ajustar clientes, operadores e duracao manualmente)
echo.
set /p OPT="Opcao (1-4) [Padrao: 1]: "

if "%OPT%"=="" set OPT=1
if "%OPT%"=="1" goto LEVE
if "%OPT%"=="2" goto MEDIA
if "%OPT%"=="3" goto ALTA
if "%OPT%"=="4" goto CUSTOM
goto LEVE

:LEVE
echo.
echo Executando Carga Leve em %S_HOST%...
server_load.exe -server %S_HOST% -clients 50 -ops 2 -duration 30s -ramp-up 3s
goto FIM

:MEDIA
echo.
echo Executando Carga Media em %S_HOST%...
server_load.exe -server %S_HOST% -clients 250 -ops 10 -duration 60s -ramp-up 5s
goto FIM

:ALTA
echo.
echo Executando Carga Alta em %S_HOST%...
server_load.exe -server %S_HOST% -clients 1000 -ops 25 -duration 2m -ramp-up 10s
goto FIM

:CUSTOM
echo.
echo Configuracao Personalizada:
set /p S_HOST_CUSTOM="Confirmar Host/IP do Servidor [Padrao: %S_HOST%]: "
if not "%S_HOST_CUSTOM%"=="" set S_HOST=%S_HOST_CUSTOM%

set /p S_CLI="Numero de Clientes Simultaneos [Padrao: 100]: "
if "%S_CLI%"=="" set S_CLI=100

set /p S_OPS="Numero de Operadores Simultaneos [Padrao: 5]: "
if "%S_OPS%"=="" set S_OPS=5

set /p S_DUR="Duracao do teste (ex: 30s, 1m, 5m) [Padrao: 1m]: "
if "%S_DUR%"=="" set S_DUR=1m

echo.
echo Executando Teste Personalizado em %S_HOST%...
server_load.exe -server %S_HOST% -clients %S_CLI% -ops %S_OPS% -duration %S_DUR%
goto FIM

:FIM
echo.
echo ===============================================================================
pause
