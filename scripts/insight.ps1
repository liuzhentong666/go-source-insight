[CmdletBinding()]
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$CommandArgs)
$ErrorActionPreference = 'Stop'
$projectDirectory = Split-Path -Parent $PSScriptRoot
function New-RandomCredential {
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return [BitConverter]::ToString($bytes).Replace('-', '').ToLowerInvariant()
}
. "$PSScriptRoot/source-reader.ps1"
Push-Location -LiteralPath $projectDirectory
try {
    $null = Initialize-SourceReader
    if (-not $CommandArgs) { $CommandArgs = @('help') }
    & docker compose -f docker-compose.yml -f .runtime/source-reader/cli-compose.json --profile cli run --rm -T insight @CommandArgs
    if ($LASTEXITCODE -ne 0) { throw "CLI exited with code $LASTEXITCODE." }
} finally { Pop-Location }
