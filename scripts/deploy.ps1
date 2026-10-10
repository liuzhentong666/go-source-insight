[CmdletBinding()]
param(
	[ValidateSet('cli','api','web')]
	[string]$Mode = 'cli',
	[switch]$WithAI,
	[string]$ApiEnvFile = '.runtime/api.env',
    [string]$SourceBindingsComposeFile = '',
    [string]$SourceDirectory = '',
    [string]$SourceStateDirectory = '.runtime/source-bindings',
    [ValidateRange(30, 3600)]
    [int]$WaitTimeout = 300
)

$ErrorActionPreference = 'Stop'
if ($Mode -eq 'cli' -and $SourceBindingsComposeFile) { throw 'Source bindings are only valid with Mode api or web.' }
if ($Mode -eq 'cli' -and $SourceDirectory) { throw 'SourceDirectory requires Mode api or web.' }
if ($SourceDirectory -and $SourceBindingsComposeFile) { throw 'Use SourceDirectory by itself; it preserves the existing API source configuration automatically.' }

function New-RandomCredential {
    $randomBytes = New-Object byte[] 32
    $generator = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $generator.GetBytes($randomBytes) } finally { $generator.Dispose() }
    return [BitConverter]::ToString($randomBytes).Replace('-', '').ToLowerInvariant()
}
function Prepare-ApiCredentials {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path) -and -not ($env:API_DB_PASSWORD -and $env:API_JWT_SECRET)) {
        if ($env:API_DB_PASSWORD -or $env:API_JWT_SECRET) { throw 'Provide both API_DB_PASSWORD and API_JWT_SECRET, or leave both unset for first deployment.' }
        $credentialDirectory = Split-Path -Parent $Path
        if (-not $credentialDirectory) { throw 'The API environment file must have a parent directory.' }
        New-Item -ItemType Directory -Force -Path $credentialDirectory | Out-Null
        try {
            $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
            $acl = Get-Acl -LiteralPath $credentialDirectory
            $acl.SetAccessRuleProtection($true, $false)
            $rule = New-Object System.Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
            $acl.SetAccessRule($rule)
            Set-Acl -LiteralPath $credentialDirectory -AclObject $acl
        } catch { Write-Warning 'Could not restrict credential-directory permissions; restrict them manually.' }
        $dbPassword = $env:API_DB_PASSWORD
        if (-not $dbPassword) { $dbPassword = New-RandomCredential }
        $jwtSecret = $env:API_JWT_SECRET
        if (-not $jwtSecret) { $jwtSecret = New-RandomCredential }
        try {
            $stream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
            $writer = New-Object IO.StreamWriter($stream, (New-Object Text.UTF8Encoding($false)))
            try { $writer.Write("API_DB_PASSWORD=$dbPassword`nAPI_JWT_SECRET=$jwtSecret`n") } finally { $writer.Dispose() }
        } catch { throw 'Could not create the API credential file. Existing files are never overwritten.' }
        Write-Host 'Created local API credentials. Keep the ignored credential file backed up securely.'
    }
    # Validate without printing values. Environment injection overrides the file.
    $stored = @{}
    if (Test-Path -LiteralPath $Path) {
        foreach ($line in (Get-Content -LiteralPath $Path)) {
            if ($line -match '^\s*(API_DB_PASSWORD|API_JWT_SECRET)\s*=(.*)$') { $stored[$matches[1]] = $matches[2].Trim().Trim('"', "'") }
        }
    }
    $dbPassword = $env:API_DB_PASSWORD
    if (-not $dbPassword) { $dbPassword = $stored['API_DB_PASSWORD'] }
    $jwtSecret = $env:API_JWT_SECRET
    if (-not $jwtSecret) { $jwtSecret = $stored['API_JWT_SECRET'] }
    if (-not $dbPassword -or $dbPassword.Length -lt 16 -or $dbPassword -match '(?i)change.?me|password|example|placeholder') { throw 'API_DB_PASSWORD is missing or weak. Existing credentials were not modified.' }
    if (-not $jwtSecret -or $jwtSecret.Length -lt 32 -or $jwtSecret -match '(?i)secret|change.?me|password|example|placeholder|your-|replace' -or (($jwtSecret.ToCharArray() | Select-Object -Unique).Count -lt 12)) { throw 'API_JWT_SECRET is missing or weak. Existing credentials were not modified.' }
}
$projectDirectory = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $projectDirectory
try {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        throw 'Docker is required. Install Docker with Compose V2 before deploying.'
    }
    docker compose version
    if ($LASTEXITCODE -ne 0) { throw 'Docker Compose V2 is required.' }
    docker info --format '{{.ServerVersion}}'
    if ($LASTEXITCODE -ne 0) { throw 'The Docker engine is unavailable. Start it and retry.' }

    if ($Mode -in @('api','web')) {
        Prepare-ApiCredentials -Path $ApiEnvFile
        $composeOptions = @('-f', 'docker-compose.yml', '-f', 'docker-compose.api.yml', '--profile', 'api')
        $services = @('postgres', 'api')
        $buildServices = @('api')
        if ($Mode -eq 'web') {
            $composeOptions += @('-f', 'docker-compose.web.yml', '--profile', 'web')
            $services += 'web'
            $buildServices += 'web'
        }
        if (Test-Path -LiteralPath $ApiEnvFile) { $composeOptions += @('--env-file', $ApiEnvFile) }
        $existingAPI = ''
        if ($SourceDirectory) {
            if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'A compatible Go toolchain is required to prepare the local source directory.' }
            $existingAPI = ((docker compose @composeOptions ps -a -q api) -join "`n").Trim()
            if ($LASTEXITCODE -ne 0) { throw 'Could not identify the existing API in this Compose project.' }
            $sourceArgs = @('run','./scripts/source-binding','--local-root',$SourceDirectory,'--state-dir',$SourceStateDirectory,'--print-compose-file')
            if ($existingAPI) { $sourceArgs += @('--active-api-id',$existingAPI) }
            $SourceBindingsComposeFile = ((& go @sourceArgs) -join "`n").Trim()
            if ($LASTEXITCODE -ne 0 -or -not $SourceBindingsComposeFile) { throw 'Local source preparation failed; no service update was attempted.' }
            Write-Host 'Prepared the local source directory. Source changes never rebuild an existing index automatically.'
        }
        if ($SourceBindingsComposeFile) {
            if (-not (Test-Path -LiteralPath $SourceBindingsComposeFile -PathType Leaf)) { throw 'The source-binding Compose override does not exist.' }
            $composeOptions += @('-f', (Resolve-Path -LiteralPath $SourceBindingsComposeFile).Path)
        }
        if (-not $env:API_SOURCE_DIR) { New-Item -ItemType Directory -Force -Path 'sources' | Out-Null }
        Write-Host 'Validating API deployment configuration...'
        docker compose @composeOptions config --quiet
        if ($LASTEXITCODE -ne 0) { throw 'API Compose configuration is invalid.' }
        Write-Host 'Building deployment images...'
        docker compose @composeOptions build @buildServices
        if ($LASTEXITCODE -ne 0) { throw 'Deployment image build failed.' }
        Write-Host 'Starting services; waiting for Web/core readiness (AI readiness is separate)...'
        $updateOptions = @()
        if ($SourceDirectory -and $existingAPI) {
            $services = @('api')
            if ($Mode -eq 'web') { $services += 'web' }
            $updateOptions += '--no-deps'
            Write-Host 'Updating API/Web with the existing database and model services; API shutdown handles in-flight work.'
        }
        docker compose @composeOptions up -d @updateOptions --wait --wait-timeout $WaitTimeout @services
        if ($LASTEXITCODE -ne 0) { throw 'Deployment did not become ready. Check service logs, PostgreSQL, instance ownership and unchanged stored credentials.' }
        if ($WithAI) {
            Write-Host 'Starting optional AI services and verifying cached models; no models will be downloaded...'
            docker compose @composeOptions up -d --wait --wait-timeout $WaitTimeout ollama milvus
            if ($LASTEXITCODE -ne 0) { throw 'Optional AI services did not become healthy. Static API operations may still be available.' }
            docker compose @composeOptions run --rm -T insight -f json doctor
            if ($LASTEXITCODE -ne 0) { throw 'Cached AI services/models failed verification. No model download was attempted.' }
        }
        if ($Mode -eq 'web') {
            Write-Host 'Web and API core are ready. Open the loopback Web address shown by Compose (default http://127.0.0.1:3000). See web/README.md for source preparation.'
        } else {
            Write-Host 'API core is ready. Use the loopback API address shown by Compose (default http://127.0.0.1:8087). See api/README.md for registration and source binding.'
        }
        return
    }
    if ($WithAI) { throw 'WithAI is only valid with Mode api or web.' }

    Write-Host 'Validating deployment configuration...'
    docker compose --profile cli --profile setup config --quiet
    if ($LASTEXITCODE -ne 0) { throw 'Compose configuration is invalid.' }
    Write-Host 'Building the CLI image...'
    docker compose build insight
    if ($LASTEXITCODE -ne 0) { throw 'CLI image build failed.' }
    Write-Host 'Starting dependencies and waiting for readiness...'
    docker compose up -d --wait --wait-timeout $WaitTimeout ollama milvus
    if ($LASTEXITCODE -ne 0) { throw 'Dependencies did not become ready. Inspect docker compose logs.' }
    Write-Host 'Downloading model weights if needed. The first run can take several minutes...'
    docker compose run --rm models
    if ($LASTEXITCODE -ne 0) { throw 'Model initialization failed. Retry this script after checking connectivity.' }
    Write-Host 'Verifying model dimensions and vector service connectivity...'
    docker compose run --rm -T insight -f json doctor
    if ($LASTEXITCODE -ne 0) { throw 'Service verification failed. Check models, vector dimensions and Milvus connectivity.' }
    Write-Host 'Deployment is ready. Run: docker compose run --rm insight scan /workspace'
    Write-Host 'Then index the source directory and ask a question using the same collection.'
} finally {
    Pop-Location
}
