#Requires -Version 5.1
<#
  构建发布产物：测试 / vet / viva vet → viva package → 校验 NSIS 安装器。

  用法：
    powershell -NoProfile -ExecutionPolicy Bypass -File build/build.ps1
    powershell -NoProfile -ExecutionPolicy Bypass -File build/build.ps1 -SkipTests

  NSIS 默认从 D:\software\NSIS 查找；也可以先设置 NSIS_HOME。
  产物断言是最后一步：退出码为 0 但安装器没生成，视为失败。
#>
[CmdletBinding()]
param(
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $toml = Get-Content (Join-Path $root 'viva.toml') -Raw
    $match = [regex]::Match($toml, '(?m)^\s*version\s*=\s*"([^"]+)"')
    if (-not $match.Success) { throw 'viva.toml 没有 [app] version' }
    $version = $match.Groups[1].Value

    if (-not $SkipTests) {
        Write-Host '[build] go test ./...'
        go test ./... -count=1
        if ($LASTEXITCODE -ne 0) { throw "go test 失败（exit $LASTEXITCODE）" }

        Write-Host '[build] go vet ./...'
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "go vet 失败（exit $LASTEXITCODE）" }

        Write-Host '[build] go tool viva vet'
        go tool viva vet
        if ($LASTEXITCODE -ne 0) { throw "viva vet 失败（exit $LASTEXITCODE）" }
    }

    if (-not $env:NSIS_HOME -and (Test-Path -LiteralPath 'D:\software\NSIS')) {
        $env:NSIS_HOME = 'D:\software\NSIS'
    }

    Write-Host '[build] viva package -target windows -format nsis,zip'
    go tool viva package -target windows -arch amd64 -format nsis,zip --no-sign -o build\release
    if ($LASTEXITCODE -ne 0) { throw "viva package 失败（exit $LASTEXITCODE）" }

    $generated = Join-Path $root "build\release\transactions-$version-windows-amd64-setup.exe"
    if (-not (Test-Path -LiteralPath $generated)) {
        throw "缺少构建产物: $generated"
    }

    # Release assets use the old project's stable name: Transactions-<version>-setup.exe.
    $output = Join-Path $root 'build\output'
    New-Item -ItemType Directory -Force -Path $output | Out-Null
    $setup = Join-Path $output "Transactions-$version-setup.exe"
    Copy-Item -LiteralPath $generated -Destination $setup -Force

    Write-Host '[build] 完成'
    Get-Item -LiteralPath $setup | ForEach-Object {
        $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        Write-Host ("  {0}  {1:N1} MB  sha256={2}" -f $_.Name, ($_.Length / 1MB), $hash)
    }
}
finally {
    Pop-Location
}
