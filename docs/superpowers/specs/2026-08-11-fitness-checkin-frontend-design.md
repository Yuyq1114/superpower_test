# 健身打卡前端设计

## 1. 目标与范围

本阶段为现有健身打卡 Go 微服务 MVP 增加响应式 Web 前端，使用户可通过浏览器完成核心个人健身流程。前端同时支持手机和桌面，采用行动型 Dashboard 与活力简洁的视觉风格。

首版包含：

- 注册、登录、刷新会话和登出
- 查看、创建、编辑和删除训练计划
- 管理训练日与训练项目
- 完成训练打卡
- 查看打卡历史与连续打卡天数
- 简化录入体重和体脂
- 展示本周训练次数、连续打卡和最近身体数据摘要

首版不包含教练、团队、社交、排行榜、支付、复杂身体趋势分析、服务端渲染或原生移动应用。

## 2. 技术方案

前端使用 React、TypeScript 和 Vite，作为独立应用部署。React Router 管理页面路由，TanStack Query 管理服务端状态与缓存，React Hook Form 和 Zod 管理表单及校验。视觉系统使用 CSS Variables 和 CSS Modules，不引入大型 UI 组件库。

生产与 Kubernetes 环境使用 Nginx 提供静态文件和同源反向代理：

```text
Browser
  |
  v
Frontend Nginx
  |-- /                  -> React SPA
  |-- /api/v1/*          -> api-gateway:8080
  |-- /api-healthz       -> api-gateway:8080/healthz
  `-- /api-readyz        -> api-gateway:8080/readyz
```

Frontend 是浏览器唯一公开入口。Gateway 在 Kubernetes 中改为内部 ClusterIP，避免浏览器跨域访问。开发模式由 Vite dev server 把 `/api/v1` 代理到本地或 Kubernetes Gateway。

不采用把前端构建产物嵌入 Go Gateway 的方案，避免前后端发布耦合；也不采用前端与 API 分离端口的生产方案，避免额外 CORS 和 Cookie 配置。

## 3. 信息架构与页面

### 3.1 导航

移动端底部导航：

```text
首页 | 计划 | 打卡 | 历史 | 我的
```

桌面端固定侧边栏：

```text
Dashboard
训练计划
今日打卡
训练历史
身体数据
```

### 3.2 Dashboard

Dashboard 是登录后的默认页面，视觉焦点是“今日训练”和“立即打卡”。页面包含：

- 今日训练卡片和主操作按钮
- 当前训练计划摘要
- 连续打卡天数
- 本周训练次数
- 最近体重和体脂摘要
- 最近训练记录

### 3.3 训练计划

计划页面支持计划列表及计划创建、编辑和删除。计划详情支持管理训练日，以及为训练日添加、编辑和删除训练项目。训练项目字段与现有 Gateway 契约一致，包括名称、组数、次数、重量和时长。

### 3.4 打卡

打卡页面允许用户选择训练项目、确认日期、填写备注并提交。写请求生成独立的 `Idempotency-Key`，同一次用户操作重试时复用该值，新的用户操作生成新值。提交期间禁用主按钮，成功后显示明确反馈并使历史和 Dashboard 缓存失效。

### 3.5 历史

历史页面按时间倒序展示动作、完成日期、完成时间和备注，支持日期范围筛选，并显示当前连续打卡天数。移动端使用卡片列表，桌面端可使用紧凑列表。

### 3.6 身体数据

身体数据页面提供体重和体脂快速录入、最近记录与简化摘要。首版只展示最近值和轻量趋势，不提供复杂图表、目标预测或医学建议。

### 3.7 认证

认证页面包含登录和注册。未认证用户只能访问认证页面；访问受保护页面时跳转登录页，成功登录后返回原目标页面。

## 4. 认证与安全

Access Token 仅保存在 React 内存状态中，不写入 `localStorage` 或 `sessionStorage`。Refresh Token 由 Gateway 写入 Cookie，并设置：

- `HttpOnly`
- `SameSite=Strict`
- `Path=/api/v1/auth`
- 生产环境 `Secure`

Gateway 的注册、登录和刷新响应只向浏览器 JavaScript 返回 Access Token 及必要的用户信息，不暴露 Refresh Token。

页面首次加载时，前端调用 `/api/v1/auth/refresh` 恢复会话。业务请求返回 401 后，API Client 只尝试刷新一次并重放原请求。多个请求同时返回 401 时共享同一个刷新 Promise，避免 Refresh Token 并发轮换冲突。刷新失败后清除内存认证状态并跳转登录页。

Gateway 的刷新和登出入口校验同源 `Origin`。登出会撤销 Refresh Token 并清除 Cookie。Cookie 属性、可信代理和生产 TLS 配置必须由环境明确提供，不能依赖请求头猜测安全模式。

## 5. 前端模块边界

```text
frontend/src/
  app/                 应用入口、路由、Provider 和布局
  features/auth/       会话恢复、登录、注册和登出
  features/dashboard/  首页聚合展示
  features/plans/      计划、训练日和训练项目
  features/checkins/   打卡提交
  features/history/    历史与连续天数
  features/body-metrics/ 身体数据
  shared/api/          类型化 API Client、错误和刷新协调
  shared/ui/           通用表单、按钮、卡片、反馈和布局组件
  shared/lib/          日期、幂等键和格式化工具
