$ErrorActionPreference = 'Stop'

$workspacePath = Split-Path -Parent $PSScriptRoot
$env:GOCACHE = Join-Path $workspacePath 'tmp/go-cache'
$env:GOMODCACHE = Join-Path $workspacePath 'tmp/go-mod-cache'
$env:GOPATH = Join-Path $workspacePath 'tmp/go-path'
$env:GOMAXPROCS = '2'
$env:GOMEMLIMIT = '512MiB'
$env:RAYON_NUM_THREADS = '1'
$env:NODE_OPTIONS = '--max-old-space-size=512'
