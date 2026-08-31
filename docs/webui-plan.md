# tdl_webui WebUI 开发计划

> Fork 仓库: <https://github.com/ArisMaid/tdl_webui>
> 目标: 为 tdl (Telegram Downloader) 增加一个 WebUI 入口，方便用户在浏览器中快速调整参数并执行下载、上传、转发等任务。

## 1. 背景与目标

tdl 是一个功能强大的 Telegram 下载器 CLI，但所有操作都依赖命令行参数与交互式终端。本计划为其增加 **tdl webui 子命令**：启动一个内嵌静态页面的 HTTP 服务，用户在浏览器中即可完成参数调整、任务启动、进度查看、日志监控与账号登录，无需记忆任何 CLI 参数。

### 1.1 设计原则

1. **复用而非重写**：tdl 的 app 层深度依赖全局 viper 配置与终端交互（survey），直接在进程内调用风险高、耦合重。WebUI 通过**自执行子进程**（tdl <subcommand>）完成任务执行，天然获得与 CLI 完全一致的能力、参数校验与行为。
2. **零外部依赖**：后端仅使用 Go 标准库 (net/http) 与项目已有依赖；前端为单文件静态页面（原生 JS + CSS，无构建步骤），通过 go:embed 嵌入二进制，保持单文件启动的优良传统。
3. **安全优先**：Web 服务默认监听 localhost；支持 --token 认证（Bearer Token），Docker 场景允许 --token= 显式关闭（依赖容器网络隔离）。
4. **任务隔离**：每个任务是一个独立的 tdl 子进程，可独立取消；服务端崩溃不影响正在执行的任务。
5. **行为可预期**：子进程统一加 --disable-progress-ps 与 NO_COLOR=1 环境变量，输出为可解析的行式文本，便于 Web 端流式展示。

## 2. 总体架构

浏览器 (WebUI 单文件 index.html，原生 JS + CSS)
  - 参数表单 (下载/上传/转发/登录/聊天列表)
  - 任务列表 + 进度条
  - SSE 实时日志流
        | HTTP (REST + SSE)
