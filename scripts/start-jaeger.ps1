$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$storage = New-Item -Force -ItemType Directory (Join-Path $root 'artifacts/jaeger')
$package = 'jaeger-2.23.0-windows-amd64'
$exe = Join-Path $storage.FullName "$package/jaeger.exe"
if (!(Test-Path -LiteralPath $exe)) {
    $archive = Join-Path $storage.FullName "$package.zip"
    Invoke-WebRequest "https://github.com/jaegertracing/jaeger/releases/download/v2.23.0/$package.zip" -OutFile $archive
    if ((Get-FileHash $archive -Algorithm SHA256).Hash -ne '56eccf9d72df52b65fc1bcbe0cb3c9cd566681de99274f1e8ecf07a7f9bf9f4c') {
        throw 'Jaeger 压缩包 SHA256 校验失败'
    }
    Expand-Archive -LiteralPath $archive -DestinationPath $storage.FullName -Force
}

function Start-LocalEndpoint($name, $command, $arguments, $directory, $url, $marker) {
    try {
        $existing = Invoke-WebRequest $url -TimeoutSec 2
    } catch { $existing = $null }
    if ($existing) {
        if (!$existing.Content.Contains($marker)) { throw "$url 由其他服务占用" }
        return
    }

    $p = Start-Process -FilePath $command -ArgumentList $arguments -WorkingDirectory $directory -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput (Join-Path $storage.FullName "$name.out.log") -RedirectStandardError (Join-Path $storage.FullName "$name.err.log")
    for ($i = 0; $i -lt 40; $i++) {
        if ($p.HasExited) { throw "$name 启动失败，详见 artifacts/jaeger/$name.err.log" }
        try {
            $response = Invoke-WebRequest $url -TimeoutSec 1
            if ($response.Content.Contains($marker)) { return }
        } catch {}
        Start-Sleep -Milliseconds 250
    }
    throw "$name 尚未就绪，详见 artifacts/jaeger/$name.err.log"
}

$config = Join-Path $PSScriptRoot 'jaeger/local.yaml'
Start-LocalEndpoint 'jaeger' $exe @('--config', ('"' + $config + '"')) $storage.FullName 'http://127.0.0.1:16686/' 'Jaeger'
Start-LocalEndpoint 'lookup' (Get-Command node).Source @('scripts/jaeger/request-id.mjs') $root 'http://127.0.0.1:16687/' 'X-Request-ID'
Write-Host '按 X-Request-ID 查询：http://127.0.0.1:16687/'
