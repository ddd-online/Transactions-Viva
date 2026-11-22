# 发布流程

发布规范：版本、标签、正文、产物名与清理都由脚本检查，不靠记忆。构建与发布使用 Viva 与 GitHub Release。

**发布不是默认动作。** 日常修改只 commit；只有用户在当次对话里明确要求“发布 / 推送 /
创建 release”时，才执行 push、打标签、创建 GitHub Release。发布前必须完成下方第 3 节的
全量验证。

## 1. 版本

版本只有一个来源：`viva.toml` 的 `[app] version`。`internal/buildinfo.Version` 必须与它相同，
`go test ./internal/buildinfo` 会检查；`viva package` 也用这个版本命名产物。

发布标签必须形如 `vX.Y.Z`，且必须指向当前 `HEAD`。

## 2. 发布说明

`build/release_notes.md` 只放**当前版本这一节**：

```markdown
## [0.1.0] - 2026-11-22

### 新功能

- **小标题**：用户看到的变化。

### 改进

- **小标题**：用户看到的变化。
```

- 第一行版本必须等于 `viva.toml`；日期用 `YYYY-MM-DD`。
- 节名按用户感知选择：`新功能` / `改进` / `修复`；空的节删掉。
- 一项一个变更，以 `**小标题**：` 开头；根因、文件、判据留在提交里。
- 历史版本说明留在各自的 GitHub Release 里，不写进这个文件。
- `build/release_template.md` 是空白骨架，不要改动它来写当次说明。

## 3. 构建

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File build/build.ps1
```

脚本依次运行 `go test ./... -count=1`、`go vet ./...`、`go tool viva vet`，然后调用
`viva package -target windows -arch amd64 -format nsis,zip`。NSIS 默认从
`D:\software\NSIS` 查找，也可以用 `NSIS_HOME` 指定。

成功后生成：

```
build/output/Transactions-X.Y.Z-setup.exe   # 上传的安装器
build/release/transactions-X.Y.Z-windows-amd64-setup.exe
build/release/transactions-X.Y.Z-windows-amd64.zip
```

安装器是用户级安装，不需要管理员权限。生成后先本地安装 / 卸载一次，确认版本与
「关于软件」一致，再发布。

## 4. 发布

确保工作树干净，然后打标签、跑发布脚本：

```powershell
git tag -a vX.Y.Z -m "Transactions X.Y.Z"
git push origin main
git push origin vX.Y.Z

powershell -NoProfile -ExecutionPolicy Bypass -File build/release.ps1 -Tag vX.Y.Z
```

`release.ps1` 会再次校验标签、版本、工作树、安装器、发布说明只有一节，打印 SHA256，
然后调用 `gh release create`。它只上传 `Transactions-X.Y.Z-setup.exe`；不生成
`latest.yml`，应用内更新直接读 release 的 `tag_name` 与 asset 的 `digest`。

需要先看草稿时加 `-Draft`；发布成功后默认清理 `build/output` 与 `build/release`，要保留
现场加 `-SkipClean`。

## 5. 发布后

- 打开 GitHub Release，确认只有一个 `.exe` 资产，正文只有当前版本。
- 在已安装的上一版本应用里检查一次更新：应识别新 tag、显示同一份 release 正文，并下载校验安装器。
- 不要修改已发布标签或已上传的安装器；有问题就发下一版。
