@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo         Compilacao do Zyron Client (Desktop)
echo ========================================================
echo.

echo [1/5] Verificando e finalizando instancias abertas do app...
taskkill /F /IM zyron-client.exe >nul 2>&1
taskkill /F /IM zyron.exe >nul 2>&1
ping -n 2 127.0.0.1 >nul


echo [2/5] Compilando executavel com Wails (icone oficial incorporado)...
wails build -windowsconsole=false -clean
if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERRO] Falha durante a compilacao com Wails.
    echo Verifique os erros acima.
    pause
    exit /b %ERRORLEVEL%
)

echo.
echo [3/5] Copiando e ocultando arquivos de configuracao...
copy /Y config.json build\bin\config.json >nul
attrib +h build\bin\config.json >nul 2>&1
attrib +h config.json >nul 2>&1
echo   - Arquivo config.json definido como oculto (+h)

echo [4/5] Distribuindo binarios atualizados...
if exist "..\zyron-server\downloads" (
    copy /Y build\bin\zyron-client.exe "..\zyron-server\downloads\zyron-client.exe" >nul
    copy /Y build\bin\config.json "..\zyron-server\downloads\config.json" >nul 2>&1
    attrib +h "..\zyron-server\downloads\config.json" >nul 2>&1
    echo   - Atualizado no servidor de downloads: zyron-server\downloads\zyron-client.exe
    docker cp "..\zyron-server\downloads\zyron-client.exe" zyron_api:/app/downloads/zyron-client.exe >nul 2>&1
    if exist "..\zyron-server\downloads\version.json" (
        docker cp "..\zyron-server\downloads\version.json" zyron_api:/app/downloads/version.json >nul 2>&1
    )
    if exist "..\zyron-server\downloads\config.json" (
        docker cp "..\zyron-server\downloads\config.json" zyron_api:/app/downloads/config.json >nul 2>&1
    )
)

if exist "%USERPROFILE%\Desktop\zyron" (
    copy /Y build\bin\zyron-client.exe "%USERPROFILE%\Desktop\zyron\zyron-client.exe" >nul
    copy /Y config.json "%USERPROFILE%\Desktop\zyron\config.json" >nul
    attrib +h "%USERPROFILE%\Desktop\zyron\config.json" >nul 2>&1
    echo   - Atualizado na pasta Desktop\zyron\zyron-client.exe (config.json oculto)
)

echo [5/5] Atualizando cache de icones do Windows...
ie4uinit.exe -show >nul 2>&1

echo.
echo ========================================================
echo  [SUCESSO] Build concluido com o novo icone!
echo  Executavel local: build\bin\zyron-client.exe
echo ========================================================
echo.
pause
