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

## 当前边界

**已完成：**

- runtime 更新检查与弹窗链路
- `runtime/current.json` 切换
- 更新失败在弹窗内展示完整错误

**未完成：**

- 应用本体自更新
- 安装包级别的客户端下载、替换与重启

结论：

- 现在可以做到“更新运行时资源”
- 还不能宣称“以后所有 bugfix 都能只靠客户端点更新完成”

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
9. 清理 `publish/staging/`

## Windows 打包成功判定

同时满足下面条件，才算打包成功：

1. `bat\publish.bat W` 退出码为 `0`
2. 控制台出现：
   - `发布契约校验通过`
   - `运行时哈希校验通过`
   - `Windows 安装包生成成功`
3. 产物存在：
   - `publish/output/AntBrowser-Setup-<version>.exe`

## Windows 真机安装回归

### 场景 1：安装包能正常安装

检查项：

1. 安装程序可启动
2. 安装流程可完成
3. 安装目录存在：
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
