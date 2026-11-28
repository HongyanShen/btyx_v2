. (Join-Path $PSScriptRoot 'environment.ps1')

Push-Location $workspacePath

try {
    go run scripts/check-go-style.go
    if ($LASTEXITCODE -ne 0) { throw 'Go 可读性检查失败' }

    Push-Location (Join-Path $workspacePath 'backend')

    try {
        go test -p 1 ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go 测试失败' }

        go vet -p 1 ./...
        if ($LASTEXITCODE -ne 0) { throw 'Go 静态检查失败' }

        go build -buildvcs=false -p 1 ./cmd/...
        if ($LASTEXITCODE -ne 0) { throw 'Go 进程入口构建失败' }
    }
    finally {
        Pop-Location
    }

    Push-Location (Join-Path $workspacePath 'backend/cmd/admin/tests')

    try {
        npm run format:check
        if ($LASTEXITCODE -ne 0) { throw '管理后台格式检查失败' }

        npm run typecheck
        if ($LASTEXITCODE -ne 0) { throw '管理后台回归代码类型检查失败' }

        npm test -- --maxWorkers=1
        if ($LASTEXITCODE -ne 0) { throw '管理后台测试失败' }
    }
    finally {
        Pop-Location
    }

    Push-Location (Join-Path $workspacePath 'apps/miniapp')

    try {
        npm test
        if ($LASTEXITCODE -ne 0) { throw '小程序纯 JS 单元测试失败' }
    }
    finally {
        Pop-Location
    }
}
finally {
    Pop-Location
}
