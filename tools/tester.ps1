<#
.SYNOPSIS
Lance les tests Go d'Aria Shield. Code de retour non nul en cas d'échec.

.DESCRIPTION
À lancer avant toute proposition de commit. Commun à la CI (job « tests ») et au poste.

Il faut Go (https://go.dev/dl/), et le Tor Browser embarqué : le code l'embarque à la compilation, donc les tests ne
compilent pas sans lui. build.ps1 le télécharge ; la CI le télécharge et en vérifie la signature avant ce script.

Sous GitHub Actions, chaque échec remonte en annotation, lisible sans ouvrir les journaux.
#>
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$racine = Split-Path -Parent $PSScriptRoot

if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "go est introuvable. Installe Go : https://go.dev/dl/" }
$tor = Join-Path $racine 'src\internal\torbrowser\assets\tor-browser-portable.exe'
if (-not (Test-Path -LiteralPath $tor)) {
    throw "Tor Browser n'est pas embarqué (src\internal\torbrowser\assets). Lance d'abord .\build.ps1, qui le télécharge."
}

Push-Location -LiteralPath (Join-Path $racine 'src')
try {
    # Sous Windows PowerShell 5.1, la sortie d'erreur d'un programme natif redirigée arrêterait le script :
    # on la lit comme une sortie, puis on regarde le code de retour.
    $ErrorActionPreference = 'Continue'
    $sortie = @(& go test ./internal/... 2>&1 | ForEach-Object { "$_" })
    $code = $LASTEXITCODE
}
finally {
    Pop-Location
    $ErrorActionPreference = 'Stop'
}

foreach ($ligne in $sortie) { Write-Host $ligne }

if ($code -ne 0) {
    if ($env:GITHUB_ACTIONS -eq 'true') {
        $sortie | Where-Object { $_ -match '--- FAIL|_test\.go:\d+|^FAIL|panic:' } | Select-Object -First 20 |
            ForEach-Object { Write-Host "::error::$_" }
    }
    Write-Host "Tests en échec (code $code)." -ForegroundColor Red
    exit 1
}
Write-Host "Tests réussis."
