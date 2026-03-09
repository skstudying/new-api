# my-api → new-api 合并日志

## 仓库信息

| 角色 | 仓库 | 说明 |
|------|------|------|
| **origin** | `skstudying/new-api` | 当前主仓库，持续更新 |
| **upstream** | `QuantumNous/new-api` | 上游源仓库 |
| **myapi** | `skstudying/my-api` | 曾在某节点分叉独立开发的仓库 |

- **共同祖先**: `62b796fa` — `feat: /v1/chat/completion -> /v1/response (#2629)`
- **my-api 独有 commit**: 43 个（含 5 个可跳过的 merge/已覆盖 commit）

---

## my-api 独有 Commit 分类

### 🔴 组 A：火山引擎视频通道 — 5 commits

| Hash | 日期 | 描述 |
|------|------|------|
| `cfd2a4fa` | 2025-09-10 | add volc-video-channel (初始) |
| `311c7c93` | 2025-09-11 | complete volc-video channel |
| `f1b1ed80` | 2026-01-11 | volc-api init |
| `85f3fe83` | 2026-02-06 | add volc seedanc-1-5-pro |
| `f45e2710` | 2026-02-06 | fix volc video task model name map |

> new-api 有火山引擎基础 API 支持，但无视频通道。

### 🔴 组 B：Gemini/VertexAI 增强 — 6 commits

| Hash | 日期 | 描述 |
|------|------|------|
| `1e393f46` | 2025-12-21 | Gemini embedding 请求处理 |
| `ae489770` | 2025-12-22 | 添加 CachedContentTokenCount |
| `8d98d1a8` | 2025-12-22 | ConvertEmbeddingRequest 处理 |
| `70fd738b` | 2025-12-23 | fix mass/llama URL on global region |
| `70bfc17f` | 2026-02-20 | gemini/vertexai 图片 token 缓存 |
| `c625a51d` | 2026-02-20 | 补充 Gemini 非流式 CandidatesTokensDetails |

> **分析结论**：当前 new-api 缓存计费已正常工作，无需合并。

### 🟡 组 C：图片输出计费 — 2 commits

| Hash | 日期 | 描述 |
|------|------|------|
| `6a3f8b10` | 2025-12-25 | feat: add image completion ratio |
| `2ae56ad8` | 2025-12-29 | feat: openai /v1/images 图片输出 token 计费 |

> 分离文字/图片输出 token 的倍率，实现按不同倍率计费。

### 🔴 组 D：Replicate2 通道 (img2img) — 5 commits

| Hash | 日期 | 描述 |
|------|------|------|
| `a7892e52` | 2026-01-13 | add Replicate2 channel |
| `23dc3186` | 2026-01-13 | fix compile error |
| `4fa6ffed` | 2026-01-13 | remove task_video gin |
| `88d645fb` | 2026-01-13 | update front & change DIY channel id |
| `3ccf08e3` | 2026-01-13 | fix replicate2 http 201 code |

> new-api 无 Replicate2 通道。

### 🔴 组 E：xAI 图片/视频适配器 — 16 commits

| Hash | 日期 | 描述 |
|------|------|------|
| `934c6303` | 2026-02-20 | add xAI image/video adaptor |
| `6c46257d` | 2026-02-20 | xAI grok-imagine 多项修复 |
| `3b88aa70` | 2026-02-20 | xAI video unknown status, 价格调整 |
| `9c88c1f6` | 2026-02-20 | xAI video polling 改进 |
| `bc5c75ab` | 2026-02-21 | xAI video poll HTTP 检查 |
| `e358474e` | 2026-02-21 | xAI video poll URL double /v1 |
| `0b2f7012` | 2026-02-21 | support xAI /v1/videos/edits |
| `d1a1b498` | 2026-02-21 | video edit default 8s |
| `9e52c61f` | 2026-02-21 | video edit 计费预收+退款 |
| `ea4e4461` | 2026-02-21 | video edit action override |
| `78fb90e0` | 2026-02-21 | video edit quota log |
| `aa5aa410` | 2026-02-21 | xAI task_video pricing |
| `00fc7467` | 2026-02-22 | xAI video task 可靠性 |
| `5b2134b2` | 2026-02-22 | revert PrivateData.Key save |
| `96c2c37b` | 2026-02-22 | 优化 xAI video 计费显示 |
| `1b262361` | 2026-02-23 | xAI video 内容审核计费 |

> new-api 仅有 xAI `/v1/responses`，无图片/视频适配器。

### 🟡 组 F：xAI 内容违规惩罚 — 1 commit

| Hash | 日期 | 描述 |
|------|------|------|
| `a6434ad0` | 2026-01-14 | xai content violation penalty + group ratio |

> new-api 已有 `grok Usage Guidelines Violation Fee`，需对比后决定。

### 🟢 组 G：Token 配额 USD 显示 — 1 commit ⚠️ 建议跳过

| Hash | 日期 | 描述 |
|------|------|------|
| `3e9a363d` | 2026-01-14 | change token quota to USD |

> new-api 已有更完善的 USD/CNY/TOKENS 显示系统。

### 🟡 组 H：通用计费修复 — 3 commits

| Hash | 日期 | 描述 |
|------|------|------|
| `601e573a` | 2026-02-23 | add task_id to consumption log |
| `3ecbfb22` | 2026-02-24 | video edit model_price (-1) 修复 |
| `3731856c` | 2026-02-25 | fallback model_price when -1 |

### 🟢 组 I：视频请求格式修复 — 1 commit

| Hash | 日期 | 描述 |
|------|------|------|
| `c62ac4ea` | 2026-03-03 | 视频生成 image 字段支持 object 格式 |

