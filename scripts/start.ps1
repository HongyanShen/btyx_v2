param(
    [switch]$Initialize,
    [switch]$BootstrapAdmin,
    [switch]$Seed,
    [switch]$SkipBuild
)

. (Join-Path $PSScriptRoot 'environment.ps1')

$backendPath = Join-Path $workspacePath 'backend'
$binaryPath = Join-Path $workspacePath 'artifacts/bin'
$logPath = Join-Path $workspacePath 'artifacts/logs'
$manifestPath = Join-Path $workspacePath 'artifacts/processes.json'
$env:BTYX_CONFIG = Join-Path $workspacePath '.env'
$entries = @('miniapi', 'admin', 'worker')

if (!(Test-Path -LiteralPath $env:BTYX_CONFIG)) { throw '请先根据 .env.example 创建本地 .env' }
if (Test-Path -LiteralPath $manifestPath) { throw '已有启动记录；请先运行 scripts/stop.ps1，避免重复启动' }

New-Item -ItemType Directory -Force -Path $binaryPath, $logPath | Out-Null

Push-Location $backendPath

try {
    if ($Initialize) {
        go run ./cmd/migrate
        if ($LASTEXITCODE -ne 0) { throw '数据库迁移失败' }
    }

    if ($BootstrapAdmin) {
        go run ./cmd/adminbootstrap
        if ($LASTEXITCODE -ne 0) { throw '超级管理员初始化失败' }
    }

    if ($Seed) {
        go run ./cmd/seed
        if ($LASTEXITCODE -ne 0) { throw '示例数据初始化失败' }
    }

    foreach ($entry in $entries) {
        $executable = Join-Path $binaryPath "$entry.exe"
        if ($SkipBuild) {
            if (!(Test-Path -LiteralPath $executable)) { throw "缺少 $entry 二进制，请去掉 -SkipBuild" }
        }
        else {
            go build -buildvcs=false -p 1 -o $executable "./cmd/$entry"
            if ($LASTEXITCODE -ne 0) { throw "$entry 构建失败" }
        }
    }
}
finally {
    Pop-Location
}

$started = @()

try {
    foreach ($entry in $entries) {
        $executable = Join-Path $binaryPath "$entry.exe"
        $process = Start-Process -FilePath $executable -WorkingDirectory $backendPath -WindowStyle Hidden -PassThru `
            -RedirectStandardOutput (Join-Path $logPath "$entry.out.log") -RedirectStandardError (Join-Path $logPath "$entry.err.log")
        $started += @{ name = $entry; pid = $process.Id; executable = $executable; startedAt = $process.StartTime.ToUniversalTime().Ticks.ToString() }
    }

    foreach ($address in @('http://127.0.0.1:8081/healthz', 'http://127.0.0.1:8083/healthz')) {
        $ready = $false
        for ($attempt = 0; $attempt -lt 20; $attempt++) {
            try {
                $response = Invoke-WebRequest -Uri $address -TimeoutSec 2
                if ($response.StatusCode -eq 200) { $ready = $true; break }
            }
            catch {
                Start-Sleep -Milliseconds 250
            }
        }

        if (!$ready) { throw "服务未就绪：$address，请查看 artifacts/logs" }
    }

    $started | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Write-Host '管理后台：http://127.0.0.1:8083'
    Write-Host '小程序 API：127.0.0.1:8081；worker 已启动。'
    Write-Host '后台使用运营账号用户名和密码登录。日志位于 artifacts/logs。'
}
catch {
    foreach ($entry in $started) {
        $process = Get-Process -Id $entry.pid -ErrorAction SilentlyContinue
        if ($process -and $process.Path -eq $entry.executable) {
            Stop-Process -Id $entry.pid
        }
    }

    throw
}
