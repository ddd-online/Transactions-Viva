#Requires -Version 5.1
<#
  清理构建产物（只删仓库内 build/release 与 build/output）。
#>
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
foreach ($name in @('release', 'output')) {
    $path = Join-Path (Join-Path $root 'build') $name
    if (Test-Path -LiteralPath $path) {
        $resolved = (Resolve-Path -LiteralPath $path).Path
        if (-not $resolved.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "拒绝删除仓库外路径: $resolved"
        }
        Write-Host "[clean] 删除 $resolved"
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
Write-Host '[clean] 完成'
