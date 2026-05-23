# ASM 店铺运营闭环设计

日期：2026-05-23
分支：`codex/windows-phase1-stability`
状态：已写入，待用户 review 和 implementation plan

## 背景

当前 Windows 与 macOS 客户端稳定性阶段已按现有产品范围收尾。桌面端发布、安装、运行时更新、应用自更新链路进入维护模式。

下一阶段的产品缺口应回到业务功能。

当前 Ant Browser 桌面端已经具备第一层 1688 Workspace 接入能力：

- 桌面端登录与会话恢复
- 本地 Workspace agent 启动
- 授权店铺同步
- managed profile 自动对账
- 一键打开店铺后台
- 共享凭据更新
- 本机会话验证
- managed instance 启动与打开结果回传

但当前业务界面仍不完整：

- `实例列表` 页面已经展示授权店铺，但信息架构仍像浏览器实例管理
- 最近验证、最近打开字段当前是固定空值
- 批量打开当前只弹 warning toast，没有真实执行
- local agent 已有 `runs/events`，但客户端没有把它作为一等运行证据展示
- ASM 店铺接入后，还没有独立的店铺资料中心
- 单店运营任务在桌面端还没有清晰归属位置

本阶段目标是把桌面端升级成真正的 ASM 店铺运营控制台。

## 产品目标

建立 ASM 店铺运营闭环：

1. 用户能把 ASM 店铺资料作为业务主数据查看。
2. 用户能判断每个店铺在当前桌面设备上是否可执行。
3. 用户能在聚焦的工作台里打开、验证、修复或重试店铺执行动作。
4. 用户能查看单店与跨店铺运营任务，并且不把运营任务混进执行状态表。
5. 用户每次操作都有可追踪的运行证据：状态、事件、失败原因、下一步建议。

每个店铺都必须能回答四个问题：

```text
这家店现在能不能操作？
如果不能，卡在哪里？
用户下一步应该做什么？
这次操作有没有运行证据？
```

## 已确认产品方向

以店铺工作台作为主要执行界面，并由 local agent 的运行证据支撑。

已确认决策：

- 优先走 `A：店铺工作台优先`
- 吸收 `B：任务中心` 作为运行证据层和跨店铺任务层
- 不走只打补丁的轻改造路线
- ASM 店铺资料列表与详情页进入必须做范围
- 店铺资料、执行可用性、运营任务、运行证据必须保持概念分离

## 核心信息架构

下一阶段业务界面包含三个一等模块和一个共享证据层。

### 1. 店铺资料中心

店铺资料中心负责 ASM 店铺业务主数据。

它回答：

- 这是什么店？
- 属于哪个平台和哪个店铺标识？
- ASM 是否已接入？
- 这家店的经营归属和运营上下文是什么？
- 当前执行状态和任务状态摘要如何？

具体路由可在实现阶段按本地风格命名，但建议产品形态是：

```text
/shops
/shops/:shopId
```

列表页应展示：

- 店铺名称
- 平台
- shop id
- ASM 接入状态
- 授权状态
- 负责人或运营人
- 标签
- 主营类目或经营类目
- 数据完整度
- 执行状态摘要
- 未完成运营任务数
- 最近同步时间
- 推荐下一步动作

详情页应拆分为 tab 或区块：

- `基础资料`
- `ASM 接入`
- `执行状态`
- `运营任务`
- `运行记录`
- `诊断`

店铺资料详情页可以链接到工作台执行动作，但不应成为批量执行的主入口。

### 2. 店铺工作台

店铺工作台负责执行可用性和即时修复动作。

它回答：

- 当前桌面端能不能打开这家店？
- 本地 profile 是否已映射？
- 指纹内核是否就绪？
- 共享会话是否 ready？
- 店铺实例是否正在运行？
- 最近一次打开或验证是否失败？
- 下一步修复动作是什么？

当前 `BrowserListPage` 应从实例列表升级为授权店铺工作台。

推荐布局：

- 左侧工作队列
- 中间店铺操作主表
- 右侧详情和修复抽屉

左侧队列：

- 可直接打开
- 待人工验证
- 凭据缺失或疑似过期
- 打开失败
- 当前运行中
- 授权失效或待回收

中间主表：

- 店铺名称
- 平台
- ASM 状态摘要
- 执行状态
- profile / core / session readiness
- 最近打开
- 最近验证
- 最近失败
- 未完成任务数
- 主要推荐动作
- 次级动作

