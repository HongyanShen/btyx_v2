$ErrorActionPreference = 'Stop'

$workspacePath = Split-Path -Parent $PSScriptRoot
$manifestPath = Join-Path $workspacePath 'artifacts/processes.json'

if (!(Test-Path -LiteralPath $manifestPath)) {
    Write-Host '没有本项目的启动记录。'
    exit 0
}

$entries = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json

foreach ($entry in $entries) {
    $process = Get-Process -Id $entry.pid -ErrorAction SilentlyContinue
    if (!$process) { continue }
    if ($process.Path -ne $entry.executable) { throw "进程 ID 已被复用，拒绝停止：$($entry.name)" }

    if (!$entry.startedAt -or $process.StartTime.ToUniversalTime().Ticks.ToString() -ne $entry.startedAt) {
        throw "进程启动时间不匹配，拒绝停止：$($entry.name)"
    }

    Stop-Process -Id $entry.pid
    Write-Host "已停止 $($entry.name)"
}

Remove-Item -LiteralPath $manifestPath