```

每个 feature 只通过 `shared/api` 访问 Gateway，不直接依赖其他 feature 内部实现。Dashboard 通过公开的 query hooks 组合摘要，不复制领域请求逻辑。

## 6. 数据流与状态

```text
React Page
  -> Query / Mutation Hook
  -> Typed API Client
  -> Same-origin /api/v1
  -> Nginx
  -> Go API Gateway
  -> gRPC services
```

TanStack Query 保存远端数据；组件局部状态只保存暂存表单和界面交互。查询键按用户和资源层级组织。计划、打卡和身体数据 mutation 成功后只失效相关查询，不清空整个缓存。

统计服务是最终一致的。Dashboard 在打卡成功后显示“统计更新中”，并使用有上限的短轮询刷新摘要；达到上限后停止自动请求并提供手动重试，不无限轮询。

## 7. 错误与空状态

- 400：显示字段级校验提示
- 401：执行一次会话刷新；失败后返回登录页
- 403：显示无权限
- 404：显示资源不存在
- 409：提示重复操作或幂等键冲突
- 429：提示操作过于频繁
- 503/504：提示服务暂不可用并允许重试
- 网络失败：保留用户输入，不清空表单
- 未知错误：展示统一错误信息和后端 `request_id`

所有列表提供加载、空数据、错误和重试状态。破坏性操作需要二次确认。表单提交期间阻止重复操作，但保留用户取消或返回的明确路径。

## 8. 视觉与响应式

视觉采用浅灰白背景、深灰黑文字与荧光绿或青绿色主强调色。危险操作使用红色。卡片使用轻边框、小幅阴影和适中圆角，突出运动感而不过度装饰。

响应式规则：

- 小于 768px：单列布局、底部导航、主要操作位于拇指易触区域
- 768px 至 1199px：双列 Dashboard 和紧凑侧栏
- 1200px 及以上：固定侧栏、多列卡片和并排详情
- 桌面表格在移动端转换为卡片列表
- 桌面弹窗在移动端转换为底部抽屉
- 主要点击区域不小于 44×44px

所有交互提供可见键盘焦点，颜色对比满足 WCAG AA。状态不能只依赖颜色表达。

## 9. 部署与运行

新增 Frontend 多阶段 Dockerfile：Node.js 阶段构建静态文件，Nginx 阶段运行。运行容器使用非 root 用户、只读根文件系统、最小 Linux capabilities、资源 requests/limits 及 startup/readiness/liveness probes。

Docker Compose 新增 `frontend` 服务并暴露本地浏览器端口。Kubernetes 新增 Frontend Deployment 和 Service；Frontend 成为唯一面向浏览器的 NodePort。Gateway Service 改为 ClusterIP，服务间仍通过 Kubernetes DNS 通信。

Nginx 对 SPA 路由使用 `try_files` 回退到 `index.html`，但 `/api/` 请求不得回退到 HTML。静态资源使用内容哈希和长期缓存，`index.html` 禁止长期缓存。

## 10. 测试策略

- Vitest：格式化、幂等键、API Client 和刷新协调
- React Testing Library：认证、表单、页面状态和错误反馈
- MSW：模拟 Gateway 的 401、409、429、503 和 504
- Playwright：注册、登录、计划、训练日、训练项目、打卡、历史和身体数据主流程
- Axe：认证、Dashboard、计划和打卡页基础可访问性
- 响应式测试：手机和桌面视口均运行关键流程
- Go Gateway 测试：Cookie、Origin 校验、Token 响应裁剪和登出清理
- Kubernetes 验收：Frontend、Gateway 和后端 Pod Ready，并从 Frontend 入口执行完整流程

## 11. 验收标准

- 浏览器打开公开根地址显示 React 页面，不再返回 Gateway 404
- 用户可以完成注册、登录、计划管理、打卡和历史查看
- 用户可以录入体重和体脂并查看简化摘要
- 页面刷新后可通过 HttpOnly Refresh Cookie 安全恢复登录
- Access Token 和 Refresh Token 均不进入浏览器持久存储
- 手机和桌面均可完成核心流程
- 错误提示保留可定位的 `request_id`
- Frontend 容器符合非 root 和只读根文件系统要求
- 普通前端测试、浏览器 E2E、后端回归测试和 Kubernetes 验收全部通过

## 12. 实施前置条件

实施开始前安装 Node.js 当前 LTS 版本及随附 npm，并验证 `node --version` 和 `npm --version`。依赖版本由包管理器在创建前端项目时选择并锁定，不在设计阶段预设虚构版本。

