# 运维环境轻量化自动发布系统 实施计划

> 关联 Spec：`spec/ops-deploy-agent/spec.md`
> 创建日期：2026-04-05
> 状态：已确认

---

## 1. 技术方案概述

整个系统使用 **Go** 开发，编译产出两个独立二进制：`ops-server`（控制端）和 `ops-agent`（发布节点），共享同一个 Go 模块。

**Server** 是一个 HTTP + WebSocket 服务，负责：静态 Web 页面服务、Jar 包上传与存储、发布任务调度、通过 WebSocket 向 Agent 下发指令、接收并持久化发布结果。发布记录以 JSON 文件落盘，Jar 包直接存储在 Server 本地文件系统目录。

**Agent** 运行在设备 A，启动后主动与 Server 建立 WebSocket 长连接（断线自动重连），连接时发送握手信息（携带本机管理的设备列表）。接收到发布指令后，串行执行：HTTP 下载 Jar → SCP 分发到各目标设备 → SSH 串行触发每台设备的替换流程（停服/校验/替换/启服/校验）→ 汇总结果写本地日志并上报 Server。Agent 通过 `nohup` 手动启动。

前端为纯 HTML + 原生 JS，由 Server 静态文件服务托管，包含三个功能区：Jar 包管理、发布操作、历史记录查询。

---

## 2. 技术决策

### 2.1 方案选型

| 决策项 | 选择 | 理由 |
|--------|------|------|
| 编程语言 | Go | 单二进制部署，无运行时依赖，交叉编译方便，适合运维工具 |
| Agent-Server 通信 | WebSocket（`gorilla/websocket`） | 支持 Server 主动推送，Agent 在内网可出站连接 |
| SSH/SCP 执行 | `os/exec`（系统 `ssh`/`scp` 命令） | 统一走系统命令，完整复用 `~/.ssh/config`、ssh-agent、任意密钥算法（ed25519/ecdsa/rsa），无需在 Go 层重新实现密钥加载和 ssh config 解析；避免两套认证路径导致"分发成功但 SSH 失败"的排查困难 |
| 发布记录存储 | JSON 文件（`records.json`，追加写入） | 无需数据库，轻量，满足测试环境查询需求 |
| Jar 包存储 | Server 本地文件系统目录 | 简单直接，无存储上限限制 |
| 前端 | 纯 HTML + 原生 JS | 零构建依赖，由 Go `embed.FS` 打包进 binary，无需额外部署步骤 |
| HTTP 路由 | Go 标准库 `net/http` | 无需引入框架，功能简单，依赖最少 |

### 2.2 外部依赖

| 库 | 用途 | 使用方 |
|----|------|--------|
| `github.com/gorilla/websocket` | WebSocket 服务端与客户端 | Server + Agent |
| Go 标准库（`net/http`、`os/exec`、`os`、`encoding/json`、`log`） | HTTP 服务、ssh/scp 命令执行、文件 I/O、JSON 序列化、日志 | Server + Agent |

> **无需引入 `golang.org/x/crypto/ssh`**：所有远程操作（SCP 分发、SSH 停服/启服/进程校验）均通过 `os/exec` 调用系统 `scp`/`ssh` 命令执行，认证链路完全统一。

### 2.3 内部依赖

- Agent 依赖 Server 提供的 HTTP 下载接口（`GET /api/jars/download/{filename}`）
- Agent 依赖 Server 提供的 WebSocket 端点（`/ws/agent`）
- Server Web 页面依赖 Server HTTP API（`/api/jars`、`/api/deploy`、`/api/tasks`）
- Agent 的 SCP/SSH 操作依赖设备 A 上已配置好的系统 SSH 免密（`~/.ssh/config` 或 ssh-agent，不在本系统范围内）

> **路径参数约定（P2-1）**：使用 Go 1.22+ 内置路由模式（`{param}`），如 `GET /api/tasks/{task_id}`、`GET /api/jars/download/{filename}`，无需引入第三方路由库。

