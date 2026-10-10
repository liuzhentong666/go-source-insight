# Deployment flow tests use a fake Docker command and isolated ignored files.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$scriptDirectory = $PSScriptRoot
$testRoot = Join-Path (Split-Path -Parent $scriptDirectory) ('bin/deploy-tests/' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $testRoot | Out-Null
$savedVariables = @{}
foreach ($name in @('API_DB_PASSWORD','API_JWT_SECRET','API_SOURCE_DIR')) { $savedVariables[$name] = [Environment]::GetEnvironmentVariable($name,'Process'); [Environment]::SetEnvironmentVariable($name,$null,'Process') }
$env:API_SOURCE_DIR = $testRoot
$global:deploymentCalls = New-Object System.Collections.Generic.List[string]
$global:deploymentFailure = ''
$global:deploymentAPI = ''
function docker {
    $global:deploymentCalls.Add(($args -join ' '))
    $global:LASTEXITCODE = 0
    if ($global:deploymentFailure -and ($args -contains $global:deploymentFailure)) { $global:LASTEXITCODE = 17 }
    if ($args -contains 'ps' -and $global:deploymentAPI) { Write-Output $global:deploymentAPI }
}
try {
    $parseErrors = $null
    [void][System.Management.Automation.Language.Parser]::ParseFile((Join-Path $scriptDirectory 'deploy.ps1'),[ref]$null,[ref]$parseErrors)
    if ($parseErrors.Count) { throw 'PowerShell syntax validation failed.' }
    $envFile = Join-Path $testRoot 'credentials/api.env'
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode api -ApiEnvFile $envFile | Out-Null
    $initialHash = (Get-FileHash -LiteralPath $envFile).Hash
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode api -ApiEnvFile $envFile | Out-Null
    if ((Get-FileHash -LiteralPath $envFile).Hash -ne $initialHash) { throw 'Repeated deployment changed credentials.' }
    if (($global:deploymentCalls -join "`n") -match 'pull|run --rm models') { throw 'API mode attempted a model download.' }
    if (-not (($global:deploymentCalls -join "`n") -match 'up -d --wait --wait-timeout 300 postgres api')) { throw 'API flow did not wait for core readiness.' }
    $global:deploymentCalls.Clear()
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -ApiEnvFile $envFile | Out-Null
    if ((Get-FileHash -LiteralPath $envFile).Hash -ne $initialHash) { throw 'Web mode changed existing credentials.' }
    $webCalls = $global:deploymentCalls -join "`n"
    if ($webCalls -notmatch 'docker-compose.web.yml' -or $webCalls -notmatch 'build api web' -or $webCalls -notmatch 'up -d --wait --wait-timeout 300 postgres api web') { throw 'Web build/readiness flow is incomplete.' }
    if ($webCalls -match 'pull|run --rm models|ollama milvus') { throw 'Basic Web mode started or downloaded AI.' }
    $bindingFile = Join-Path $testRoot 'source bindings.compose.json'
    Set-Content -LiteralPath $bindingFile -Value '{"services":{"api":{"environment":{"SOURCE_BINDINGS_FILE":"/app/source-bindings.json"}}}}' -Encoding ascii
    $global:deploymentCalls.Clear()
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -ApiEnvFile $envFile -SourceBindingsComposeFile $bindingFile | Out-Null
    if (-not (($global:deploymentCalls -join "`n").Contains($bindingFile))) { throw 'Source binding override was not applied.' }
    if ((Get-FileHash -LiteralPath $envFile).Hash -ne $initialHash) { throw 'Binding deployment changed credentials.' }
    $localSource = Join-Path $testRoot 'local source $(&;) with spaces'
    New-Item -ItemType Directory -Path $localSource | Out-Null
    Set-Content -LiteralPath (Join-Path $localSource 'main.go') -Value 'package fixture' -Encoding ascii
    $localState = Join-Path $testRoot 'local state'
    & go run ./scripts/source-binding --owner-id 17 --project-id 29 --host-root $localSource --state-dir $localState | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Existing fixture binding preparation failed.' }
    $localEntry = Join-Path $localState 'source-bindings.compose.json'
    $global:deploymentCalls.Clear()
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -ApiEnvFile $envFile -SourceDirectory $localSource -SourceStateDirectory $localState | Out-Null
    $localHash = (Get-FileHash -LiteralPath $localEntry).Hash
    $localDoc = (Get-Content -LiteralPath $localEntry -Raw) | ConvertFrom-Json
    $localGeneration = $localDoc.services.api.volumes[0].source.Replace('$$','$')
    $registry = (Get-Content -LiteralPath $localGeneration -Raw) | ConvertFrom-Json
    if ($registry.local_root.host_root -ne $localSource -or $registry.bindings.Count -ne 1 -or $registry.local_root.container_root -ne '/bound-sources/local') { throw 'Directory entry lost local root or existing binding.' }
    if (-not (($global:deploymentCalls -join "`n").Contains($localEntry))) { throw 'Directory entry did not apply generated mounts.' }
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode api -ApiEnvFile $envFile -SourceDirectory $localSource -SourceStateDirectory $localState | Out-Null
    if ((Get-FileHash -LiteralPath $localEntry).Hash -ne $localHash -or (Get-FileHash -LiteralPath $envFile).Hash -ne $initialHash) { throw 'Repeated directory entry changed configuration or credentials.' }
    foreach ($invalid in @((Join-Path $localSource 'main.go'),(Join-Path $testRoot 'missing source'))) {
        $global:deploymentCalls.Clear()
        $failed = $false
        try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -ApiEnvFile $envFile -SourceDirectory $invalid -SourceStateDirectory $localState | Out-Null } catch { $failed = $true }
        if (-not $failed -or ($global:deploymentCalls -join "`n") -match 'build|up -d' -or (Get-FileHash -LiteralPath $localEntry).Hash -ne $localHash) { throw 'Invalid directory caused deployment or changed configuration.' }
    }
    # Wrapper-only existing-service branch: actual mount/pending validation is in Go fixtures.
    $realGo = (Get-Command go -CommandType Application).Source
    function go { $global:LASTEXITCODE = 0; Write-Output $localEntry }
    $global:deploymentAPI = 'abcdef123456'
    $global:deploymentCalls.Clear()
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -ApiEnvFile $envFile -SourceDirectory $localSource | Out-Null } finally { Remove-Item Function:\go; $global:deploymentAPI = '' }
    $existingCalls = $global:deploymentCalls -join "`n"
    if ($existingCalls -notmatch 'up -d --no-deps --wait --wait-timeout 300 api web' -or $existingCalls -match 'up .*postgres') { throw 'Directory update did not preserve existing dependency services.' }
    $failed = $false
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode cli -SourceBindingsComposeFile $bindingFile | Out-Null } catch { $failed = $true }
    if (-not $failed) { throw 'CLI accepted API source bindings.' }
    $failed = $false
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode api -ApiEnvFile $envFile -SourceBindingsComposeFile (Join-Path $testRoot 'missing.json') | Out-Null } catch { $failed = $true }
    if (-not $failed) { throw 'Missing binding override was accepted.' }
    $global:deploymentCalls.Clear()
    & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -WithAI -ApiEnvFile $envFile -WaitTimeout 60 | Out-Null
    $webCalls = $global:deploymentCalls -join "`n"
    if ($webCalls -notmatch 'up -d --wait --wait-timeout 60 ollama milvus' -or $webCalls -notmatch 'run --rm -T insight -f json doctor' -or $webCalls -match 'pull|run --rm models') { throw 'Web AI mode did not verify cached models safely.' }
    $global:deploymentFailure = 'up'
    $failed = $false
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode api -ApiEnvFile $envFile | Out-Null } catch { $failed = $true }
    if (-not $failed) { throw 'Failed readiness was reported as success.' }
    $failed = $false
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode web -ApiEnvFile $envFile | Out-Null } catch { $failed = $true }
    if (-not $failed) { throw 'Web readiness failure was reported as success.' }
    $global:deploymentFailure = ''
    $invalidFile = Join-Path $testRoot 'invalid.env'
    Set-Content -LiteralPath $invalidFile -Value 'API_DB_PASSWORD=' -Encoding ascii
    $invalidHash = (Get-FileHash -LiteralPath $invalidFile).Hash
    $failed = $false
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode api -ApiEnvFile $invalidFile | Out-Null } catch { $failed = $true }
    if (-not $failed -or (Get-FileHash -LiteralPath $invalidFile).Hash -ne $invalidHash) { throw 'Invalid existing credentials were accepted or changed.' }
    $failed = $false
    try { & (Join-Path $scriptDirectory 'deploy.ps1') -Mode unsupported | Out-Null } catch { $failed = $true }
    if (-not $failed) { throw 'Invalid mode was accepted.' }
    $global:deploymentCalls.Clear()
    & (Join-Path $scriptDirectory 'deploy.ps1') | Out-Null
    if (-not (($global:deploymentCalls -join "`n") -match 'build insight') -or (($global:deploymentCalls -join "`n") -match 'api.yml|web.yml|postgres api')) { throw 'Default CLI flow changed.' }
    Write-Host 'PowerShell deployment flow tests passed.'
} finally {
    foreach ($name in $savedVariables.Keys) { [Environment]::SetEnvironmentVariable($name,$savedVariables[$name],'Process') }
    Remove-Item Function:\docker
}
