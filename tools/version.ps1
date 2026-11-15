<#
.SYNOPSIS
Affiche la version d'Aria Shield sur sa dernière ligne, sans « v » (0.12.0).

.DESCRIPTION
La version vit dans src\build\windows\info.json (ProductVersion). Le script vérifie qu'elle a la forme X.Y.Z et
qu'elle est la même dans src\main.go (const Version) et src\build\config.yml : une version qui diffère d'un
fichier à l'autre arrête le script. Le tag d'une release est « v » suivi de ce numéro.
#>
[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$racine = Split-Path -Parent $PSScriptRoot

$info = Get-Content -LiteralPath (Join-Path $racine 'src\build\windows\info.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$version = "$($info.info.'0000'.ProductVersion)".Trim()
if ($version -notmatch '\A\d+\.\d+\.\d+\z') { throw "ProductVersion « $version » n'a pas la forme X.Y.Z (src\build\windows\info.json)." }

$main = Get-Content -LiteralPath (Join-Path $racine 'src\main.go') -Raw -Encoding UTF8
if ($main -notmatch 'const Version = "([^"]+)"') { throw "const Version introuvable dans src\main.go." }
if ($Matches[1] -ne $version) { throw "src\main.go annonce $($Matches[1]), info.json annonce $version." }

$config = Get-Content -LiteralPath (Join-Path $racine 'src\build\config.yml') -Raw -Encoding UTF8
if ($config -notmatch '(?m)^\s*version:\s*"([^"]+)"') { throw "version introuvable dans src\build\config.yml." }
if ($Matches[1] -ne $version) { throw "src\build\config.yml annonce $($Matches[1]), info.json annonce $version." }

Write-Output $version
