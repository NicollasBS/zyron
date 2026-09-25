@echo off
chcp 65001 >nul
title Zyron - Teste de Qualidade P2P (UDP/WebRTC)
echo ===============================================================================
echo             ZYRON P2P - TESTE DE QUALIDADE DE REDE UDP (WEBRTC)
echo ===============================================================================
echo.
echo Selecione o papel desta maquina no teste:
echo.
echo [1] Servidor / Receptor (Executar na maquina do OPERADOR de suporte)
echo [2] Cliente / Emissor   (Executar na maquina do CLIENTE que pede suporte)
echo.
set /p ROLE="Opcao (1 ou 2) [Padrao: 1]: "

if "%ROLE%"=="" set ROLE=1
if "%ROLE%"=="1" goto SERVER
if "%ROLE%"=="2" goto CLIENT
goto SERVER

:SERVER
echo.
echo Iniciando Servidor UDP na porta 19800...
echo (Aguardando pacotes do cliente para medir latencia e jitter)
echo.
p2p_benchmark.exe -mode server -port 19800
goto FIM

:CLIENT
echo.
set /p TARGET_IP="Digite o endereco IP do Operador (Servidor UDP): "
if "%TARGET_IP%"=="" (
    echo [!] O IP do operador e obrigatorio para o modo cliente.
    pause
    exit /b 1
)

echo.
echo Perfil de simulacao de midia WebRTC:
echo [1] Padrao    (~600 Kbps - 60 pacotes/s, 1200 bytes, duracao 30s)
echo [2] Alta Taxa (~1.2 Mbps - 120 pacotes/s, 1200 bytes, duracao 30s)
echo [3] Leve/Voz  (~200 Kbps - 30 pacotes/s, 800 bytes, duracao 20s)
set /p P_OPT="Opcao [Padrao: 1]: "
if "%P_OPT%"=="" set P_OPT=1

if "%P_OPT%"=="1" p2p_benchmark.exe -mode client -target %TARGET_IP% -port 19800 -rate 60 -size 1200 -duration 30s
if "%P_OPT%"=="2" p2p_benchmark.exe -mode client -target %TARGET_IP% -port 19800 -rate 120 -size 1200 -duration 30s
if "%P_OPT%"=="3" p2p_benchmark.exe -mode client -target %TARGET_IP% -port 19800 -rate 30 -size 800 -duration 20s

goto FIM

:FIM
echo.
echo ===============================================================================
pause
