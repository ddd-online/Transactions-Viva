#Requires -Version 5.1
<#
  发布：校验版本 / 标签 / 工作树 / 产物 / 发布说明 → 创建 GitHub Release 并上传安装包。

  用法：
    powershell -NoProfile -ExecutionPolicy Bypass -File build/release.ps1 -Tag v0.1.0
    powershell -NoProfile -ExecutionPolicy Bypass -File build/release.ps1 -Tag v0.1.0 -Draft
    powershell -NoProfile -ExecutionPolicy Bypass -File build/release.ps1 -Tag v0.1.0 -SkipClean

  发布成功后默认清空 build/output 与 build/release；要保留现场排查时加 -SkipClean。

  只上传普通 .exe 资产；应用内更新读取 release 的 tag_name 与 asset.digest，
  所以不生成 latest.yml，也不需要签名密钥。
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$Tag,
    [string]$Repo = 'ddd-online/Transactions-Viva',
    [switch]$Draft,
    [switch]$SkipClean
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    if ($Tag -notmatch '^v\d+\.\d+\.\d+$') {
        throw "Tag 必须形如 v1.0.0: $Tag"
    }
    $version = $Tag.Substring(1)

    $toml = Get-Content (Join-Path $root 'viva.toml') -Raw
    $match = [regex]::Match($toml, '(?m)^\s*version\s*=\s*"([^"]+)"')
    if (-not $match.Success) { throw 'viva.toml 没有 [app] version' }
    if ($match.Groups[1].Value -ne $version) {
        throw "viva.toml 版本 $($match.Groups[1].Value) 与 Tag $Tag 不一致"
    }

    $status = git status --porcelain
    if ($status) {
        throw "工作树不干净，先提交再发布：`n$status"
    }

    $tagCommit = (git rev-parse "$Tag^{commit}" 2>$null)
    if ($LASTEXITCODE -ne 0) { throw "本地没有标签 $Tag" }
    $head = (git rev-parse HEAD 2>$null)
    if ($tagCommit.Trim() -ne $head.Trim()) {
        throw "标签 $Tag 不在 HEAD 上；先发布最新提交或把标签移到当前提交"
    }

    $setup = Join-Path $root "build\output\Transactions-$version-setup.exe"
    if (-not (Test-Path -LiteralPath $setup)) {
        throw "缺少发布产物: $setup（先运行 build/build.ps1）"
    }

    $notes = Join-Path $root 'build\release_notes.md'
    if (-not (Test-Path -LiteralPath $notes)) { throw "缺少发布说明: $notes" }

    # Release 正文只能是本次版本这一节：应用内「关于 / 更新」显示的就是 release 正文，
    # 历史条目留在各自版本的 release 里。这里再切一次，文件里攒了历史也不会整份发出去。
    $allNotes = Get-Content -LiteralPath $notes -Raw
    $allSections = [regex]::Matches($allNotes, '(?m)^##\s*\[')
    if ($allSections.Count -ne 1) {
        throw "release_notes.md 只能有当前版本一节，实际 $($allSections.Count) 节"
    }
    $sectionPattern = '(?ms)^##\s*\[' + [regex]::Escape($version) + '\]\s*-\s*\d{4}-\d{2}-\d{2}[^\n]*\n.*?(?=^##\s*\[|\z)'
    $section = [regex]::Match($allNotes, $sectionPattern)
    if (-not $section.Success) {
        throw "发布说明里没有 $version 这一节: $notes"
    }
    $releaseBody = Join-Path ([System.IO.Path]::GetTempPath()) "transactions-release-$version.md"
    $utf8 = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($releaseBody, $section.Value.TrimEnd() + "`n", $utf8)

    Write-Host '[release] 校验产物摘要'
    $hash = (Get-FileHash -LiteralPath $setup -Algorithm SHA256).Hash.ToLowerInvariant()
    Write-Host ("  {0} sha256={1}" -f (Split-Path -Leaf $setup), $hash)
    Write-Host ("[release] 正文只含 {0} 一节（{1} 行）" -f $version, ($section.Value.TrimEnd() -split "`n").Count)

    $arguments = @(
        'release', 'create', $Tag,
        $setup,
        '--repo', $Repo,
        '--title', "Transactions $version",
        '--notes-file', $releaseBody,
        '--latest'
    )
    if ($Draft) { $arguments += '--draft' }

    Write-Host "[release] gh $($arguments -join ' ')"
    gh @arguments
    if ($LASTEXITCODE -ne 0) { throw "gh release create 失败（exit $LASTEXITCODE）" }

    if (-not $SkipClean) {
        Write-Host '[release] 清理本地发布产物（build/output、build/release）'
        & (Join-Path $PSScriptRoot 'clean.ps1')
    }

    Write-Host "[release] 完成：https://github.com/$Repo/releases/tag/$Tag"
}
finally {
    Pop-Location
}