---

## 3. 工程目录结构

```
ops/
├── go.mod
├── go.sum
├── cmd/
│   ├── server/
│   │   └── main.go              # Server 入口
│   └── agent/
│       └── main.go              # Agent 入口
├── internal/
│   ├── protocol/
│   │   └── messages.go          # WebSocket 消息结构定义（共享）
│   ├── server/
│   │   ├── config/
│   │   │   └── config.go        # Server 配置加载
│   │   ├── store/
│   │   │   ├── jar_store.go     # Jar 包文件管理
│   │   │   └── record_store.go  # 发布记录 JSON 持久化
│   │   ├── deploy/
│   │   │   ├── agent_hub.go     # Agent 连接管理
│   │   │   ├── manager.go       # 发布任务调度与状态机
│   │   │   └── task.go          # Task 数据结构
│   │   └── api/
│   │       ├── jar_handler.go   # Jar 上传/列表接口
│   │       ├── deploy_handler.go# 发布触发/任务查询接口
│   │       └── ws_handler.go    # WebSocket 握手与消息路由
│   └── agent/
│       ├── config/
│       │   └── config.go        # Agent 配置加载
│       ├── ws/
│       │   └── client.go        # WebSocket 客户端（含自动重连）
│       └── deploy/
│           ├── runner.go        # 发布流程编排
│           ├── downloader.go    # Jar 包 HTTP 下载
│           ├── distributor.go   # SCP 分发
│           ├── replacer.go      # SSH 远程替换
│           └── logger.go        # 本地发布日志写入
├── web/
│   └── index.html               # 前端单页面（embed 进 Server binary）
├── config/
│   ├── server.yaml              # Server 配置示例
│   └── agent.yaml               # Agent 配置示例
└── data/                        # 运行时目录（gitignore）
    ├── jars/                    # Jar 包存储
    ├── records.json             # 发布记录
    └── logs/                    # Agent 本地发布日志
```

---

## 4. 任务拆解

### 4.1 共享模块

- [x] **TODO-G1：Go 模块初始化 + WebSocket 消息协议定义**
  - **描述**：初始化 Go 模块（`go mod init`），创建工程目录骨架；在 `internal/protocol/messages.go` 中定义 Server 与 Agent 之间所有 WebSocket 消息的数据结构：
    - `AgentHandshake`：Agent 连接时发送，携带 agent_id 及其管理的设备 ID 列表
    - `DeployInstruction`：Server 下发的发布指令，含 task_id、service_name、jar_name、jar_download_url、目标设备 ID 列表
    - `DeviceResult`：单台设备的执行结果，含 device_id、status（success/failed）、error_msg
    - `TaskReport`：Agent 上报的汇总结果，含 task_id、service_name、jar_name、target_device_ids、start_time、end_time 及各设备 `DeviceResult`；字段需足以在 Server 无内存状态时独立生成完整发布记录
    - 所有消息用 `type` 字段区分（deploy / report / ping / pong）
  - **涉及文件**：`go.mod`、`go.sum`、`internal/protocol/messages.go`、工程目录骨架
  - **依赖**：无
  - **验收标准**：`go build ./...` 通过；协议结构可被 Server 和 Agent 包正常 import

---

### 4.2 Server 模块

- [x] **TODO-S1：Server 配置加载**
  - **描述**：定义 Server 配置文件结构（YAML）并实现加载逻辑。配置项包括：HTTP 监听地址（`listen_addr`）、对外可访问的基础地址（`public_base_url`，用于生成 Agent 可下载的 Jar URL）、Jar 存储目录（`jar_dir`）、发布记录文件路径（`record_file`）、WebSocket Agent 端点路径（`ws_path`）。程序启动时从指定路径（默认 `config/server.yaml`）加载，加载失败则退出并打印错误。
  - **涉及文件**：`internal/server/config/config.go`、`config/server.yaml`、`cmd/server/main.go`
  - **依赖**：TODO-G1
  - **验收标准**：`./ops-server --config config/server.yaml` 启动时能正确读取并打印配置；配置文件缺失时输出明确错误