右侧抽屉：

- 健康摘要
- 店铺资料摘要
- 本地 profile 映射
- 共享登录状态
- 最近打开 run
- 最近凭据 run
- run 事件时间线
- 失败诊断
- 下一步推荐动作
- 运营任务摘要

工作台应暴露执行动作：

- 打开店铺后台
- 更新共享凭据
- 本机验证
- 重试最近失败动作
- 查看运行证据
- 打开店铺资料详情
- 创建店铺运营任务

### 3. 运营任务中心

运营任务中心负责跨店铺运营任务。

它回答：

- 今天需要做哪些事？
- 哪些店铺任务在等待、运行、阻塞、失败或完成？
- 哪些任务被凭据或本地执行可用性阻塞？
- 哪些任务需要人工介入？
- 哪些任务可以批量重试？

本阶段应引入任务中心的清晰产品骨架，而不是完整自动化平台。

本阶段任务中心范围：

- 展示运营任务列表
- 按店铺、任务类型、状态、失败原因、阻塞原因筛选
- 展示任务摘要
- 打开任务详情
- 链接到相关店铺资料
- 链接到相关工作台动作
- 在安全时重试失败的执行前置任务

任务中心在本阶段不实现完整选品、铺货或报表生成工作流。

### 4. 运行证据层

运行证据层由店铺资料中心、店铺工作台、运营任务中心共享。

它消费 local agent 的运行数据，包括：

- `/local/runs`
- `/local/runs/:runId`
- `/local/runs/:runId/events`

它应把运行证据规范化成前端模型，用来回答：

- 每个店铺最近一次打开 run
- 每个店铺最近一次验证 run
- 每个店铺最近一次凭据更新 run
- 每个店铺当前活跃 run
- 每个店铺最近失败
- 选中 run 的事件时间线

运行证据应包含：

- run id
- 任务类型
- shop id
- 状态
- 状态标签
- 开始时间
- 结束时间
- profile id
- runtime 元数据
- 失败 code
- 失败 message
- 是否需要人工动作
- challenge 类型
- 事件时间线

## 领域边界

### 店铺资料

业务主数据。

示例字段：

- shop id
- 店铺名称
- platform code
- ASM 接入状态
- 授权状态
- 负责人
- 标签
- 主营类目
- 数据完整度
- 最近 ASM 同步时间

### 授权店铺投影

来自 Workspace 和本地运行态的执行侧投影。

示例字段：

- shared login status
- local profile id
- local instance id
- profile exists
- core ready
- instance running
- reclaim pending

它只能是投影，不能成为店铺业务真相源。

### 运营任务

挂在店铺上的业务工作。

示例：

- 采集店铺商品状态
- 检查机会榜
- 执行铺货前置检查
- 准备选品工作流
- 生成店铺运营摘要

本阶段建立任务界面和生命周期边界，不要求实现未来每一种任务类型。

### Run

由 local agent 或桌面端执行流产生的运行证据。

示例：

- open
- bind
- validate
- diagnose
- retry

Run 不是店铺资料，也不是业务任务。Run 是动作结果的证据。

## 必须做

### ASM 店铺资料列表与详情

新增或重构 ASM 店铺资料界面。

最低行为：

- 展示 ASM 店铺列表
- 展示关键业务资料字段
- 展示 ASM 接入状态和授权状态
- 展示来自工作台/证据层的执行状态摘要
- 展示运营任务摘要
- 打开店铺详情
- 从详情跳转到工作台动作
- 从详情跳转到任务中心

如果后端 ASM profile API 暂时不可用，spec 仍应定义前端与 Wails/client 边界。实现可以先使用 Workspace 提供的字段和明确的 unavailable 状态起步，但不能把 `WorkspaceAuthorizedShop` 默默当成最终 ASM profile 模型。

### 店铺工作台重构

围绕执行可用性重构当前店铺执行页。

最低行为：

- 展示队列数量
- 展示店铺操作主表
- 支持搜索和筛选
- 展示推荐动作
- 展示详情抽屉
- 展示最近打开和最近验证证据
- 保留现有 open、bind、validate 动作
- 把当前 warning-only 的批量打开替换成真实安全批量执行

### Runs And Events 接入

