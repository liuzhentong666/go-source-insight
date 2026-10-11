function Initialize-SourceReader {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'A compatible host Go toolchain is required for local source access.' }
    $state = Join-Path $projectDirectory '.runtime/source-reader'
    New-Item -ItemType Directory -Force -Path $state | Out-Null
    if ((Get-Item -LiteralPath $state).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Source reader state cannot be a linked directory.' }
    $identity = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
    $existingAcl = Get-Acl -LiteralPath $state
    if ($existingAcl.GetOwner([Security.Principal.SecurityIdentifier]).Value -ne $identity.Value) { throw 'Source reader state is not owned by the current user.' }
    $acl = $existingAcl
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($entry in @($acl.Access)) { $acl.RemoveAccessRuleSpecific($entry) }
    $acl.SetAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')))
    [IO.FileSystemAclExtensions]::SetAccessControl((Get-Item -LiteralPath $state), $acl)
    foreach ($file in (Get-ChildItem -LiteralPath $state -File)) {
        if ($file.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Source reader state cannot contain linked files.' }
        $existingFileAcl = Get-Acl -LiteralPath $file.FullName
        if ($existingFileAcl.GetOwner([Security.Principal.SecurityIdentifier]).Value -ne $identity.Value) { throw 'Source reader state contains a file owned by another user.' }
        $fileAcl = $existingFileAcl
        $fileAcl.SetAccessRuleProtection($true, $false)
        foreach ($entry in @($fileAcl.Access)) { $fileAcl.RemoveAccessRuleSpecific($entry) }
        $fileAcl.SetAccessRule((New-Object Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'Allow')))
        [IO.FileSystemAclExtensions]::SetAccessControl($file, $fileAcl)
    }
    $tokenFile = Join-Path $state 'token'
    if (-not (Test-Path -LiteralPath $tokenFile)) {
        try {
            $tokenStream = [IO.File]::Open($tokenFile, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
            $tokenWriter = New-Object IO.StreamWriter($tokenStream, (New-Object Text.UTF8Encoding($false)))
            try { $tokenWriter.Write((New-RandomCredential)) } finally { $tokenWriter.Dispose() }
        } catch { throw 'Could not create source-reader token; retry after any concurrent preparation completes. Existing tokens were not overwritten.' }
    }
    $tokenAcl = Get-Acl -LiteralPath $tokenFile
    $tokenAcl.SetAccessRuleProtection($true, $false)
    foreach ($entry in @($tokenAcl.Access)) { $tokenAcl.RemoveAccessRuleSpecific($entry) }
    $tokenAcl.SetAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', 'Allow')))
    [IO.FileSystemAclExtensions]::SetAccessControl((Get-Item -LiteralPath $tokenFile), $tokenAcl)
    $token = [IO.File]::ReadAllText($tokenFile).Trim()
    if ($token -notmatch '^[a-f0-9]{64}$') { throw 'Invalid source reader token file.' }
    $port = 19100
    if ($env:SOURCE_READER_PORT) { $port = [int]$env:SOURCE_READER_PORT }
    if ($port -lt 1024 -or $port -gt 65535) { throw 'Invalid source reader port.' }
    $binary = Join-Path $state 'source-reader.exe'
    $pidFile = Join-Path $state 'process.json'
    $sourceFiles = @(Get-ChildItem -LiteralPath (Join-Path $projectDirectory 'cmd/source-reader'),(Join-Path $projectDirectory 'internal/source'),(Join-Path $projectDirectory 'internal/localfiles') -Filter '*.go' -File -Recurse | Sort-Object FullName)
    $sourceFiles += Get-Item -LiteralPath (Join-Path $projectDirectory 'go.mod'),(Join-Path $projectDirectory 'go.sum')
    $sourceIdentity = (($sourceFiles | ForEach-Object { (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash }) -join '')
    $healthy = $false
    try { $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$port/health" -Headers @{Authorization="Bearer $token"} -TimeoutSec 2; $healthy = $response.StatusCode -eq 200 } catch {}
    if ($healthy) {
        if (-not (Test-Path -LiteralPath $pidFile)) { throw 'Reader port is occupied by an unmanaged service.' }
        $record = Get-Content -LiteralPath $pidFile -Raw | ConvertFrom-Json
        $process = Get-CimInstance Win32_Process -Filter "ProcessId = $($record.pid)"
        $recordedCreated = ([datetime]$record.created).ToUniversalTime().ToString('o')
        if (-not $process -or $process.ExecutablePath -ne $binary -or $record.port -ne $port -or $process.CreationDate.ToUniversalTime().ToString('o') -ne $recordedCreated) { throw 'Source reader identity does not match its managed state.' }
        if ($record.sourceIdentity -ne $sourceIdentity) { throw 'Managed source reader code changed. Finish source operations and stop the identified source-reader process before retrying; no running process was stopped.' }
    } else {
        if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { throw 'Source reader port is occupied. Select a free SOURCE_READER_PORT; no process was stopped.' }
        & go build -o $binary ./cmd/source-reader
        if ($LASTEXITCODE -ne 0) { throw 'Source reader build failed.' }
        $started = Start-Process -FilePath $binary -ArgumentList @('--listen', "0.0.0.0:$port", '--token-file', ('"' + $tokenFile + '"')) -WindowStyle Hidden -RedirectStandardOutput (Join-Path $state 'stdout.log') -RedirectStandardError (Join-Path $state 'stderr.log') -PassThru
        $process = Get-CimInstance Win32_Process -Filter "ProcessId = $($started.Id)"
        @{pid=$started.Id; port=$port; created=$process.CreationDate.ToUniversalTime().ToString('o');sourceIdentity=$sourceIdentity} | ConvertTo-Json | Set-Content -LiteralPath $pidFile
        for ($attempt=0; $attempt -lt 30; $attempt++) {
            try { $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$port/health" -Headers @{Authorization="Bearer $token"} -TimeoutSec 1; $healthy = $response.StatusCode -eq 200 } catch {}
            if ($healthy) { break }; Start-Sleep -Milliseconds 200
        }
        if (-not $healthy) { throw 'Source reader did not become ready; inspect its restricted runtime log.' }
    }
    $override = Join-Path $state 'compose.json'
    $settings = @{environment=@{SOURCE_ACCESS_MODE='host';SOURCE_READER_URL="http://host.docker.internal:$port";SOURCE_READER_TOKEN=$token};extra_hosts=@('host.docker.internal:host-gateway')}
    @{services=@{api=$settings;insight=$settings}} | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $override
    @{services=@{insight=$settings}} | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $state 'cli-compose.json')
    return $override
}