- [x] **TODO-S2：Jar 包上传与文件管理**
  - **描述**：实现 `jar_store.go`，提供以下能力：
    - `JarMeta` 结构：含 filename、service_name、size、upload_time
    - `Upload(serviceName, filename, reader)`：将 `.jar` 文件保存到 jar_dir，文件名做 basename 处理防注入；同步将元数据写入 `jar_dir/jars.json`（NDJSON 追加）
    - `List()`：读取 `jars.json`，返回所有 `JarMeta`，含 service_name 字段，供前端按服务名过滤展示
    - `GetFilePath(filename)`：返回指定 jar 的绝对路径，供 HTTP 下载
    - `ExistsForService(serviceName, filename) bool`：校验指定 jar 是否属于指定服务，供发布触发时验证
    在 `jar_handler.go` 中实现：
    - `POST /api/jars`（multipart 上传，表单含 `service_name` 和 `file` 字段）
    - `GET /api/jars`（返回全量 JarMeta 列表，含 service_name）
    - `GET /api/jars/download/{filename}`（文件下载，供 Agent 使用）
  - **涉及文件**：`internal/server/store/jar_store.go`、`internal/server/api/jar_handler.go`
  - **依赖**：TODO-S1
  - **重复上传策略**：同一 service_name + filename 重复上传时，以最新上传的文件内容和时间**覆盖**旧条目（jars.json 中替换旧记录，不追加）；不同 service_name 的同名文件视为独立条目，各自保留。
  - **验收标准**：`curl -F "service_name=svc-a" -F "file=@xxx.jar" http://server/api/jars` 成功上传；`GET /api/jars` 返回含 service_name 的列表；`GET /api/jars/download/xxx.jar` 可下载；同一 service_name + filename 重复上传后 `List()` 中该条目仅有一条（最新）；不同 service_name 的同名文件各自独立保留；`ExistsForService("svc-b", "svc-a-1.0.jar")` 返回 false

- [x] **TODO-S3：发布记录 JSON 持久化**
  - **描述**：实现 `record_store.go`，以 upsert 方式管理 `records.json`（每条记录一行，NDJSON）。提供：
    - `DeployRecord` 结构：task_id、create_time、service_name、jar_name、target_device_ids、overall_status、device_results（含各设备结果）、end_time
    - `Create(record DeployRecord)`：任务创建时立即写入一条状态为"执行中"的记录（overall_status="执行中"，device_results 为空）；若 task_id 已存在则跳过（幂等）
    - `Update(taskID, overallStatus, deviceResults, endTime)`：任务完成时更新该记录的状态和结果；实现方式：重写整个文件（读全量 → 更新目标行 → 全量写回），用 `sync.Mutex` 保护
    - `List()`：读取全部记录，按 create_time 倒序返回
    - 文件不存在时自动创建
  - **涉及文件**：`internal/server/store/record_store.go`
  - **依赖**：TODO-S1
  - **验收标准**：`Create` 后记录立即可在 `List` 中查到（状态为"执行中"）；`Update` 后状态更新正确；程序重启后记录不丢失；Server 重启期间执行中的任务状态在重启后仍可查（显示"执行中"，等待 Agent 重连后上报或人工确认）