### 🟡 组 J：UI/杂项 — 1 commit

| Hash | 日期 | 描述 |
|------|------|------|
| `91f7e322` | 2026-01-14 | remove footer + fix volc image param |

> 仅合并 volc image param 修复部分，footer 受项目保护规则约束。

### ⚪ 可跳过 — 5 commits

| Hash | 原因 |
|------|------|
| `19a31229` | Gemini zero values — 已被 new-api `0e9198e9` 覆盖 |
| `ad0195a5` | upstream merge — new-api 已同步 |
| `572ee2b9` | 分支合并 commit，非功能性 |
| `8c7b155f` | Merge pr-2522 — 功能 commit 已单独列出 |
| `8b491f7f` | Merge pr-2488 — 功能 commit 已单独列出 |

---

## 建议合并顺序

按依赖关系和风险排序：

1. ~~**组 B** (Gemini/VertexAI)~~ → 分析后无需合并
2. **组 C** (图片输出计费) → 独立功能，冲突少
3. **组 A** (火山引擎视频) → 新通道 + 组 J volc 修复
4. **组 D** (Replicate2) → 新通道，独立
5. **组 E** (xAI 图片/视频) → 最大功能组
6. **组 F** (xAI 违规惩罚) → 与已有实现对比
7. **组 H** (通用计费修复) → 小修复，依赖 E
8. **组 I** (视频请求修复) → 小修复
9. ~~**组 G** (Token USD)~~ → 跳过

---

## Update Log

### v0.0.2

**组 E + 组 H + 组 I — xAI 生图生视频完整支持** — 适配 new-api 架构并内嵌计费修复

**Commit**: `be22646d` — feat: xAI image and video generation/edit support with unified task billing

#### 架构适配说明

my-api 的 16 个迭代 commit 使用独立 `controller/task_video.go` 做轮询计费。本次**完全适配 new-api 架构**，使用统一的 `TaskAdaptor` 接口 + `service/task_polling.go`：

| my-api | new-api |
|---|---|
| `ValidateRequestAndSetAction` 含计费 | 拆分为 `ValidateRequestAndSetAction` + `EstimateBilling` |
| `controller/task_video.go` 轮询计费 | `AdjustBillingOnComplete` (成功退差额/失败保留审核费) |
| 直接操作用户余额 | `service.RecalculateTaskQuota` / `RefundTaskQuota` |

#### 后端改动（7 个文件）

- `constant/task.go` — 新增 `TaskActionEdit`
- `relay/relay_adaptor.go` — 注册 xAI task adaptor
- `relay/relay_task.go` — xAI 加入实时查询白名单 + 检测 `/videos/edits` 路由
- `relay/common/relay_info.go` — `TaskInfo` 添加 `Duration` 字段 + `TaskSubmitReq.UnmarshalJSON` 支持 image 对象格式（组 I）
- `relay/channel/xai/adaptor.go` — `ConvertImageRequest` 增加 xAI 特有字段 + 输入图片计费
- `relay/channel/xai/dto.go` — `ImageRequest` 添加 xAI 字段
- `router/video-router.go` — 新增 `/v1/videos/generations` 和 `/v1/videos/edits` 路由
- `service/task_polling.go` — 失败时调用 `AdjustBillingOnComplete`（支持内容审核保留扣费）

#### 新增文件（2 个）

- `relay/channel/task/xai/adaptor.go` — xAI 视频任务适配器
- `relay/channel/task/xai/constants.go` — 模型列表

#### 前端改动（2 个文件）

- `web/src/helpers/render.jsx` — 新增 `renderVideoEditPrice` 和 `renderVideoGenerationPrice`
- `web/src/hooks/usage-logs/useUsageLogsData.jsx` — 视频价格展示 + 内容审核/输入图片展示

#### 核心功能

- **视频生成** — 按秒 × 分辨率倍率计费，支持输入图片附加费
- **视频编辑** — 预扣 8.7s，完成后按实际时长退差额
- **内容审核** — 生成保留扣费，编辑按 8s 计费
- **超时/瞬态/404 处理** — 统一由轮询器处理
- **图片生成增强** — 支持 `aspect_ratio`、`resolution`、`images` 参数

### v0.0.1

**组 C 图片输出计费** — 分离 Gemini 生图模型的文字/图片输出 token 计费

改动摘要：
- `dto/gemini.go` — 新增 `CandidatesTokensDetails`，重命名 `GeminiTokensDetails`
- `dto/openai_response.go` — `OutputTokenDetails` 新增 `ImageTokens`
- `types/price_data.go` — 新增 `ImageOutputRatio`
- `setting/ratio_setting/model_ratio.go` — 新增 `ImageOutputRatio` 读写函数
- `setting/ratio_setting/exposed_cache.go` — 暴露 image/audio ratio 数据
- `relay/helper/price.go` — 传递 `imageOutputRatio`
- `relay/compatible_handler.go` — 拆分计费：文字 `completionRatio` / 图片 `imageOutputRatio`
- `relay/channel/gemini/relay-gemini.go` — 解析 `CandidatesTokensDetails` IMAGE modality
- `relay/channel/openai/relay-openai.go` — 图片生成模式下标记图片输出 token
- `controller/option.go` + `model/option.go` — 新增 `ImageOutputRatio` 配置项
- 前端：`render.jsx` / `useUsageLogsData.jsx` / `ModelRatioSettings.jsx` / `RatioSetting.jsx` — 展示分别计费 + 配置入口

配置示例：`{"gemini-3-pro-image-preview": 60}`