tdl webui (Go 服务)
  - /api/*   REST API (任务 CRUD)
  - /api/events SSE 事件流
  - TaskManager: 内存任务表 + 环形日志缓冲
  - 子进程执行器: os.Executable() 自执行
        | exec (stdio 管道)
tdl <download|upload|...> 与 CLI 完全一致的执行逻辑 (独立进程)

## 3. 功能范围

### 3.1 任务类型（v0.1.0）

| 任务 | 对应子进程命令 | 支持参数 |
|------|---------------|---------|
| 下载 | tdl download | urls/files、模板、目录、包含/排除扩展名、rewrite-ext、skip-same、desc、takeout、group、continue/restart |
| 上传 | tdl upload | 目标 chat/topic、路径、包含/排除扩展名、rm、photo、caption |
| 转发 | tdl forward | from、to、edit、mode、silent、dry-run、single、desc |
| 聊天列表 | tdl chat ls -o json | filter 表达式 |
| 登录(QR) | tdl login -T qr | QR 文本化输出 → 浏览器端渲染二维码 |

全局参数：namespace、proxy、threads、limit、pool、delay、ntp、reconnect-timeout、debug。

### 3.2 后续版本（backlog）

- 登录(code/desktop) 交互式任务、聊天导出(chat export)任务、serve 模式
- 任务持久化（服务重启恢复）
- WebSocket 替代 SSE、多用户与权限体系
- 移动端自适应与主题切换

## 4. 后端设计

### 4.1 目录结构

- cmd/webui.go            # cobra 命令: tdl webui
- app/webui/webui.go      # 服务入口、路由注册
- app/webui/task.go       # TaskManager、Task、环形日志缓冲
- app/webui/exec.go       # 子进程执行器、命令构建、取消
- app/webui/api.go        # REST handlers
- app/webui/sse.go        # SSE 事件流
- app/webui/index.html    # 前端单文件（go:embed）

### 4.2 REST API

| Method | Path | 说明 |
|--------|------|------|
| GET  | /api/info | 版本、认证方式、全局配置默认值 |
| GET  | /api/chats | 拉取聊天列表（执行 chat ls -o json 并解析） |
| POST | /api/tasks | 创建任务 {type, options, global} |
| GET  | /api/tasks | 任务列表（含日志尾部） |
| GET  | /api/tasks/{id} | 任务详情 |
| DELETE | /api/tasks/{id} | 取消并删除任务 |
| GET  | /api/events | SSE 流：任务状态/进度/日志事件 |
| POST | /api/login/qr | 启动 QR 登录任务，解析登录链接返回 |

任务状态机：pending → running → succeeded | failed | canceled。

### 4.3 执行模型

- 每个任务一个 exec.Cmd，stdout/stderr 合并管道，逐行（按 CR 分割以支持进度行）写入任务日志环形缓冲（每个任务保留最近 2000 行）。
- 进度解析：从输出行中提取常见进度模式（百分比、速度、ETA 等），尽力而为提取并展示在任务卡片上。
- 取消：优先 cmd.Process.Kill()，并设置任务状态为 canceled。
- 进程环境：NO_COLOR=1，stdin 指向 os.DevNull 以禁用 survey 交互（交互类任务通过专用端点处理）。

## 5. 前端设计

- 单文件 index.html：CSS 变量主题（暗色）、响应式布局。
- 页面分区：
  1. 顶栏：服务信息、token 设置（localStorage）、SSE 连接状态。
  2. 任务区：新建任务表单（按任务类型动态渲染参数）、任务卡片列表（状态徽章、进度条、速度/ETA、实时日志折叠面板、取消按钮）。
  3. 账号区：登录状态查询、QR 登录（浏览器渲染二维码 + 登录链接）、聊天列表浏览/搜索。
  4. 设置区：全局参数（namespace、proxy、threads 等）。
- 前端通过 fetch + EventSource 与后端交互，所有交互无需刷新页面。

## 6. 安全

- 默认 127.0.0.1:8080；--host 允许显式绑定其他地址并输出安全警告。
- --token 开启 Bearer 认证（无 token 时默认允许，localhost 场景）；Docker 镜像 ENTRYPOINT ["tdl","webui"]，用户可通过环境变量 TDL_WEBUI_TOKEN 配置。
- 子进程参数采用白名单式构建（从不拼接 shell），杜绝命令注入。
- 聊天列表等只读接口限流（复用 chat 的 ratelimit）。

## 7. 测试与验收

1. 单元测试：命令构建（参数白名单/注入测试）、进度行解析、环形缓冲。
2. 集成测试：构建二进制 → 启动 tdl webui → 调用 /api/info、/api/tasks 生命周期（用 tdl version 型内部任务验证执行器）。
3. 端到端（需真实 Telegram 账号，由维护者执行）：
   - QR 登录 → 聊天列表 → 创建下载任务 → 观察进度/日志 → 取消任务 → 文件校验。
4. 回归：go build ./...、go vet ./...、既有 go test ./... 通过。
5. Docker：构建镜像 → 运行 → 端口映射验证页面与 API。

## 8. 版本与发布

- 版本号：fork 首个版本 **v0.1.0**（上游版本为 0.20.x 系列，fork 独立编号避免冲突）。
- Tag：git tag v0.1.0 并推送。
- 镜像：aris-maid/tdl_webui:0.1.0 与 :latest（多阶段构建复用现有 Dockerfile 模式，注入 VERSION/COMMIT 构建参数）。
- 文档：更新 README，补充 WebUI 使用说明与截图。

## 9. 里程碑

| 阶段 | 内容 | 验收标准 |
|------|------|---------|
| M1 | 计划文档 + 骨架代码 | go build 通过 |
| M2 | 任务执行器 + API + SSE | 单元/集成测试通过 |
| M3 | 前端页面 | 浏览器手动验收 |
| M4 | 集成测试 + Docker 镜像 | 镜像运行验收 |
| M5 | 推送 fork + tag + 文档 | 远端可见 |