- [x] **TODO-S4：Agent WebSocket 连接管理（AgentHub）**
  - **描述**：实现 `agent_hub.go`，管理单个 Agent 的 WebSocket 连接生命周期：
    - Agent 连接时解析 `AgentHandshake`，记录 agent_id 和设备列表，标记在线
    - Agent 断开时标记离线，清理连接
    - 提供 `IsOnline() bool`、`GetDevices() []DeviceInfo`、`SendInstruction(inst DeployInstruction) error`
    - 提供注册结果回调的机制：`OnReport(handler func(TaskReport))`
    - Hub 是全局单例（当前只支持单 Agent）
  - **涉及文件**：`internal/server/deploy/agent_hub.go`、`internal/server/api/ws_handler.go`
  - **依赖**：TODO-G1、TODO-S1
  - **验收标准**：Agent 连接后 Hub `IsOnline()` 返回 true；Agent 断开后返回 false；`SendInstruction` 在 Agent 在线时能发送消息

- [x] **TODO-S5：发布任务管理与状态机**
  - **描述**：实现 `task.go`（Task 数据结构和状态枚举）和 `manager.go`（任务调度逻辑）：
    - Task 状态：`待下发 → 执行中 → 成功 / 部分失败 / 失败`
    - `CreateAndDispatch(req DeployRequest)`：
      1. 校验 jar 属于指定 service（调用 `JarStore.ExistsForService`），不匹配则拒绝
      2. 检查同一服务是否有内存中状态为"执行中"的任务，有则拒绝
      3. 创建 Task（含 task_id、service_name、jar_name、target_device_ids、create_time）
      4. 立即调用 `RecordStore.Create` 持久化初始记录（状态"执行中"）
      5. 检查 Agent 在线：离线则直接调用 `RecordStore.Update` 置为"失败"（含原因"Agent 离线"）；在线则通过 Hub 下发指令
    - `HandleReport(report TaskReport)`：接收 Agent 上报结果（TaskReport 携带完整上下文），根据各设备结果计算最终状态，调用 `RecordStore.Update` 更新；**无需依赖内存中的 Task 对象**，因为 TaskReport 本身包含 service_name、jar_name 等完整信息
    - `GetTask(task_id)`：优先查内存，不存在则从 `RecordStore.List` 中查找（支持 Server 重启后查询历史任务）
    - **Server 重启恢复策略**：Server 启动时从 records.json 加载所有"执行中"记录到内存，标记为"待恢复"；若 Agent 随后重连并上报该 task_id 的结果，正常处理；若 Agent 未上报（如 Agent 也重启），这些任务的最终状态由运维人员通过记录人工判断（不自动标记失败，避免误判正在执行的任务）
  - **涉及文件**：`internal/server/deploy/task.go`、`internal/server/deploy/manager.go`
  - **依赖**：TODO-S2、TODO-S3、TODO-S4
  - **验收标准**：Agent 离线时 CreateAndDispatch 返回失败任务且 records.json 立即有记录；同一服务并发触发时第二次被拒绝；jar 与 service 不匹配时被拒绝；Agent 上报结果后状态正确转换；Server 重启后 `GetTask` 仍能查到历史任务状态

- [x] **TODO-S6：HTTP API 路由组装**
  - **描述**：在 `cmd/server/main.go` 中组装所有路由，实现以下接口：
    - `POST /api/deploy`：触发发布，body 含 service_name、jar_name、target_device_ids；返回 task_id
    - `GET /api/tasks/{task_id}`：查询任务状态（含各设备结果）
    - `GET /api/tasks`：查询发布历史（从 RecordStore 读取）
    - `GET /api/agent/status`：查询 Agent 在线状态和设备列表
    - `GET /ws/agent`：WebSocket 升级端点（供 Agent 连接）
    - 静态文件服务：将 `web/` 目录下的文件通过 `embed.FS` 嵌入，`GET /` 和 `GET /*` 返回前端页面
  - **涉及文件**：`cmd/server/main.go`、`internal/server/api/deploy_handler.go`
  - **依赖**：TODO-S2、TODO-S4、TODO-S5
  - **验收标准**：所有接口返回正确 HTTP 状态码和 JSON；`GET /` 返回 HTML 页面；`go build ./cmd/server` 通过

