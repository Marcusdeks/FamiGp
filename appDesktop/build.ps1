# Compila la aplicación de escritorio para Windows: primero la interfaz React
# (que se incrusta en el ejecutable) y después el propio ejecutable.
# Uso: .\build.ps1   →   genera bin\FamiGP.exe
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    if (-not (Test-Path ../web/node_modules)) { npm --prefix ../web ci }
    npm --prefix ../web run build
    if ($LASTEXITCODE -ne 0) { throw 'Falló la compilación de la interfaz' }

    # -H windowsgui evita que se abra una consola junto a la ventana.
    go build -trimpath -ldflags '-H windowsgui -s -w' -o bin/FamiGP.exe .
    if ($LASTEXITCODE -ne 0) { throw 'Falló la compilación del ejecutable' }

    Write-Host 'Listo: appDesktop\bin\FamiGP.exe'
} finally {
    Pop-Location
}
