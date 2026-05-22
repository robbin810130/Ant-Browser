# Windows 发布 / 安装 / 更新回归 Runbook

## 目标

这份 runbook 只服务内部发布、真机回归与排障，不面向终端用户。

当前 Windows 线分两层：

1. **安装包 / 首启稳定性**
   - 生成 `publish/output/AntBrowser-Setup-<version>.exe`
   - 安装后首启 Gate、runtime pointer repair、workspace host 检查正常
2. **运行时更新**
   - 启动检查更新
   - `soft / required / manifest load fail` 三类场景可回归
3. **应用本体自更新**
   - 生成 `publish/output/app-update-stable.json`
   - 生成 `publish/output/AntBrowser-<version>-windows-amd64.zip`
   - 客户端内执行下载、hash 校验、staging、runner 替换与重启

## 当前边界

**已完成：**

- runtime 更新检查与弹窗链路
- `runtime/current.json` 切换
- 更新失败在弹窗内展示完整错误
- 应用本体自更新
- 安装包级别的客户端下载、替换与重启
- Windows 新安装默认使用 `%LOCALAPPDATA%\Programs\Ant Browser`

结论：

- 现在可以做到“更新运行时资源”
- 现在也具备“更新客户端本体”的代码与发布产物链路
- 真机发布前仍必须跑完本 runbook 的应用本体自更新回归场景

## 发布前检查

Windows 打包前，操作者应先确认：

1. 当前分支已对齐目标发布分支
2. 工作区干净，或至少没有会污染发布产物的临时改动
3. 已安装：
   - `node`
   - `npm`
   - `go`
   - `wails`
   - `python3`
   - `NSIS / makensis.exe`
4. 仓库中以下文件存在：
   - `publish/runtime-manifest.json`
   - `publish/runtime-sources.json`
   - `bin/xray.exe`
   - `bin/sing-box.exe`
5. 如要把浏览器内核一并打包，`chrome/` 目录下存在可识别的 Windows `chrome.exe`

## Windows 打包命令

交互式：

```bat
bat\publish.bat
```

非交互：

```bat
bat\publish.bat W
bat\publish.bat W -Version 1.1.0
```

## 打包脚本实际会做什么

`bat/publish.ps1` 会按这个顺序执行：

1. 校验版本号
2. 检测 NSIS (`makensis.exe`)
3. 运行 `python3 tools/runtime/verify-publish-contract.py`
4. 校验 `publish/runtime-manifest.json` 对应的 runtime hash
5. 执行 `wails build`
6. 组装 `publish/staging/`
7. 调用 `publish/installer.nsi`
8. 输出 `publish/output/AntBrowser-Setup-<version>.exe`
9. 生成应用本体更新 zip 与 manifest
10. 运行 `tools/app-update/verify-app-update-package.py`
11. 清理 `publish/staging/`

## Windows 打包成功判定

同时满足下面条件，才算打包成功：

1. `bat\publish.bat W` 退出码为 `0`
2. 控制台出现：
   - `发布契约校验通过`
   - `运行时哈希校验通过`
   - `Windows 安装包生成成功`
   - `应用本体更新包生成成功`
3. 产物存在：
   - `publish/output/AntBrowser-Setup-<version>.exe`
   - `publish/output/app-update-stable.json`
   - `publish/output/app-update-stable.json.sha256`
   - `publish/output/AntBrowser-<version>-windows-amd64.zip`
   - `publish/output/AntBrowser-<version>-windows-amd64.zip.sha256`

## Windows 真机安装回归

### 场景 1：安装包能正常安装

检查项：

1. 安装程序可启动
2. 安装流程可完成
3. 新安装默认目录为 `%LOCALAPPDATA%\Programs\Ant Browser`
4. 安装目录存在：
   - `ant-chrome.exe`
   - `publish/runtime-manifest.json`
   - `publish/runtime-sources.json`

### 场景 2：首启 pointer 缺失

预期：

1. 首启 Gate 先卡在 pointer 缺失
2. 点击“自动修复”后生成 `runtime/current.json`
3. 然后进入 workspace host 连通性检查

### 场景 3：workspace host 不可达

预期：

1. Gate blocked
2. 错误码按来源分层
3. details / recommendedAction 正确展示

### 场景 4：workspace host 可达

预期：

1. Gate 放行
2. 应用进入登录页或后续正常流程

## 更新回归场景

本节只覆盖 runtime/resource 更新，不替换 `ant-chrome.exe`。

### A：soft update

构造规则：

- `remote appVersion > local appVersion`
- `minimumResourceVersion == 当前 runtime resourceVersion`

预期：

