# Encerra instância anterior se estiver em execução para liberar o arquivo executável
Stop-Process -Name zyron-client -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

# 1. Compilação via Wails v2
wails build -windowsconsole=false -clean

if ($LASTEXITCODE -eq 0) {
    if (Test-Path "config.json") {
        Copy-Item -Path "config.json" -Destination "build\bin\config.json" -Force
        # Oculta o arquivo config.json (attrib +h)
        attrib +h "build\bin\config.json" 2>$null
        attrib +h "config.json" 2>$null
        Write-Host "  -> config.json copiado e definido como oculto (+h)" -ForegroundColor Cyan
    }

    # Distribuição para downloads do servidor se existir
    $downloadsDir = "..\zyron-server\downloads"
    if (Test-Path $downloadsDir) {
        Copy-Item -Path "build\bin\zyron-client.exe" -Destination (Join-Path $downloadsDir "zyron-client.exe") -Force
        if (Test-Path "config.json") {
            $destServerConfig = Join-Path $downloadsDir "config.json"
            Copy-Item -Path "config.json" -Destination $destServerConfig -Force
            attrib +h $destServerConfig 2>$null
        }
        Write-Host "  -> Sincronizado em zyron-server\downloads" -ForegroundColor Cyan
    }

    # Distribuição para Desktop do usuário se existir
    $desktopZyron = Join-Path $env:USERPROFILE "Desktop\zyron"
    if (Test-Path $desktopZyron) {
        Copy-Item -Path "build\bin\zyron-client.exe" -Destination (Join-Path $desktopZyron "zyron-client.exe") -Force
        if (Test-Path "config.json") {
            $destDesktopConfig = Join-Path $desktopZyron "config.json"
            Copy-Item -Path "config.json" -Destination $destDesktopConfig -Force
            attrib +h $destDesktopConfig 2>$null
        }
        Write-Host "  -> Sincronizado em Desktop\zyron (config.json oculto)" -ForegroundColor Cyan
    }

    Write-Host "`n[SUCESSO] Executável gerado com sucesso!" -ForegroundColor Green
    Write-Host "Arquivos em build\bin\ (incluindo ocultos):" -ForegroundColor Yellow
    Get-ChildItem -Force "build\bin" | Select-Object Name, Mode, Length, LastWriteTime | Format-Table -AutoSize
} else {
    Write-Host "`n[ERRO] Falha durante a compilação do Wails." -ForegroundColor Red
}
