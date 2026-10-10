# Isolated Web runtime verification; never starts the API/DB/AI deployment stack.
[CmdletBinding()]
param(
    [string]$Image = 'go-source-insight-web:local',
    [string]$MockImage = 'node:24-alpine'
)
$ErrorActionPreference = 'Stop'
$testID = 'insight-web-test-' + [Guid]::NewGuid().ToString('N').Substring(0, 12)
$network = $testID
$apiContainer = $testID + '-api'
$webContainer = $testID + '-web'
$testRoot = Join-Path (Split-Path -Parent $PSScriptRoot) ('bin/web-proxy-tests/' + $testID)
New-Item -ItemType Directory -Force -Path $testRoot | Out-Null
$mockFile = Join-Path $testRoot 'mock-api.cjs'
@'
const http = require('node:http');
http.createServer((request, response) => {
  let bytes = 0;
  request.on('data', chunk => { bytes += chunk.length; });
  request.on('end', () => {
    response.writeHead(request.url.includes('/missing') ? 404 : 200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify({ path: request.url, authorization: request.headers.authorization || '', bytes }));
  });
}).listen(8080, '0.0.0.0');
'@ | Set-Content -LiteralPath $mockFile -Encoding ascii

function Invoke-DockerChecked {
    param([string[]]$Arguments)
    $result = & docker @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Isolated Docker operation failed: $($Arguments[0])" }
    return $result
}
function Get-TestResponse {
    param([string]$Path, [string]$Body = $null)
    $hasBody = $PSBoundParameters.ContainsKey('Body')
    $request = New-Object Net.Http.HttpRequestMessage
    $request.Method = if ($hasBody) { [Net.Http.HttpMethod]::Post } else { [Net.Http.HttpMethod]::Get }
    $request.RequestUri = $base + $Path
    $request.Headers.TryAddWithoutValidation('Authorization', 'Bearer isolated-test-token') | Out-Null
    if ($hasBody) { $request.Content = New-Object Net.Http.StringContent($Body, [Text.Encoding]::UTF8, 'application/json') }
    try {
        $response = $client.SendAsync($request).GetAwaiter().GetResult()
        try {
            return @{ Status = [int]$response.StatusCode; Body = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult(); Cache = [string]$response.Headers.CacheControl }
        } finally { $response.Dispose() }
    } finally { $request.Dispose() }
}

Add-Type -AssemblyName System.Net.Http
$client = New-Object Net.Http.HttpClient
$client.Timeout = [TimeSpan]::FromSeconds(15)
try {
    Invoke-DockerChecked -Arguments @('network','create',$network) | Out-Null
    Invoke-DockerChecked -Arguments @('run','-d','--name',$apiContainer,'--network',$network,'--network-alias','api','--mount',"type=bind,source=$mockFile,target=/test/mock-api.cjs,readonly",$MockImage,'node','/test/mock-api.cjs') | Out-Null
    Invoke-DockerChecked -Arguments @('run','-d','--name',$webContainer,'--network',$network,'--read-only','--tmpfs','/tmp:size=32m,mode=1777','--cap-drop','ALL','--security-opt','no-new-privileges:true','-p','127.0.0.1::8080','-e','WEB_PROXY_TIMEOUT_SECONDS=330',$Image) | Out-Null
    $binding = Invoke-DockerChecked -Arguments @('port',$webContainer,'8080/tcp')
    if ($binding -notmatch '^127\.0\.0\.1:(\d+)$') { throw 'Web test did not bind only to loopback.' }
    $base = 'http://127.0.0.1:' + $matches[1]
    $ready = $false
    for ($attempt = 0; $attempt -lt 20; $attempt++) {
        try { $ready = (Get-TestResponse -Path '/web-health').Status -eq 200 } catch { $ready = $false }
        if ($ready) { break }
        Start-Sleep -Milliseconds 500
    }
    if (-not $ready) { throw 'Isolated Web failed readiness.' }
    $root = Get-TestResponse -Path '/'
    $spa = Get-TestResponse -Path '/workspace/42'
    if ($root.Status -ne 200 -or $spa.Status -ne 200 -or $root.Body -ne $spa.Body -or $root.Body -notmatch '<div id="root">') { throw 'SPA deep link did not return the built app.' }
    $proxy = Get-TestResponse -Path '/api/v1/probe?scope=project'
    $echo = $proxy.Body | ConvertFrom-Json
    if ($proxy.Status -ne 200 -or $echo.path -ne '/api/v1/probe?scope=project' -or $echo.authorization -ne 'Bearer isolated-test-token' -or $proxy.Cache -ne 'no-store') { throw 'API path/auth/no-cache proxy check failed.' }
    $missing = Get-TestResponse -Path '/api/v1/missing'
    if ($missing.Status -ne 404 -or $missing.Body -match '<!doctype') { throw 'API error was replaced by SPA content.' }
    $exact = Get-TestResponse -Path '/api/v1/body' -Body ('{"x":"' + ('x' * 16376) + '"}')
    if ($exact.Status -ne 200 -or ($exact.Body | ConvertFrom-Json).bytes -ne 16384) { throw 'A valid 16KiB body was truncated or rejected.' }
    if ((Get-TestResponse -Path '/api/v1/body' -Body ('{"x":"' + ('x' * 16377) + '"}')).Status -ne 413) { throw 'Oversized API body was accepted.' }
    $runtime = Invoke-DockerChecked -Arguments @('inspect','--format','{{.Config.User}} {{.HostConfig.ReadonlyRootfs}}',$webContainer)
    if ($runtime -ne '101:101 true') { throw 'Web runtime is not non-root/read-only.' }
    Invoke-DockerChecked -Arguments @('exec',$webContainer,'nginx','-c','/tmp/nginx.conf','-t') | Out-Null
    Invoke-DockerChecked -Arguments @('exec',$webContainer,'wget','-q','-O','/dev/null','http://127.0.0.1:8080/web-health') | Out-Null
    $healthy = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        $state = Invoke-DockerChecked -Arguments @('inspect','--format','{{.State.Health.Status}}',$webContainer)
        if ($state -eq 'healthy') { $healthy = $true; break }
        Start-Sleep -Milliseconds 500
    }
    if (-not $healthy) { throw 'The Web image Docker health check did not become healthy.' }
    & docker run --rm --network $network -e WEB_PROXY_TIMEOUT_SECONDS=invalid $Image 2>$null | Out-Null
    if ($LASTEXITCODE -eq 0) { throw 'Invalid runtime proxy timeout was accepted.' }
    Write-Host 'Isolated Web proxy checks passed: SPA, API path/auth/errors/no-store, exact/oversized bodies, runtime permissions, Nginx syntax and health command.'
} catch {
    & docker logs --tail 20 $webContainer 2>$null
    throw
} finally {
    $client.Dispose()
    & docker rm -f $webContainer $apiContainer 2>$null | Out-Null
    & docker network rm $network 2>$null | Out-Null
}
