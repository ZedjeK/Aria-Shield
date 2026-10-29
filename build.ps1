# Aria Shield : génère le projet Wails v3, applique les sources et compile l'exe.
# Usage (PowerShell normal, pas besoin d'admin) :
#   powershell -ExecutionPolicy Bypass -File .\build.ps1

$ErrorActionPreference = 'Stop'
$WailsVersion = 'v3.0.0-beta.28'

$Root = $PSScriptRoot
$Src  = Join-Path $Root 'src'
$Work = Join-Path $Root '.work'
$App  = Join-Path $Work 'aria-shield'

function Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }

function Invoke-Native {
    param([string]$Exe, [string[]]$Arguments)
    & $Exe @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Exe a échoué (code $LASTEXITCODE)" }
}

function Require($name, $hint) {
    if (-not (Get-Command $name -ErrorAction SilentlyContinue)) {
        throw "$name introuvable. $hint"
    }
}

Step 'Vérification des outils'
Require 'go'   'Installe Go : https://go.dev/dl/'
Require 'node' 'Installe Node.js LTS : https://nodejs.org/'
Require 'npm'  'npm est fourni avec Node.js.'
Write-Host ("  " + (go version))
Write-Host ("  node " + (node --version))

$GoBin = Join-Path (go env GOPATH) 'bin'
if ($env:Path -notlike "*$GoBin*") { $env:Path = "$GoBin;$env:Path" }

if (-not (Get-Command 'wails3' -ErrorAction SilentlyContinue)) {
    Step "Installation du CLI Wails $WailsVersion"
    Invoke-Native 'go' @('install', "github.com/wailsapp/wails/v3/cmd/wails3@$WailsVersion")
}
Write-Host ("  " + (wails3 version))

if (-not (Test-Path (Join-Path $App 'main.go'))) {
    Step 'Génération du projet Wails (template Svelte)'
    New-Item -ItemType Directory -Force -Path $Work | Out-Null
    Invoke-Native 'wails3' @('init', '-n', 'aria-shield', '-t', 'svelte-js', '-mod', 'aria-shield', '-d', $Work)
    Remove-Item (Join-Path $App 'greetservice.go') -ErrorAction SilentlyContinue
    # Fichiers de démo du template, inutiles ici
    Get-ChildItem (Join-Path $App 'frontend\public') -ErrorAction SilentlyContinue | Remove-Item -Recurse -Force
}

Step 'Tor Browser à embarquer'
# Trop lourd pour le zip des sources : récupéré sur le site officiel de Tor.
# Sa signature est vérifiée par Aria avant toute installation.
$TorVersion = '15.0.24'
$TorAssets  = Join-Path $Src 'internal\torbrowser\assets'
$TorExe     = Join-Path $TorAssets 'tor-browser-portable.exe'
if (-not (Test-Path $TorExe)) {
    # dist.torproject.org ne garde que les dernières versions ; les plus anciennes
    # passent sur archive.torproject.org.
    $file = "tor-browser-windows-x86_64-portable-$TorVersion.exe"
    $bases = @(
        "https://dist.torproject.org/torbrowser/$TorVersion",
        "https://archive.torproject.org/tor-package-archive/torbrowser/$TorVersion"
    )
    $ProgressPreference = 'SilentlyContinue' # sinon Invoke-WebRequest est très lent
    Write-Host "  Téléchargement de Tor Browser $TorVersion (environ 110 Mo)"
    $ok = $false
    foreach ($base in $bases) {
        try {
            Invoke-WebRequest -Uri "$base/$file.asc" -OutFile "$TorExe.asc" -UseBasicParsing
            Invoke-WebRequest -Uri "$base/$file" -OutFile $TorExe -UseBasicParsing
            $ok = $true
            break
        } catch {
            Write-Host "  Introuvable sur $base, essai suivant"
            Remove-Item $TorExe, "$TorExe.asc" -ErrorAction SilentlyContinue
        }
    }
    if (-not $ok) { throw "Impossible de télécharger Tor Browser $TorVersion." }
} else {
    Write-Host '  Déjà présent'
}

Step 'Application des sources Aria Shield'
# robocopy fusionne proprement les dossiers ; codes de sortie 0 à 7 = succès
robocopy $Src $App /E /NFL /NDL /NJH /NJS /NP | Out-Null
if ($LASTEXITCODE -ge 8) { throw "Copie des sources échouée (robocopy code $LASTEXITCODE)" }

Step 'Compilation (peut prendre quelques minutes la première fois)'
Push-Location $App
try {
    Invoke-Native 'wails3' @('build')
} finally {
    Pop-Location
}

$Exe = Join-Path $App 'bin\aria-shield.exe'
if (-not (Test-Path $Exe)) { throw "Compilation terminée mais $Exe est introuvable." }
Copy-Item $Exe (Join-Path $Root 'Aria-Shield.exe') -Force

Step 'Terminé'
Write-Host "  Exe prêt : $(Join-Path $Root 'Aria-Shield.exe')" -ForegroundColor Green
Write-Host '  Au lancement, Windows demandera les droits administrateur : c''est voulu.'
