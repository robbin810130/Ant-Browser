# Windows 小Q协作手册

## 目标

小Q负责 Windows 侧发布验证，不负责设计服务端架构，也不直接修改生产 stable manifest。

小Q的核心产出是：

- Windows 客户端安装包
- Windows app-update zip
- Windows app-update manifest
- Windows app-update e2e 验证报告
- Windows 真机登录/工作台/授权流程验收结论

## 固定仓库与分支

Ant Browser 仓库：

```text
https://github.com/robbin810130/Ant-Browser.git
```

默认协作分支：

```text
codex/windows-app-update-validation
```

每次开始前必须执行：

```powershell
git fetch origin
git checkout codex/windows-app-update-validation
git pull --ff-only origin codex/windows-app-update-validation
git rev-parse --short HEAD
git status --short
```

如果工作树不干净，先汇报，不要擅自清理未知改动。

## 当前服务器参数

业务服务端：

```text
http://192.168.210.169:4174
```

服务端健康检查：

```powershell
curl.exe -fsS http://192.168.210.169:4174/api/health
curl.exe -fsS http://192.168.210.169:4174/api/client/health
```

Windows 更新包分发：

```text
http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
```

Nginx 静态分发健康检查：

```powershell
curl.exe -fsS http://192.168.210.169:18080/healthz
```

## 小Q职责边界

小Q应该做：

- 拉取最新 Ant Browser 分支。
- 在 Windows 机器上打包。
- 跑 Windows app-update e2e。
- 安装真实客户端，登录页填服务端地址。
- 验证店铺资料、工作台、授权、打开后台。
- 将测试产物和报告反馈给 Robbin / Vera。

小Q不应该做：

- 不直接改服务器 stable manifest。
- 不跳过 e2e 门禁。
- 不删除旧版本 zip、installer、manifest。
- 不改 Mac 签名、公证流程。
- 不把密码、token、cookie 写进日志或提交。

## Windows 打包命令

在 Ant Browser 仓库根目录执行：

```powershell
bat\publish.bat W -Version <target-version>
```

期望产物：

```text
publish\output\AntBrowser-Setup-<target-version>.exe
publish\output\AntBrowser-<target-version>-windows-amd64.zip
publish\output\AntBrowser-<target-version>-windows-amd64.zip.sha256
publish\output\app-update-stable.json
publish\output\app-update-stable.json.sha256
```

如果缺任何一个文件，停止并汇报。

## Windows 自动更新门禁

每次发版必须跑：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tools\app-update\windows-app-update-e2e.ps1 -BaselineVersion <baseline-version> -TargetVersion <target-version>
```

通过标准：

- baseline 安装成功。
- target 打包成功。
- manifest 和 zip 校验成功。
- baseline -> target 自动更新成功。
- `state.json.status = idle`。
- `localAppVersion = <target-version>`。
- 安装目录 `ant-chrome.exe` SHA256 等于 target zip 内 exe SHA256。
- `data/`、`runtime/`、`diagnostics/`、`config.yaml` 保留。
- `lastError.code` 和 `lastError.message` 为空。

失败时必须报告：

- 当前分支和 HEAD。
- BaselineVersion / TargetVersion。
- 失败步骤。
- `state.json` 内容。
- `app-update-debug.log` 关键错误。
- 是否有 `ant-chrome.exe`、`xray.exe`、`sing-box.exe` 残留进程。

## 远端服务端登录验收

安装 target 客户端后，在登录页填写：

```text
http://192.168.210.169:4174
```

必须验证：

| 编号 | 验证项 | 通过标准 |
|---|---|---|
| V1 | 保存服务端地址 | 重启客户端后仍显示同一地址 |
| V2 | 登录打到远端 4174 | 登录成功，无默认 127.0.0.1 不可达错误 |
| V3 | 店铺资料加载 | 店铺资料页有真实店铺数据 |
| V4 | 工作台加载 | 工作台可显示店铺执行状态 |
| V5 | 授权/打开后台 | 至少一个可操作店铺完成授权或打开后台流程 |

如果出现：

```text
ENV-WORKSPACE-HOST-DEFAULT-UNREACHABLE
```

先做三件事：

1. 确认登录页服务端地址不是空，也不是 `127.0.0.1`。
2. 重启客户端，让 Wails backend 重新读取 `server-connection.json`。
3. 检查本机 runtime config 是否存在：

```powershell
Get-ChildItem -Recurse "$env:ProgramData\1688shop-agent\runtime\config" -ErrorAction SilentlyContinue
```

若仍失败，把错误截图、runtime config 路径和日志一起回传。

## 产物上传约定

当前由 Mac/Vera 侧负责上传到服务器。如果需要小Q协助上传，先上传到 test 通道，不直接上 stable。

目标结构：

```text
/opt/1688shop/releases/windows/test/<target-version>/
  app-update.json
  app-update-stable.json
  app-update-stable.json.sha256
  AntBrowser-<target-version>-windows-amd64.zip
  AntBrowser-<target-version>-windows-amd64.zip.sha256
  AntBrowser-Setup-<target-version>.exe
```

稳定发布由 Vera / 服务器侧执行 promote：

```bash
scripts/release/promote-release.sh --root /opt/1688shop/releases --platform windows --version <target-version>
```

## 汇报格式

小Q每次汇报用这个格式：

```text
## Windows 发布验证报告

分支：
HEAD：
BaselineVersion：
TargetVersion：
服务端地址：
Manifest URL：

### 打包产物
- Setup exe: PASS/FAIL，路径，大小
- App update zip: PASS/FAIL，路径，大小
- Manifest: PASS/FAIL，version/channel/url/sha256

### e2e
- Check: PASS/FAIL
- Download: PASS/FAIL
- Apply: PASS/FAIL
- Final state: idle/localAppVersion/lastError
- Data preserved: PASS/FAIL

### 真机验收
- V1 保存服务端地址：PASS/FAIL
- V2 远端登录：PASS/FAIL
- V3 店铺资料：PASS/FAIL
- V4 工作台：PASS/FAIL
- V5 授权/打开后台：PASS/FAIL

### 问题
- 无 / 具体错误

### 结论
- 允许进入 test/stable / 不允许，原因
```

## 小Q收到任务后的第一句话

小Q开始工作前，应先确认：

```text
我会按 docs/release/windows-agent-q-playbook.md 执行。
先拉取 Ant Browser 的 codex/windows-app-update-validation 分支，确认 HEAD，然后跑 Windows 打包和 e2e。
不会直接改 stable manifest，不会删除旧产物。
```

## 关联文档

- `docs/release/agent-operated-cicd.md`
- `docs/desktop/release-handover-2026-05-28-remote-server.md`
- `docs/release/windows-packaging-and-update-runbook.md`（Ant Browser 仓库）