1. 弹窗出现
2. 显示正确 `manifestSource / manifestUrl`
3. 有“稍后再说”
4. 关闭后继续进入正常流程

### B：required update

构造规则：

- `minimumResourceVersion > 当前 runtime resourceVersion`
- manifest 中的 package path 必须指向真实 payload

预期：

1. 弹窗显示 required 样式
2. 没有“稍后再说”
3. 点击“立即更新并继续”后成功切换 `runtime/current.json`

### C：manifest load fail

构造规则：

- 设置一个不存在的 manifest 路径

预期：

1. 弹窗出现
2. 错误直接显示在弹窗里
3. 文案里能看到：
   - 来源（如 `env:DESKTOP_UPDATE_MANIFEST_URL`）
    - manifest 路径
    - 文件不存在 / load failed 信息

## 应用本体自更新回归场景

应用本体更新与 runtime 更新分开验证。runtime 更新只切换 `runtime/current.json`；应用本体更新会替换用户态安装目录中的 `ant-chrome.exe` 与随包 payload。

### 前置条件

1. 新安装默认目录为 `%LOCALAPPDATA%\Programs\Ant Browser`
2. `publish/output/app-update-stable.json` 存在
3. `publish/output/AntBrowser-<version>-windows-amd64.zip` 存在
4. manifest 中的 `sha256` 与 zip 文件一致
5. 执行以下命令通过：

```powershell
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json publish/output/AntBrowser-<version>-windows-amd64.zip windows-amd64
```

### A：soft app update success

构造规则：

- 本地 app version 小于 manifest `version`
- 本地 app version 大于等于 manifest `minimumAppVersion`
- payload 为 `payloadType: full`

预期：

1. 弹窗显示客户端更新
2. 用户可稍后处理
3. 点击更新后应用先下载、校验并 staging
4. 应用退出，runner 替换用户态安装目录
5. 应用自动重启
6. 新版本号等于 manifest `version`
7. `stateRoot/app-update/state.json` 最终为 `succeeded`

### B：required app update success

构造规则：

- 本地 app version 小于 manifest `minimumAppVersion`
- manifest `version` 大于本地 app version

预期：

1. 弹窗显示 required 客户端更新
2. 没有“稍后再说”
3. 登录恢复和主工作台被阻断
4. 点击更新后完成下载、staging、替换与重启
5. 新版本通过 `--post-update-check`

### C：unsupported install

构造规则：

- 当前安装目录位于 `C:\Program Files` 或其他不可写目录
- app-update manifest 指向可用的新版本

预期：

1. 弹窗显示当前安装位置不支持自动更新
2. 不执行下载与替换
3. 提示迁移到用户态安装目录
4. 诊断包包含：
   - `appUpdateRoot`
   - `appUpdateStatePath`
   - `appUpdatePlanPath`

### D：manifest load fail

构造规则：

- 设置 `DESKTOP_APP_UPDATE_MANIFEST_URL` 为不存在的路径

预期：

1. 弹窗显示客户端更新检查失败
2. 错误码为 `APP-UPDATE-MANIFEST-LOAD-FAILED`
3. 不影响 runtime 更新 API 的状态

### E：checksum mismatch

构造规则：

- manifest package `sha256` 与 zip 实际 hash 不一致

预期：

1. 下载后拒绝进入 staging
2. `state.json` 保留错误码 `APP-UPDATE-DOWNLOAD-FAILED`
3. 不启动 apply runner
4. 安装目录没有变化

## macOS Application Self-Update Regression

### Scope

macOS app-update uses the same app-update manifest and shared backend contract as Windows. The platform target must be `darwin-arm64` or `darwin-amd64`.

This phase supports full package updates only. Delta patching and release channel rollout are out of scope.

### Supported Install Location

Supported:

```text
~/Applications/Ant Browser.app
```

Unsupported for automatic update:

```text
/Applications/Ant Browser.app
/System/Applications/...
```

Unsupported installs must return `unsupported_install` and must not delete or replace any bundle files.

### Required Payload Shape

The macOS app-update zip must contain:

```text
Ant Browser.app/
  Contents/
    Info.plist
    MacOS/
      ant-chrome
      publish/runtime-manifest.json
      bin/xray
      bin/sing-box
```

The payload must not contain `data/`, `User Data/`, `.db`, `.sqlite`, or `.sqlite3` files.

### Package Verification