- [x] **TODO-S7：Web 前端单页面**
  - **描述**：实现 `web/index.html`，使用原生 HTML + JS（无框架、无构建步骤），包含三个功能区：
    - **Jar 管理**：上传 `.jar` 文件（`<input type="file">` + fetch POST，表单含 `service_name` 输入框）；展示已上传 Jar 列表（服务名、文件名、大小、上传时间），与 Spec "服务名 + 文件名"的包列表口径一致
    - **发布操作**：输入/选择服务名称；从已上传 Jar 列表选择目标 Jar；从 Agent 上报的设备列表勾选目标设备；点击"发布"按钮触发 `POST /api/deploy`；展示返回的任务状态（轮询 `GET /api/tasks/{task_id}`，5 秒一次，直到终态）
    - **发布历史**：展示历史记录列表（时间、服务、Jar、设备、结果），从 `GET /api/tasks` 获取
    - 页面顶部展示 Agent 在线/离线状态（从 `GET /api/agent/status` 获取，页面加载时请求）
  - **涉及文件**：`web/index.html`
  - **依赖**：TODO-S6（需先确定所有 API 路径和响应结构）
  - **验收标准**：浏览器打开 `http://server-ip:port/` 能正常加载页面；三个功能区可正常操作；Agent 离线时页面有明确提示；发布触发后状态轮询正常更新

---

### 4.3 Agent 模块

- [x] **TODO-A1：Agent 配置加载**
  - **描述**：定义 Agent 配置文件结构（YAML）并实现加载逻辑。配置项包括：
    - `server.ws_url`：Server WebSocket 地址（如 `ws://10.0.0.1:8080/ws/agent`）
    - `server.jar_base_url`：Jar 下载 HTTP 地址前缀
    - `agent.id`：Agent 标识符
    - `agent.workspace`：本地 Jar 下载临时目录
    - `agent.log_dir`：发布日志存放目录
    - `devices[]`：每台设备的 id、host、ssh_user、ssh_port（默认 22）、temp_dir（Jar 接收临时目录）
    - `services[]`：每个服务在每台设备上的配置，含 device_id、service_name、deploy_dir、**target_jar_name**（启动脚本引用的固定目标文件名，如 `app.jar`，可与上传文件名不同）、start_script、stop_script、process_name
  - **涉及文件**：`internal/agent/config/config.go`、`config/agent.yaml`、`cmd/agent/main.go`
  - **依赖**：TODO-G1
  - **验收标准**：`./ops-agent --config config/agent.yaml` 能正确读取配置；配置缺失时输出明确错误并退出

- [x] **TODO-A2：WebSocket 客户端（含自动重连）**
  - **描述**：实现 `internal/agent/ws/client.go`：
    - 启动时连接 Server WS 端点，连接成功后立即发送 `AgentHandshake`（含 agent_id 和设备列表）
    - 连接断开后（任何原因），以指数退避（初始 3s，最大 60s）自动重连，重连成功后重新发送握手
    - 监听 Server 下发的 `DeployInstruction` 消息，收到后异步调用注册的 handler（不阻塞 WS 读循环）
    - 提供 `SendReport(report TaskReport) error` 方法，供 runner 上报结果
    - 整个客户端在独立 goroutine 运行，提供 `Stop()` 优雅退出
  - **涉及文件**：`internal/agent/ws/client.go`、`cmd/agent/main.go`
  - **依赖**：TODO-G1、TODO-A1
  - **验收标准**：Agent 启动后 Server 侧显示 Agent 在线；手动断开 Server 后 Agent 自动重连；重连后 Server 侧重新显示在线

