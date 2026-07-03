# Maka Browser 截图证据提交修复设计

## 背景

Maka Browser v1.1.26 已能读取真实专员任务及其 SOP。后台链接和处理摘要证据可以提交，但截图证据在客户端请求发出前被遗留的 `unsupportedContract` 分支主动拦截。

服务端 Maka 专员任务合同已经支持 `screenshot`，并校验文件名、MIME、base64 data URL、图片结构和请求大小。因此本次只修复客户端合同接入，不修改服务端和数据库。

## 目标

- 截图经过现有客户端压缩后，按服务端合同正常提交。
- 保持现有图片格式、原图大小、压缩后大小和尺寸限制。
- 后台链接与处理摘要提交行为不回归。
- 提交成功后继续刷新任务详情、列表和统计。

## 非目标

- 不新增独立文件上传或对象存储。
- 不修改服务端证据校验、权限、任务状态或审计逻辑。
- 不修改 `server/data/app.db`。
- 不触发 Maka Browser 新版本发布。

## 设计

### 客户端请求

删除 `submitSpecialistTaskEvidence` 中针对 `screenshot` 的主动拒绝分支。截图继续使用现有载荷：

```json
{
  "stepId": "step-id",
  "evidenceType": "screenshot",
  "payload": {
    "fileName": "evidence.jpg",
    "mimeType": "image/jpeg",
    "size": 123456,
    "dataUrl": "data:image/jpeg;base64,...",
    "note": "可选说明"
  }
}
```

请求仍通过 `DesktopWorkspaceRequest` 代理到：

```text
POST /api/maka/specialist/tasks/:taskId/evidence
```

### 校验边界

客户端保留：

- 仅接受 PNG、JPEG、WebP。
- 原图不超过 8 MB。
- 最长边压缩至不超过 1600 像素。
- 转为 JPEG，压缩后不超过 800 KB。
- 未选图片时禁止提交。

服务端继续负责：

- 证据类型是否属于当前 SOP 允许范围。
- MIME、base64 data URL 与实际图片结构一致性。
- 请求及证据载荷大小限制。
- 任务状态、操作人、店铺权限与审计。

## 错误处理

- 客户端图片读取、格式、压缩失败继续显示字段级错误。
- 服务端拒绝时继续展示服务端返回的明确错误。
- 服务端已接收但详情刷新失败时，继续提示用户手动刷新，不把成功提交误报为失败。

## 测试与验收

1. 先增加失败契约测试，证明截图提交不能再命中客户端主动拒绝。
2. 验证截图请求保留 `fileName`、`mimeType`、`size`、`dataUrl` 和可选 `note`。
3. 运行 specialist presentation smoke。
4. 运行 Maka Browser frontend build。
5. 人工验收时选择截图并提交，确认详情出现截图证据记录。
6. 回归后台链接和处理摘要提交。

