# Windows 发布稳定性阶段报告

## 分支

- `codex/windows-phase1-stability`

## 当前状态

- 工作区干净
- 已完成 Windows 启动 Gate 与 runtime 更新链闭环
- 已实现应用本体自更新代码链路与发布产物链路
- 本地分支包含未推送的 app self-update 阶段提交

## 已完成范围

### 1. Windows 启动 Gate Phase 1/2

已完成：

- install root / runtime root / state root / diagnostics root 检查
- `recommendedAction` / `details` 结构化展示
- workspace host 来源分层：
  - `runtime-config`
  - `env:DESKTOP_SERVER_BASE_URL`
  - `config.yaml`
  - `default`
- diagnostics bundle 补齐来源信息

### 2. Windows 更新检查 + 用户确认升级

已完成：

- 更新来源优先级
  - `runtimeDir/config/release-update.json`
  - `DESKTOP_UPDATE_MANIFEST_URL`
  - `config.yaml -> release.update_manifest_url`
- 更新弹窗展示：
  - `manifestSource`
  - `manifestUrl`
- runtime pointer 切换
- manifest 加载失败在弹窗内展示完整错误

### 3. 发布 contract 校验

已完成：

- `tools/runtime/verify-publish-contract.py`
- 接入 Windows / macOS 发布脚本
- 校验：
  - `publish/runtime-manifest.json`
  - `publish/runtime-sources.json`
  - staging 规则
  - installer 规则

### 4. 应用本体自更新

已完成：

- `backend/internal/appupdate` shared core：
  - manifest / source / layout / state / plan
  - download + sha256 + size 校验
  - full zip staging + zip-slip 防护
  - manager 状态机
- Windows Phase 1 backend：
  - 用户态安装目录校验
  - Program Files / 不可写安装拒绝
  - runner copy
  - backup / replace / rollback
  - `--apply-update` / `--post-update-check`
- Wails 后端 API：
  - `CheckDesktopAppUpdate`
  - `DownloadDesktopAppUpdate`
  - `ApplyDesktopAppUpdate`
  - `GetDesktopAppUpdateState`
  - `ClearDesktopAppUpdateFailure`
- 前端：
  - app update store
  - 客户端更新弹窗
  - required / unsupported install 阻断登录恢复
- 发布产物：
  - `publish/output/app-update-stable.json`
  - `publish/output/AntBrowser-<version>-windows-amd64.zip`
  - `tools/app-update/verify-app-update-package.py`
  - 新安装默认目录改为 `%LOCALAPPDATA%\Programs\Ant Browser`

## Windows 真机实证结果

### 启动 Gate

- 首启 pointer 缺失 -> 自动修复 -> 进入 host 检查：PASS
- workspace host 不可达：PASS
- workspace host 可达放行：PASS

### 更新链

#### A. soft update

- 弹窗出现：PASS
- 来源与地址显示：PASS
- “稍后再说”存在：PASS
- 关闭后继续流程：PASS

#### B. required update

- 弹窗出现：PASS
- 来源为 `runtime-config`：PASS
- required 样式与按钮：PASS
- `runtime/current.json` 切到新版本：PASS

#### C. manifest load fail

- 弹窗出现：PASS
- 错误直接显示在弹窗里：PASS
- 包含来源、路径和失败原因：PASS

### 应用本体更新链

本阶段已完成代码与本地测试，尚未完成 Windows 真机端到端回归。

待回归：

- A：soft app update success
- B：required app update success
- C：unsupported install
- D：manifest load fail
- E：checksum mismatch

## 关键提交

- `d48387b Show structured startup diagnostics in runtime gate`
- `a52cc31 Finalize Windows startup gate phase 2`
- `ee9e137 Build verify runtime publish contract`
- `3953611 Add release update manifest sources`
- `bfaab81 Improve update prompt failure feedback`
- `abff3ee Fix soft update prompt visibility`
- `44af442 Show update manifest load errors in modal`
- `4ebc8e0 Preserve Wails string update errors`
- `cb8be52 Design application self-update`
- `a6416fb Plan application self-update implementation`
- `e54d78b Add app update manifest model`
- `998da16 Remove app update source test hook`
- `88373ce Harden app update state persistence`
- `bfef997 Add app update payload staging`
- `ef139b8 Add app update manager skeleton`
- `32fc8e3 Add Windows app update backend`
- `d0ded0a Add app update CLI modes`
- `cb9ea40 Expose desktop app update APIs`
- `eac2fba Add frontend app update prompt`
- `94a34f9 Add app update publish artifacts`

## 明确未完成项

### 1. 还没有正式 Windows 安装包实物验收

当前已具备打包脚本与 contract 校验，但还缺：

- `bat\publish.bat W` 真机打包成功
- `publish/output/AntBrowser-Setup-<version>.exe` 实物产出
- 安装后整链回归

### 2. 还没有应用本体自更新真机验收

当前已经实现代码链路，但还缺 Windows 真机证明：

- 使用真实 `publish/output/app-update-stable.json`
- 使用真实 `AntBrowser-<version>-windows-amd64.zip`
- 验证客户端内下载、替换、重启、post-check
- 验证失败回滚与 unsupported install 文案

## 非阻断残留

- Windows `dev.bat` 尾部噪音 backlog：
  - `backlog/2026-05-15-windows-dev-launcher-follow-ups.md`

该项不阻断当前发布稳定性主线。

## 下一阶段建议

### 先做

1. Windows 正式安装包产出与安装回归
2. 应用本体自更新 Windows 真机 A/B/C/D/E 回归
3. 固化小Q的打包、安装与 app-update 验收流程

### 再做

1. macOS backend 对接同一套 appupdate shared core
2. 增加 release channel / 灰度策略
