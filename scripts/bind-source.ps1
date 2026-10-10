# Generates ignored read-only binding configuration. Never restarts services.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][UInt64]$OwnerId,
    [Parameter(Mandatory=$true)][UInt64]$ProjectId,
    [Parameter(Mandatory=$true)][string]$HostRoot,
    [string[]]$ExcludePath = @(),
    [string]$StateDirectory = '.runtime/source-bindings',
    [switch]$Replace
)
$ErrorActionPreference = 'Stop'
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'A compatible Go toolchain is required to prepare source bindings.' }
Push-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
try {
    $bindingArgs = @('run','./scripts/source-binding','--owner-id',"$OwnerId",'--project-id',"$ProjectId",'--host-root',$HostRoot,'--state-dir',$StateDirectory)
    foreach ($item in $ExcludePath) { $bindingArgs += @('--exclude',$item) }
    if ($Replace) { $bindingArgs += '--replace' }
    & go @bindingArgs
    if ($LASTEXITCODE -ne 0) { throw 'Source binding preparation failed; no service update was attempted.' }
} finally { Pop-Location }
