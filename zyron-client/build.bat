@echo off
echo Iniciando compilacao do Zyron Client...
wails build -windowsconsole=false -clean
if %ERRORLEVEL% EQU 0 (
    copy /Y config.json build\bin\config.json >nul
    echo.
    echo [SUCESSO] Executavel gerado em: build\bin\zyron-client.exe
    echo [CONFIG] Arquivo de configuracao copiado para: build\bin\config.json
) else (
    echo.
    echo [ERRO] Falha durante o build.
)
pause