- [x] **TODO-A3：Jar 包 HTTP 下载**
  - **描述**：实现 `internal/agent/deploy/downloader.go`：
    - `Download(downloadURL, destDir) (localPath string, err error)`
    - 使用标准库 HTTP GET 下载到 `agent.workspace` 目录，文件名从 URL 路径中提取
    - 下载完成后校验：文件大小 > 0、文件可正常读取
    - 失败时清理已下载的不完整文件，返回错误
  - **涉及文件**：`internal/agent/deploy/downloader.go`
  - **依赖**：TODO-A1
  - **验收标准**：给定合法 URL 时文件成功下载到指定目录且通过校验；Server 返回 404 时函数返回清晰错误；下载失败不留残留文件

- [x] **TODO-A4：SCP 分发到目标设备**
  - **描述**：实现 `internal/agent/deploy/distributor.go`：
    - `Distribute(localJarPath string, device DeviceConfig) error`
    - 调用系统 `scp` 命令（`os/exec`）将 jar 文件传输到目标设备的 `temp_dir`
    - 命令格式：`scp -o StrictHostKeyChecking=no <localJarPath> <user>@<host>:<temp_dir>/`
    - 捕获 scp 的 stdout/stderr，写入发布日志
    - 返回成功或包含 stderr 信息的错误
  - **涉及文件**：`internal/agent/deploy/distributor.go`
  - **依赖**：TODO-A1、TODO-A6（日志接口）
  - **验收标准**：SSH 免密配置正常时 scp 成功，目标设备 temp_dir 下出现 jar 文件；SSH 不通时返回包含 scp stderr 的错误信息

- [x] **TODO-A5：SSH 远程替换**
  - **描述**：实现 `internal/agent/deploy/replacer.go`：
    - `Replace(device DeviceConfig, service ServiceConfig, uploadedJarFilename string, logger *DeployLogger) error`
    - **全部通过 `os/exec` 调用系统 `ssh` 命令**执行远程操作，与 SCP 分发共享同一套认证链路（`~/.ssh/config`、ssh-agent、任意密钥算法），命令格式：`ssh -o StrictHostKeyChecking=no <user>@<host> "<remote_cmd>"`
    - 串行执行以下步骤，每步的 stdout/stderr 写入 logger：
      1. 执行 stop_script：`ssh ... "<stop_script>"`
      2. 轮询进程：每 3 秒执行 `ssh ... "pgrep -f <process_name>"`，最长等待 60 秒；仍存在则返回"停止超时"错误，终止该设备替换
      3. 覆盖 Jar：`ssh ... "mv <temp_dir>/<uploadedJarFilename> <deploy_dir>/<target_jar_name>"`；**目标文件名使用配置中的 `target_jar_name`**，而非上传文件名，确保覆盖启动脚本所引用的固定文件
      4. 执行 start_script：`ssh ... "<start_script>"`
      5. 轮询进程：每 3 秒执行 `ssh ... "pgrep -f <process_name>"`，最长等待 60 秒；未出现则返回"启动超时"错误
    - 返回 nil 表示成功，否则返回含失败步骤说明的错误
  - **涉及文件**：`internal/agent/deploy/replacer.go`
  - **依赖**：TODO-A1、TODO-A6（日志接口）
  - **验收标准**：正常场景下五步串行执行完成，`deploy_dir/target_jar_name` 为新文件，进程存在；停止超时（60s 内进程未退出）时返回错误并不执行后续步骤；启动超时时返回错误；`target_jar_name` 与 `uploadedJarFilename` 不同时仍能正确覆盖；所有步骤 stdout/stderr 均写入日志

- [x] **TODO-A6：发布日志写入**
  - **描述**：实现 `internal/agent/deploy/logger.go`（`DeployLogger`）：
    - `NewDeployLogger(logDir, taskID string) (*DeployLogger, error)`：在 `log_dir/<taskID>_<timestamp>.log` 创建日志文件
    - `Log(deviceID, step, message string)`：以结构化格式写入一行：`[时间戳] [deviceID] [step] message`
    - `Close()`：关闭文件
    - distributor 和 replacer 均接受 `*DeployLogger` 参数并调用 `Log` 记录各步骤输出
  - **涉及文件**：`internal/agent/deploy/logger.go`
  - **依赖**：TODO-A1
  - **验收标准**：每次发布在 log_dir 下生成独立日志文件；文件内容包含各设备各步骤的时间戳和输出；多次发布日志文件不互相覆盖

