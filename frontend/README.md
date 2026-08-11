# Fitness Check-in 前端

React + TypeScript + Vite 单页应用，通过同源反向代理访问后端 API Gateway。生产构建打包为独立的 Nginx 静态镜像（`Dockerfile`），Compose 与 Kubernetes 都运行同一个镜像。

## 前置依赖

Node.js LTS（本项目在 Node 24 下开发验证；Dockerfile 使用 `node:lts-alpine`）与 npm。

```bash
npm install
```

## 常用命令

```bash
npm run dev        # 本地 Vite dev server，代理到 vite.config.ts 中配置的后端地址
npm run build       # tsc -b && vite build，产物输出到 dist/
npm run typecheck   # tsc -b --pretty false，不产出文件，仅类型检查
npm run lint        # eslint .
npm run test        # vitest watch 模式
npm run test:run    # vitest run，单元/组件测试（MSW 模拟网络，不需要真实后端）
npm run e2e         # playwright test，见下文
```

根目录 `Makefile` 也提供等价的聚合目标：`make frontend-test`（`test:run` + `typecheck` + `lint`）、`make frontend-build`、`make frontend-e2e`（见下文）。

## 端到端（Playwright）测试

`e2e/` 下的测试针对**真实运行中的完整技术栈**（Compose 或 Kubernetes），不会启动 Vite dev server，也不会自动拉起后端。运行前必须已经有一套完整栈在监听：

- Compose：`http://127.0.0.1:8088`（默认 `FRONTEND_PORT`）
- Kubernetes：`http://127.0.0.1:30080`（frontend Service 的 NodePort）

```bash
# Compose：先在仓库根目录 `make up`，等待服务健康后再运行
PLAYWRIGHT_BASE_URL=http://127.0.0.1:8088 npm run e2e

# Kubernetes：先 `make k8s-up` 并等待 Deployment/StatefulSet 全部 Ready
PLAYWRIGHT_BASE_URL=http://127.0.0.1:30080 npm run e2e
```

或者从仓库根目录使用 `make frontend-e2e`（会强制要求设置 `PLAYWRIGHT_BASE_URL`，避免误用陈旧的默认地址）：

```bash
PLAYWRIGHT_BASE_URL=http://127.0.0.1:8088 make frontend-e2e
```

直接用 `npm run e2e` 而不设置 `PLAYWRIGHT_BASE_URL` 时会回退到 `http://127.0.0.1:8088`，方便本地快速验证 Compose；但 `make frontend-e2e` 始终要求显式传入，避免 CI 或跨环境场景下悄悄用错入口。

首次使用前需要安装 Playwright 的 Chromium：

```bash
npx playwright install chromium
```

测试覆盖：

- `e2e/auth.spec.ts`：注册后仪表盘可见；access token 只存在于内存，`localStorage`/`sessionStorage` 全程为空；`fitness_refresh` 是 `HttpOnly`、`Path=/api/v1/auth` 的 cookie；刷新页面后仅凭该 cookie 恢复会话；登出清除 cookie 并回到登录页；未登录访问受保护路由会带 `returnTo` 跳转登录页。
- `e2e/fitness-flow.spec.ts`：注册 → 创建计划并置为进行中 → 创建今天的训练日与训练项目 → 在打卡页选择计划/训练日/项目完成打卡 → 历史页验证记录与备注 → 个人页保存体重并验证"最新体重" → 首页仪表盘验证当前计划、今日项目、连续天数、最近打卡备注，并等待本周训练统计（依赖 Redis Stream 异步消费，不是同步写入）最终一致。
- `e2e/accessibility.spec.ts`：使用 `@axe-core/playwright` 对登录页、注册页、认证后的首页仪表盘、计划创建表单跑自动化可访问性检查；两个 Playwright project（`desktop`/`mobile`）都要求 `violations` 严格为空数组。

每个需要登录的测试都通过 `e2e/helpers.ts` 的 `registerNewUser` 用 `crypto.randomUUID()` 生成的邮箱注册一个全新账号，测试之间不会互相污染或依赖执行顺序。

## 同源与 Cookie

前端始终通过同一个 origin（Compose 的 `8088` 或 Kubernetes 的 `30080`）同时提供静态资源和 `/api/v1/*` 反向代理，`api-gateway` 不再对外暴露端口。刷新令牌只以 `HttpOnly` cookie（`fitness_refresh`，`Path=/api/v1/auth`）形式存在，前端 JavaScript 无法读取；access token 只保存在内存中，页面刷新后靠该 cookie 换取新的 access token。

## Docker 镜像

```bash
docker build -t fitness/frontend:dev -f frontend/Dockerfile frontend
```

镜像基于 `nginxinc/nginx-unprivileged`，容器启动脚本 `nginx/start-nginx.sh` 会在运行时根据 `/etc/resolv.conf` 与（Kubernetes 下由 Downward API 注入的）`POD_NAMESPACE` 渲染 `nginx/nginx.conf` 中的 DNS resolver 与上游 `api-gateway` 地址，因此同一个镜像无需改动即可在 Compose（短服务名 `api-gateway`）和 Kubernetes（`api-gateway.<namespace>.svc.cluster.local`）之间切换。