通过 Ant Browser 后端和前端暴露 local agent 运行证据。

最低行为：

- 拉取最近 runs
- 拉取 run 详情
- 拉取 run events
- 按店铺和任务类型推导最近 run
- 推导每个店铺最近失败
- 在详情抽屉渲染时间线
- 在主表渲染最近打开和最近验证

### 失败到修复动作映射

把已知失败状态映射到下一步动作。

最低映射：

- 指纹内核缺失 -> 去内核管理
- 指纹内核不可用 -> 检查内核管理或修复后重试
- shared login not ready -> 更新凭据
- awaiting verification -> 本机验证
- validation failed -> 更新凭据或重试验证
- authorization revoked -> 禁止执行并展示回收状态
- local profile missing -> 刷新/重新对账授权店铺
- workspace agent unavailable -> 展示连接修复指引
- workspace server unreachable -> 展示服务连接状态
- ant runtime unreachable -> 展示运行时修复指引
- unknown failure -> 展示运行证据和诊断导出

### 安全批量操作

为执行动作实现安全批量操作。

最低行为：

- 批量打开 ready 店铺
- 批量验证 eligible 店铺
- 批量重试 failed eligible 店铺
- 对不可执行店铺给出明确跳过原因
- 限制并发
- 展示进度
- 展示结果汇总
- 保留每个店铺的 run 证据

批量行为不能默默尝试授权失效、缺少内核、缺少 profile 或 not-ready 店铺。

### 单店运营任务骨架

在店铺详情抽屉或详情页加入单店运营任务 tab/区块。

最低行为：

- 展示所选店铺的运营任务摘要
- 展示等待中、运行中、阻塞、失败、完成数量
- 展示最近任务行
- 当执行可用性阻塞任务时展示阻塞原因
- 暴露受控的新建任务入口

这个骨架应为后续选品、铺货、采集、报表任务做好位置。

### 全局运营任务中心骨架

新增跨店铺运营任务中心骨架。

最低行为：

- 展示运营任务列表
- 按状态筛选
- 按店铺筛选
- 按阻塞原因筛选
- 打开相关店铺资料
- 打开相关工作台动作
- 展示失败或阻塞原因

这是产品底座，不是本阶段完整自动化调度器。

## 应该做

### 诊断导出

支持导出选中诊断上下文，方便支持和回归。

建议内容：

- 选中店铺资料摘要
- 本地执行投影
- 最近 runs
- 选中 run events
- app version
- platform
- workspace agent health
- ant runtime health
- 相关 failure codes

### 导航整理

重命名并重组导航，让用户先看到业务概念。

建议导航：

- `店铺资料`
- `店铺工作台`
- `运营任务`
- `运行记录`
- `系统维护`

旧的 `指纹浏览器` 分组可以保留给底层工具，例如内核管理、代理池、默认书签、标签、日志、接口文档。

### 状态语言整理

使用面向用户的中文状态标签，同时在详情里保留 raw code。

示例：

- `ready` -> `可执行`
- `awaiting_verification` -> `待人工验证`
- `validation_failed` -> `验证失败`
- `reclaim_pending` -> `授权失效，待回收`
- `ANT_CORE_UNAVAILABLE` -> `指纹内核不可用`

## 可以晚点做

这些事项不应阻塞本阶段：

- 完整选品工作流
- 完整铺货/发布工作流
- AI 店铺经营日报生成
- 高级运营任务排程
- 跨店铺自动化编排
- 长周期任务队列迁移到外部持久化 job 系统
- release channel 或灰度发布
- delta app update

## 数据流

### 店铺资料流

```text
Workspace / ASM source
  -> local workspace agent or desktop backend API boundary
  -> Ant Browser Wails backend
  -> frontend shop profile API
  -> 店铺资料中心
```

前端不能直接查询数据库，也不能绕过 Wails/backend 边界。

### 执行工作台流

```text
Workspace authorized shops
  -> local agent /local/shops
  -> Ant Browser workspace service
  -> managed profile reconcile
  -> shop execution projection
  -> 店铺工作台
```

动作流：

```text
店铺工作台 action
  -> Wails backend
  -> workspace service / local agent
  -> managed instance service
  -> open / bind / validate
  -> run evidence
  -> frontend state refresh
```

### 运行证据流

