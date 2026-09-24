# Encerra instância anterior se estiver em execução para liberar o arquivo executável
Stop-Process -Name zyron-client -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

# 1. Compilação via Wails v2
wails build -windowsconsole=false -clean

if ($LASTEXITCODE -eq 0) {
    if (Test-Path "config.json") {
        Copy-Item -Path "config.json" -Destination "build\bin\config.json" -Force
    }
    Write-Host "`n[SUCESSO] Executável gerado com sucesso!" -ForegroundColor Green
    Write-Host "Binário e Configuração em build\bin\:" -ForegroundColor Yellow
    Get-ChildItem "build\bin" | Select-Object Name, Length, LastWriteTime | Format-Table -AutoSize
} else {
    Write-Host "`n[ERRO] Falha durante a compilação do Wails." -ForegroundColor Red
}