- [x] **TODO-A7：发布流程编排 + 结果上报**
  - **描述**：实现 `internal/agent/deploy/runner.go`（`DeployRunner`）：
    - `Run(inst DeployInstruction, ws WSClient)`：接收 Server 下发的指令，完整执行一次发布
    - 流程：创建 logger → Download jar → 串行对每台设备 Distribute → 串行对每台设备 Replace → 汇总 DeviceResult 列表 → 写 logger 汇总行 → 关闭 logger → 调用 `ws.SendReport(TaskReport)`
    - 分发失败的设备：跳过 Replace，直接记录该设备状态为失败
    - 替换失败的设备：记录失败原因，继续下一台
    - `Run` 在独立 goroutine 中执行（由 WS client 收到指令后异步调用）
    - 同时只允许一个 Run 执行（用 mutex 或 channel 限制）
  - **涉及文件**：`internal/agent/deploy/runner.go`、`cmd/agent/main.go`（组装注册）
  - **依赖**：TODO-A2、TODO-A3、TODO-A4、TODO-A5、TODO-A6
  - **验收标准**：端到端发布流程完整执行；Server 收到 TaskReport 后任务状态正确更新；日志文件完整；并发两次指令时第二次被阻塞直到第一次完成

---

## 5. 依赖关系与执行顺序

```
TODO-G1
  ├──► TODO-S1 ──► TODO-S2 ──► TODO-S6 ──► TODO-S7
  │              ├──► TODO-S3 ──► TODO-S5 ──┘
  │              └──► TODO-S4 ──► TODO-S5
  │
  └──► TODO-A1 ──► TODO-A2 ──────────────────────────────► TODO-A7
                 ├──► TODO-A3 ──────────────────────────► TODO-A7
                 ├──► TODO-A6 ──► TODO-A4 ──────────────► TODO-A7
                 │              └──► TODO-A5 ──────────► TODO-A7
                 └──────────────────────────────────────► TODO-A7
```

**可并行的任务组：**
- G1 完成后：S1 ‖ A1 可同时开始
- S1 完成后：S2 ‖ S3 ‖ S4 可同时开始
- A1 完成后：A2 ‖ A3 ‖ A6 可同时开始；A6 完成后 A4 ‖ A5 可同时开始
- S5 完成后 S6 开始；S6 完成后 S7 开始
- A2~A6 全完成后 A7 开始

**建议开发顺序（单人）：**
G1 → S1 → S3 → S4 → S5 → S2 → S6 → A1 → A6 → A3 → A4 → A5 → A7 → S7

---

## 6. 测试标准

### 6.1 单元测试标准

| TODO | 测试覆盖点 |
|------|-----------|
| G1 | 消息结构 JSON 序列化/反序列化正确；TaskReport 含完整上下文字段（service_name、jar_name、target_device_ids） |
| S2 | 上传文件名含路径符时被 basename 截断；上传时 service_name 绑定写入 jars.json；`ExistsForService("svc-b","svc-a.jar")` 返回 false |
| S3 | `Create` 后 `List` 立即可查到（状态"执行中"）；`Update` 后状态更新正确；NDJSON 格式合法；文件不存在时自动创建；程序重启后记录不丢失 |
| S4 | Agent 连接/断开时 IsOnline 状态变化；Agent 离线时 SendInstruction 返回错误 |
| S5 | Agent 离线时 CreateAndDispatch 立即持久化失败记录；同服务并发触发时第二次被拒绝；jar 与 service 不匹配时被拒绝；全设备成功→成功；部分设备失败→部分失败；全失败→失败；Server 重启后 GetTask 仍可查历史记录 |
| A3 | 正常下载文件大小校验通过；服务端返回空文件时校验失败并清理 |
| A5 | 进程存在超过 60s 时停止步骤返回超时错误并不执行后续步骤；启动后 60s 内进程未出现时返回超时错误；`target_jar_name` 与上传文件名不同时 mv 命令目标路径使用 target_jar_name |
| A6 | 多次 Log 调用后文件内容格式正确；不同 taskID 生成不同文件 |