```text
local agent runs/events
  -> Wails backend run evidence API
  -> frontend run evidence module
  -> workbench table summaries
  -> shop drawer timeline
  -> task center evidence links
```

## API 边界

实现计划应在进一步代码检查后确定准确命名，但目标后端边界是：

```text
WorkspaceShopProfiles()
WorkspaceShopProfile(shopId)
WorkspaceAuthorizedShops()
WorkspaceRuns(query)
WorkspaceRun(runId)
WorkspaceRunEvents(runId)
WorkspaceOpenShop(shopId)
StartDesktopSharedLoginBind(accessToken, shopId)
StartDesktopSharedLoginValidate(accessToken, shopId)
```

已有 Wails API 能复用就复用。新增 API 应保持为 Workspace/local-agent contract 的薄适配层。

## 前端模块边界

建议模块：

```text
frontend/src/modules/shops/
frontend/src/modules/workbench/
frontend/src/modules/operations/
frontend/src/modules/runEvidence/
```

具体目录结构可以在实现阶段跟随本地代码风格。关键是边界清楚：

- `shops` 负责业务资料页和 profile DTO
- `workbench` 负责执行可用性界面和动作
- `operations` 负责运营任务骨架和全局任务视图
- `runEvidence` 负责 run/event 拉取、派生和时间线组件

如果更符合当前代码，现有 `workspace` 模块可以继续作为 transport/integration 层。

## UI 设计要求

UI 应该像运营控制台，不像营销 dashboard。

要求：

- 信息密度高但可读
- 表格行高稳定
- 状态 badge 清晰
- 操作按钮带图标
- 使用详情抽屉承载 drill-down
- 不做卡片套卡片
- 不做装饰性 hero
- 不做单一色系界面
- 不保留只 toast "later" 的假动作
- 不做只有状态、没有下一步动作的页面

主要界面：

- 店铺资料列表
- 店铺资料详情
- 店铺工作台
- 店铺详情抽屉
- 运营任务中心
- 运行证据时间线

## 错误处理

错误必须归类为用户可理解的类别，并提供可执行下一步。

类别：

- workspace server unreachable
- workspace agent unavailable
- ant runtime unavailable
- fingerprint core missing
- fingerprint core unavailable
- local profile missing
- shared login not ready
- manual verification required
- credential update failed
- authorization revoked
- open failed
- report failed
- unknown error

每个类别应提供：

- 简短标签
- 详情说明
- raw code
- 推荐动作
- 是否可重试
- 批量执行时是否应跳过

## 测试策略

后端测试：

- shop profile DTO normalization
- run evidence API adapter
- 按店铺和任务类型推导 run/event
- failure-to-recovery mapping
- batch eligibility 和 skip reasons
- 现有 open/bind/validate 行为保持稳定

前端测试或验证：

- 店铺资料列表覆盖 empty、loading、normal、error 状态
- 店铺资料详情展示 ASM、执行、任务、证据区块
- 工作台筛选和队列数量派生正确
- 最近打开和最近验证来自 run evidence
- 失败映射展示正确下一步动作
- 批量操作汇总包含 success、skipped、failed
- 运营任务中心骨架覆盖 empty、loading、normal、blocked、failed 状态

手动回归：

- 登录和会话恢复
- workspace agent bootstrap
- 授权店铺同步
- 打开 ready 店铺
- 更新凭据
- 本机验证
- 打开失败展示证据
- 缺少内核映射到内核修复动作
- 批量打开跳过不可执行店铺
- 店铺资料详情能跳转到工作台和任务中心

## 非目标

本阶段不做：

- 完整选品系统
- 完整铺货或发布系统
- AI 经营报告生成
- 完整持久化调度器
- 在 Ant Browser 本地状态里存商品或订单业务实体
- 让前端直接访问数据库
- 重设计 release/update 基础设施
- 重启 macOS signing 或 notarization 工作

## 成功标准

本阶段成功的标志：

- ASM 店铺有独立资料列表和详情界面
- 用户能区分店铺业务资料和执行可用性
- 店铺工作台展示真实最近打开和最近验证状态
- warning-only 的批量打开被安全批量执行替换
- 每个 open/bind/validate 失败都能通过 run evidence 查看
- 已知 failure code 映射到具体下一步动作
- 单店运营任务有清晰归属位置
- 跨店铺运营任务有清晰归属位置
- release/update 链路除正常兼容性检查外不被改动