Run:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/AntBrowser-${VERSION}-darwin-arm64.zip" darwin-arm64
```

or:

```bash
VERSION="$(python3 -c 'import json; print(json.load(open("wails.json", encoding="utf-8"))["info"]["productVersion"])')"
python3 tools/app-update/verify-app-update-package.py publish/output/app-update-stable.json "publish/output/AntBrowser-${VERSION}-darwin-amd64.zip" darwin-amd64
```

Expected:

```text
[OK] app update package verified
```

### Regression Matrix

1. Local file manifest smoke test: PASS.
2. HTTP manifest smoke test: PASS.
3. Soft update from `~/Applications/Ant Browser.app`: covered by shared UI path and non-required prompt behavior; no separate manual pass in this phase.
4. Required update from `~/Applications/Ant Browser.app`: PASS.
5. Unsupported install at `/Applications/Ant Browser.app`: PASS by backend rejection regression.
6. Checksum mismatch: PASS by macOS target regression.
7. Invalid `.app` payload: PASS by payload contract and tampered-stage regression.
8. Replace failure rollback: PASS by Darwin backend rollback regression.
9. Post-check version mismatch rollback/manual-repair: PASS by Darwin post-check regression.
10. Manual repair state after rollback failure: covered by existing manual-repair state path; destructive full manual pass deferred to formal distribution readiness if needed.

### Real Manual Regression Evidence

Latest real macOS manual regression:

- Date: 2026-05-22
- Report: `docs/reports/2026-05-22-macos-app-update-manual-regression.md`
- Phase closeout: `docs/reports/2026-05-22-cross-platform-app-update-phase-closeout.md`
- Baseline: `1.0.0`
- Target: `1.1.0`
- Install shape: user-writable `~/Applications/Ant Browser.app` style sandbox
- Manifest source: runtime config with local `file://` manifest
- UI action: clicked `更新并重启`
- State progression: `verifying -> succeeded -> idle`
- Relaunched UI version: `1.1.0`
- Installed binary hash matched the `1.1.0` artifact
- macOS Chromium Framework symlink preserved in user state:

```text
Resources -> Versions/Current/Resources
```

Notes:

- `idle` after `succeeded` is expected after relaunch, because the new app rechecks the same manifest and finds no pending update.
- This pass does not cover Developer ID signing, notarization, Gatekeeper quarantine, or public distribution hosting.

### Internal macOS Deployment Readiness

For the current internal-only rollout, the goal is to make a small number of trusted Macs install, update, roll back, and verify versions reliably. Formal distribution checks are not required for this rollout.

Internal rollout checklist:

1. Install to a user-writable location, preferably:

```text
~/Applications/Ant Browser.app
```

2. Keep `/Applications/Ant Browser.app` unsupported for automatic updates.
3. Point `DESKTOP_APP_UPDATE_MANIFEST_URL` or runtime config at the internal manifest.
4. Run one update from the internal manifest and payload.
5. Confirm the UI client version after relaunch.
6. Confirm `Contents/Info.plist` reports the expected version.
7. Confirm the installed `Contents/MacOS/ant-chrome` hash matches the intended artifact.
8. Confirm failed updates leave a readable `state.json` under the user state root.
9. Keep cleanup commands for old build artifacts and `/private/tmp` smoke/regression sandboxes.

### Release Readiness Checks

Before distributing a macOS release candidate:

1. Confirm the app bundle launches before packaging.
2. Confirm the app-update verifier passes for the target.
3. Confirm signing status for the release candidate.
4. Confirm notarization status for the release candidate.
5. Confirm Gatekeeper and quarantine behavior for the distributed artifact.

Signing, notarization, and Gatekeeper checks are release readiness checks. They are not runtime backend gates in this phase.

## 小Q 的 Windows 安装包任务

在 Windows 真机上，下一步只做这条线：

1. 拉取 `codex/windows-phase1-stability`
2. 执行：

```powershell
git fetch --all
git checkout codex/windows-phase1-stability
git pull --ff-only
bat\publish.bat W
```

3. 回报：
   - `publish/output/AntBrowser-Setup-<version>.exe` 是否生成
   - 打包完整控制台输出
   - 安装后首启 Gate 表现
   - 是否能放行到登录页

## 常见失败点

### 1. `makensis.exe` 缺失

处理：

- 配置 `MAKENSIS_PATH`
- 或安装 NSIS 到默认目录

### 2. runtime hash 校验失败

处理：

- 检查 `publish/runtime-manifest.json`
- 检查 `publish/` 下实际 payload 是否被人工替换过

### 3. required update 点击后失败

优先检查：

- manifest 的 `path` 是否对应真实 payload
- payload 是否在安装目录 `publish/...` 下存在
- `runtime/current.json` 是否被回滚

### 4. 更新检查失败只显示通用文案

已在提交 `4ebc8e0 Preserve Wails string update errors` 修复。

若再复发，优先检查：

- 前端是否跑的是旧 bundle
- Wails 绑定是否重新生成
