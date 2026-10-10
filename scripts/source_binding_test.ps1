# Local fixtures, Compose merge and a disposable networkless read-only mount probe.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$testRoot = Join-Path $projectRoot ('bin/source-binding-tests/' + [Guid]::NewGuid().ToString('N'))
$source = Join-Path $testRoot 'source $($null) & semi; with spaces'
$state = Join-Path $testRoot 'state with spaces'
New-Item -ItemType Directory -Force -Path $source | Out-Null
Set-Content -LiteralPath (Join-Path $source 'main.go') -Value 'package sample' -Encoding ascii
$savedPassword = $env:API_DB_PASSWORD
$savedJWT = $env:API_JWT_SECRET
Push-Location -LiteralPath $projectRoot
try {
    foreach ($file in @('scripts/bind-source.ps1','scripts/deploy.ps1','scripts/source_binding_test.ps1')) {
        $parseErrors=$null
        [void][System.Management.Automation.Language.Parser]::ParseFile((Resolve-Path $file),[ref]$null,[ref]$parseErrors)
        if ($parseErrors.Count) { throw 'PowerShell syntax validation failed.' }
    }
    & ./scripts/bind-source.ps1 -OwnerId 17 -ProjectId 29 -HostRoot $source -StateDirectory $state -ExcludePath generated
    $composeFile = Join-Path $state 'source-bindings.compose.json'
    $firstHash = (Get-FileHash -LiteralPath $composeFile).Hash
    $firstMount = ((Get-Content -LiteralPath $composeFile -Raw) | ConvertFrom-Json).services.api.volumes[0].source
    $firstGeneration = $firstMount.Replace('$$','$')
    $oldJSON = (Get-FileHash -LiteralPath $firstGeneration).Hash
    Set-Content -LiteralPath (Join-Path $state 'api.env') -Value 'isolated-unchanged-credential-fixture' -Encoding ascii
    $credentialHash = (Get-FileHash -LiteralPath (Join-Path $state 'api.env')).Hash
    & ./scripts/bind-source.ps1 -OwnerId 17 -ProjectId 29 -HostRoot $source -StateDirectory $state -ExcludePath generated
    if ((Get-FileHash -LiteralPath $composeFile).Hash -ne $firstHash) { throw 'Repeated wrapper invocation changed committed mounts.' }
    & ./scripts/bind-source.ps1 -OwnerId 31 -ProjectId 47 -HostRoot $source -StateDirectory $state
    if ((Get-FileHash -LiteralPath $firstGeneration).Hash -ne $oldJSON) { throw 'An existing mounted generation was overwritten.' }
    if ((Get-FileHash -LiteralPath (Join-Path $state 'api.env')).Hash -ne $credentialHash) { throw 'Binding wrapper changed existing credentials.' }
    $env:API_DB_PASSWORD = [Guid]::NewGuid().ToString('N')
    $env:API_JWT_SECRET = [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
    $options = @('-f','docker-compose.yml','-f','docker-compose.api.yml','-f','docker-compose.web.yml','-f',$composeFile,'--profile','api','--profile','web')
    & docker compose @options config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Generated source override did not merge with API/Web Compose.' }
    # Capture only synthetic test configuration in memory; never print the object.
    $merged = (& docker compose @options config --format json) | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect synthetic merged configuration.' }
    $mounts = @($merged.services.api.volumes)
    foreach ($target in @('/sources','/app/source-bindings.json','/bound-sources/owners/17/projects/29','/bound-sources/owners/31/projects/47')) {
        $matching = @($mounts | Where-Object target -eq $target)
        if ($matching.Count -ne 1 -or -not $matching[0].read_only -or $matching[0].bind.create_host_path) { throw 'Merged API source mounts lost read-only/legacy protection.' }
    }
    $boundMount = $mounts | Where-Object target -eq '/bound-sources/owners/17/projects/29'
    # Compose config escapes dollars for round-tripping its output.
    if ($boundMount.source.Replace('$$','$') -ne $source) { throw 'Compose interpolation changed the literal source directory.' }
    if ($merged.services.api.environment.SOURCE_BINDINGS_FILE -ne '/app/source-bindings.json') { throw 'Merged API binding file is incorrect.' }
    $generated = (Get-Content -LiteralPath $composeFile -Raw) | ConvertFrom-Json
    $probeMounts = @(
        @{type='bind'; source=$generated.services.api.volumes[0].source; target='/binding.json'; read_only=$true; bind=@{create_host_path=$false}},
        @{type='bind'; source=$generated.services.api.volumes[1].source; target='/probe'; read_only=$true; bind=@{create_host_path=$false}}
    )
    $probeFile = Join-Path $testRoot 'mount-probe.compose.json'
    @{services=@{probe=@{image='alpine:3.22'; user='10001:10001'; network_mode='none'; read_only=$true; volumes=$probeMounts; command=@('/bin/sh','-ec','test -r /probe/main.go && test -r /binding.json && ! touch /probe/write-attempt 2>/dev/null')}}} | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $probeFile -Encoding utf8
    $probeProject = 'insight-bind-probe-' + [Guid]::NewGuid().ToString('N').Substring(0,12)
    & docker compose -p $probeProject -f $probeFile run --rm -T --no-deps probe
    if ($LASTEXITCODE -ne 0) { throw 'The isolated non-root container could not read literal-path mounts or wrote through read-only mount.' }
    & docker compose --profile cli --profile setup config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'CLI base mode now requires source bindings.' }
    Write-Host 'Source binding wrapper/Compose fixture checks passed: literal paths, idempotency, retained generations/credentials, read-only and legacy mounts, independent CLI.'
} finally {
    $env:API_DB_PASSWORD=$savedPassword
    $env:API_JWT_SECRET=$savedJWT
    Pop-Location
}
