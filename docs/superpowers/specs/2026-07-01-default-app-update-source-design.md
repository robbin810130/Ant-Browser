# Maka Browser 默认应用更新源修复设计

## 背景

Maka Browser `1.1.23` 在 stable `1.1.24` 已发布后，手工点击“检查更新”仍提示“当前已是最新版本”。

根因是 Windows 发布配置中的 `release.app_update_manifest_url` 为空。客户端没有解析到运行时配置、环境变量或 `config.yaml` 更新源时，会用本地应用清单作为远端清单，因此版本比较结果固定为无更新。

现有 Windows 应用更新 E2E 显式注入 `DESKTOP_APP_UPDATE_MANIFEST_URL`，验证了下载和替换流程，但没有覆盖真实安装包在无外部环境变量时能否发现 stable 更新。

## 目标

- 新安装的 Windows Maka Browser 无需人工配置即可发现 stable 更新。
- 显式运行时配置、环境变量和 `config.yaml` 继续拥有更高优先级。
- “无远端更新源”不能再静默伪装成“当前已是最新版本”。
- Windows 发布验证覆盖真实安装后的默认更新发现能力。
- 不改变服务端业务部署，不引入新的更新服务或依赖。

## 方案选择

采用客户端默认 stable URL 兜底，并同步修复发布配置和测试。

未采用仅修改发布配置的方案，因为 NSIS 升级安装会保留已有 `config.yaml`，旧安装仍可能保留空值。未采用仅提供环境变量的方案，因为它只能修复单台机器，不能形成产品级闭环。

## 更新源解析

应用本体更新源按以下顺序解析：

1. 安装运行时目录中的 `config/app-update.json`
2. `DESKTOP_APP_UPDATE_MANIFEST_URL`
3. `config.yaml` 的 `release.app_update_manifest_url`
4. 仅 Windows 客户端内置 stable manifest URL

默认地址：

```text
http://192.168.210.169:18080/releases/windows/stable/app-update-stable.json
```

内置默认值只作为 Windows 最后兜底，不覆盖用户或运维显式配置；macOS 等其他平台没有该默认 fallback。诊断结果必须继续返回实际使用的来源和 URL。

## 安全边界

- 默认 stable URL 仅用于受控内网 HTTP，不得暴露到公网或当作公网发布方案。
- 当前 manifest 没有独立签名；HTTPS 与 Ed25519 签名校验必须另立架构设计后实施。
- 本次对 manifest HTTP 读取加入 15 秒超时、1 MiB 大小上限、最多 3 次同源跳转，并拒绝跨源 redirect。
- 这些传输防护不等于内容真实性校验，不能宣称当前 HTTP 更新链路已经安全。

## 发布配置

`publish/config.init.yaml` 写入同一 stable manifest URL，使安装目录配置具有可读、可诊断的明确值。

客户端代码仍保留默认兜底，以覆盖：

- 从旧版本升级且旧 `config.yaml` 被保留；
- 配置文件缺失；
- 配置字段为空；
- 历史安装目录未经过新安装器初始化。

## 错误处理

- 远端 URL 已解析但请求失败：返回包含来源和 URL 的明确错误，不显示“当前已是最新版本”。
- 远端 manifest 格式或 schema 无效：保持现有校验并返回错误。
- 显式配置存在但不可用：不得静默回退到内置地址，避免掩盖运维配置错误。
- 只有成功加载远端 manifest 并完成版本比较后，才允许返回无更新。

## 测试

### Go 单元测试

- Windows 无运行时配置、环境变量和配置值时，解析到内置 stable URL。
- Darwin 无显式配置时，不解析到 Windows 内置 stable URL。
- 运行时配置覆盖内置 URL。
- 环境变量覆盖配置和内置 URL。
- `config.yaml` 覆盖内置 URL。
- 显式空值按未配置处理。

### 发布契约测试

- `publish/config.init.yaml` 必须包含 stable manifest URL。
- Windows staging 的 `config.yaml` 必须保留该值。

### Windows 应用更新 E2E

- 不再通过用户级环境变量为真实安装强制注入更新源。
- 验证安装后的客户端默认能解析 stable 更新源并发现目标版本。
- 保留现有 Check、Download、Apply、版本切换、数据保留和回滚验证。

## 发布与恢复

修复版本使用 `1.1.25`：

1. 合并修复分支。
2. Windows Release Factory 从 `1.1.24` 基线构建并验证 `1.1.25`。
3. 测试通过后推广 stable。
4. 现有 `1.1.23` 因缺少更新源，需要执行一次 PowerShell 用户环境变量配置或手工安装修复版。
5. 一次性配置必须使用 stable manifest URL；升级完成后可保留该配置，后续仍由相同 stable 通道更新。

不得修改或重新上传已经发布的 `1.1.24` 产物。