### 6.2 集成 / 场景验证标准

**场景 1：端到端正常发布（对应 AC-2）**
- 前置：Server 启动，Agent 启动并在线，设备 B/C SSH 免密可用，B/C 启停脚本正常
- 步骤：上传 jar → 触发发布（选 B、C）→ 轮询任务状态
- 期望：任务状态最终为"成功"；B/C 运行目录 jar 文件为新版本；B/C 服务进程存在；records.json 有对应记录；Agent log_dir 有日志文件

**场景 2：单台设备失败（对应 AC-3）**
- 前置：设备 B 的启动脚本故意改为异常
- 步骤：同场景 1
- 期望：任务状态为"部分失败"；B 的设备结果含失败原因；C 发布成功

**场景 3：Agent 离线（对应 AC-4）**
- 前置：停止 Agent 进程
- 步骤：在 Web 页面触发发布
- 期望：`GET /api/agent/status` 返回离线；发布接口返回失败任务（"Agent 离线"）；records.json 有失败记录

**场景 4：Agent 断线重连（对应 AC-7）**
- 步骤：Agent 运行中，重启 Server
- 期望：Agent 自动重连，Server 侧重新显示在线（最长 60s 内）

**场景 5：停止超时**
- 前置：B 的停止脚本不关闭进程
- 期望：Agent 等待 60s 后记录"停止超时"错误，跳过该设备后续步骤，继续处理下一台设备

---

## 7. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| SSH 免密配置不稳定（A→B/C） | SCP 分发和 SSH 替换失败，发布中断 | 统一走系统 ssh/scp 命令，复用全套现有 SSH 配置；失败时 stderr 完整写入日志，便于人工排查 |
| 目标设备启停脚本不规范 | 停止/启动步骤失败，超时 | 超时机制兜底（1 分钟）；失败信息含 SSH 输出，人工可根据日志排查脚本问题 |
| target_jar_name 与实际脚本引用文件名不一致 | 替换后服务仍加载旧 jar | 由运维人员在 Agent 配置中明确配置 target_jar_name，与启动脚本引用路径保持一致；首次使用前需核查配置 |
| Server 重启时有任务正在执行 | 任务状态停留在"执行中"，记录不完整 | 任务创建时立即持久化（RecordStore.Create）；Agent 重连后可继续上报结果；记录中保留"执行中"状态供人工判断，不自动误判 |
| records.json 并发写入损坏 | 发布记录丢失或 JSON 格式错误 | Server 单进程，所有写操作用 sync.Mutex 保护；Update 时全量读写，原子替换文件 |
| 同一服务并发发布 | 替换流程冲突，状态混乱 | Manager 层用内存锁判断同一服务是否有任务执行中，并发时拒绝新任务并返回明确错误 |

---

## 8. 变更记录

| 日期 | 变更内容 |
|------|---------|
| 2026-04-05 | 初稿创建 |
| 2026-04-05 | 修复 review P1/P2 问题：统一 SSH/SCP 认证路径为 os/exec；补充 Jar-服务名绑定（JarMeta + ExistsForService）；补充 target_jar_name 配置项及替换步骤目标路径；任务创建时立即持久化（RecordStore.Create/Update）；TaskReport 携带完整上下文；补充 Go 1.22 路由参数约定；更新单元测试覆盖点和风险表 |

## 9. 开放问题

无。所有技术决策已确认。
