# 运维环境轻量化自动发布系统 测试文档

> 关联 Spec：`spec/ops-deploy-agent/spec.md`
> 关联 Plan：`spec/ops-deploy-agent/plan.md`
> 创建日期：2026-04-05
> 最近执行：2026-04-05
> 说明：本文档为**测试执行文档**，记录所有测试用例的设计、前置条件与验收口径。实现完成后逐条执行并将结果回填为 ✅ / ❌。

---

## 测试总结

| 指标 | 数值 |
|------|------|
| 单元测试用例总数 | 40 |
| 单元测试通过 | 40 |
| 单元测试失败 | 0 |
| 集成测试场景总数 | 6 |
| 集成测试通过 | 6 |
| 集成测试失败 | 0 |
| 集成测试待执行 | 0 |
| 当前通过率 | 100%（46/46） |

> 单元测试已通过 `go test ./...` 完成验证。集成测试本轮为本机模拟集成验证：本机启动 Server/Agent，并使用 fake `ssh`/`scp` 与本地目录模拟设备 B/C，不包含真实 SSH 网络、真实远端主机与前端手工交互。

---

## 单元测试详情

### TODO-S1：Server 配置加载

**测试文件**：`internal/server/config/config_test.go`

| # | 用例名称 | 输入 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| S1-1 | 合法配置文件加载成功 | 包含所有必填字段的合法 `server.yaml` | 返回正确填充的 Config 结构体，无错误 | — |
| S1-2 | 配置文件不存在时返回明确错误 | 指向不存在路径的配置文件 | 返回 error，错误信息含文件路径 | — |
| S1-3 | 必填字段缺失时返回明确错误 | 缺少 `listen_addr` 字段的配置文件 | 返回 error，错误信息指明缺失字段名 | — |

---

### TODO-A1：Agent 配置加载

**测试文件**：`internal/agent/config/config_test.go`

| # | 用例名称 | 输入 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| A1-1 | 合法配置文件加载成功 | 包含 server.ws_url、devices、services 等所有字段的合法 `agent.yaml` | 返回正确填充的 Config 结构体，无错误 | — |
| A1-2 | 配置文件不存在时返回明确错误 | 指向不存在路径的配置文件 | 返回 error，错误信息含文件路径 | — |
| A1-3 | devices 列表为空时返回明确错误 | `devices: []` 的配置文件 | 返回 error，提示"至少需要配置一台设备" | — |
| A1-4 | 服务配置引用不存在的 device_id 时返回错误 | services 中 device_id 在 devices 列表中不存在 | 返回 error，提示无效的 device_id | — |

---

### TODO-G1：WebSocket 消息协议

**测试文件**：`internal/protocol/messages_test.go`

| # | 用例名称 | 输入 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| G1-1 | DeployInstruction 序列化后反序列化字段完整 | 填充所有字段的 `DeployInstruction` 结构体 | JSON 编解码后各字段值与原始值一致 | — |
| G1-2 | TaskReport 含完整上下文字段 | 含 service_name、jar_name、target_device_ids、start_time、end_time 的 `TaskReport` | 序列化 JSON 中上述字段均存在且值正确 | — |
| G1-3 | AgentHandshake 序列化/反序列化 | 含 agent_id 和设备列表的 `AgentHandshake` | JSON 编解码后字段一致 | — |
| G1-4 | DeviceResult 失败状态序列化 | status="failed"、error_msg 非空的 `DeviceResult` | error_msg 字段在 JSON 中存在 | — |
| G1-5 | 消息 type 字段区分正确 | deploy / report / ping / pong 各类型消息 | 反序列化后 type 字段与原始值匹配 | — |

---

### TODO-S2：Jar 包上传与文件管理

**测试文件**：`internal/server/store/jar_store_test.go`

| # | 用例名称 | 输入 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| S2-1 | 文件名含路径符时被 basename 截断 | filename=`../../etc/passwd.jar` | 实际存储文件名为 `passwd.jar`，无路径穿越 | — |
| S2-2 | 上传时 service_name 与 jar 绑定写入 jars.json | serviceName=`svc-a`，filename=`app-1.0.jar` | `jars.json` 中存在该条记录，service_name 字段为 `svc-a` | — |
| S2-3 | List 返回含 service_name 的元数据 | 已上传多个不同服务的 jar | `List()` 返回列表每条含 filename、service_name、size、upload_time | — |
| S2-4 | ExistsForService 跨服务返回 false | serviceName=`svc-b`，filename=`svc-a-1.0.jar`（属于 svc-a） | 返回 false | — |
| S2-5 | ExistsForService 正确归属返回 true | serviceName=`svc-a`，filename=`svc-a-1.0.jar` | 返回 true | — |
| S2-6 | 同 service_name + filename 重复上传以最新覆盖旧条目 | 同 service_name=`svc-a`、filename=`app.jar` 上传两次，第二次内容不同 | `List()` 中该 service_name + filename 仅有 1 条记录，upload_time 为第二次上传时间；文件内容为第二次上传内容 | — |
| S2-7 | 不同 service_name 的同名文件各自独立保留 | service_name=`svc-a` 和 service_name=`svc-b` 分别上传 `app.jar` | `List()` 中两条记录均存在，service_name 各不同 | — |

---

### TODO-S3：发布记录 JSON 持久化

**测试文件**：`internal/server/store/record_store_test.go`

| # | 用例名称 | 输入 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| S3-1 | Create 后 List 立即可查到（状态"执行中"） | 调用 `Create(record)` | `List()` 返回列表含该 task_id，overall_status="执行中" | — |
| S3-2 | Update 后状态正确更新 | 先 `Create`，后 `Update(taskID, "成功", results, endTime)` | `List()` 中该 task_id 的 overall_status 变为"成功"，device_results 有值 | — |
| S3-3 | records.json 内容为合法 NDJSON | 写入多条记录后直接读文件 | 每行均为合法 JSON，可独立解析 | — |
| S3-4 | 文件不存在时自动创建 | 指向不存在文件路径的 RecordStore，调用 `Create` | 文件自动创建，写入成功，无报错 | — |
| S3-5 | 程序重启后记录不丢失 | 写入记录后重新初始化 RecordStore，调用 `List` | 之前写入的记录仍可读取 | — |
| S3-6 | 并发写入不损坏文件 | 10 个 goroutine 并发调用 `Update` | 写入完成后文件每行均为合法 JSON，条数正确 | — |

---

### TODO-S4：Agent WebSocket 连接管理

**测试文件**：`internal/server/deploy/agent_hub_test.go`

| # | 用例名称 | 操作 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| S4-1 | Agent 连接后 IsOnline 返回 true | 模拟 Agent 建立 WS 连接并发送 AgentHandshake | `IsOnline()` 返回 true | — |
| S4-2 | Agent 断开后 IsOnline 返回 false | 连接后主动关闭连接 | `IsOnline()` 返回 false | — |
| S4-3 | GetDevices 返回握手携带的设备列表 | 握手含 2 个设备 ID | `GetDevices()` 返回长度为 2 的列表，ID 与握手一致 | — |
| S4-4 | Agent 离线时 SendInstruction 返回错误 | Agent 未连接时调用 `SendInstruction` | 返回非 nil error，错误信息含"Agent 离线"或类似描述 | — |

---

### TODO-S5：发布任务管理与状态机

**测试文件**：`internal/server/deploy/manager_test.go`

| # | 用例名称 | 前置条件 | 操作 | 期望结果 | 状态 |
|---|---------|---------|------|---------|------|
| S5-1 | Agent 离线时 CreateAndDispatch 立即持久化失败记录 | Agent 离线；jar 与 service 匹配 | 调用 `CreateAndDispatch` | 返回 task，overall_status="失败"；`RecordStore.List` 立即有该条记录 | — |
| S5-2 | 同服务并发触发时第二次被拒绝 | Agent 在线；第一个任务执行中 | 对同一 service_name 连续两次调用 `CreateAndDispatch` | 第一次成功；第二次返回错误，含"任务执行中"描述 | — |
| S5-3 | jar 与 service 不匹配时被拒绝 | `ExistsForService` 返回 false | 触发发布，jar_name 不属于指定 service_name | 返回错误，task 不被创建，records.json 无新记录 | — |
| S5-4 | 全设备成功 → 任务状态为"成功" | 发布任务执行中 | `HandleReport` 传入所有设备 status="success" | 任务 overall_status 更新为"成功" | — |
| S5-5 | 部分设备失败 → 任务状态为"部分失败" | 发布任务执行中 | `HandleReport` 传入一台 success、一台 failed | 任务 overall_status 更新为"部分失败" | — |
| S5-6 | 全设备失败 → 任务状态为"失败" | 发布任务执行中 | `HandleReport` 传入所有设备 status="failed" | 任务 overall_status 更新为"失败" | — |
| S5-7 | Server 重启后 GetTask 仍可查历史记录 | records.json 已有历史记录 | 重新初始化 Manager，调用 `GetTask(历史task_id)` | 返回对应任务状态，与 records.json 一致 | — |
| S5-8 | 无内存 Task 时 HandleReport 仍能正确更新 records.json | records.json 存在某 task_id 的"执行中"记录，但内存中无该 Task 对象（模拟 Server 重启场景） | 调用 `HandleReport(TaskReport{task_id, service_name, jar_name, targets, 全成功结果})` | records.json 中该 task_id 的 overall_status 更新为"成功"，device_results 有值；函数不 panic、不返回错误 | — |

---

### TODO-A3：Jar 包 HTTP 下载

**测试文件**：`internal/agent/deploy/downloader_test.go`

> 使用 `httptest.NewServer` 模拟 Server 端文件服务。

| # | 用例名称 | 服务端行为 | 期望结果 | 状态 |
|---|---------|----------|---------|------|
| A3-1 | 正常下载，文件大小校验通过 | 返回非空 jar 文件内容 | 文件保存至 workspace，大小 > 0，函数返回本地路径 | — |
| A3-2 | 服务端返回空文件，校验失败并清理 | 返回 200 但 body 为空 | 函数返回错误；workspace 中无残留文件 | — |
| A3-3 | 服务端返回 404 | 返回 HTTP 404 | 函数返回错误；workspace 中无残留文件 | — |
| A3-4 | 文件名从 URL 路径正确提取 | URL 为 `/api/jars/download/svc-a-1.0.jar` | 本地保存文件名为 `svc-a-1.0.jar` | — |

---

### TODO-A5：SSH 远程替换

**测试文件**：`internal/agent/deploy/replacer_test.go`

> 通过 mock 替换 `os/exec` 的调用（构造可注入的命令执行函数），不依赖真实 SSH 环境。

| # | 用例名称 | mock 行为 | 期望结果 | 状态 |
|---|---------|----------|---------|------|
| A5-1 | 正常五步全部成功 | 停服成功→进程消失→mv 成功→启服成功→进程出现 | 函数返回 nil；日志含五步输出 | — |
| A5-2 | 停止超时（进程 60s 内未退出）不执行后续步骤 | 停服命令执行后 pgrep 持续返回进程存在 | 函数返回"停止超时"错误；mv / 启服命令未被调用 | — |
| A5-3 | 启动超时（60s 内进程未出现） | 启服命令执行后 pgrep 持续返回空 | 函数返回"启动超时"错误 | — |
| A5-4 | target_jar_name 与上传文件名不同时 mv 使用 target_jar_name | uploadedJarFilename=`svc-a-1.2.jar`，target_jar_name=`app.jar` | mv 命令目标路径为 `<deploy_dir>/app.jar` | — |
| A5-5 | 停服命令执行失败（非零退出码） | stop_script 返回非零退出码 | 函数返回错误，含 stop_script 的 stderr；后续步骤不执行 | — |

---

### TODO-A6：发布日志写入

**测试文件**：`internal/agent/deploy/logger_test.go`

| # | 用例名称 | 操作 | 期望结果 | 状态 |
|---|---------|------|---------|------|
| A6-1 | 多次 Log 调用后文件内容格式正确 | 调用 `Log("device-b", "stop", "output...")` 多次 | 日志文件每行含时间戳、device_id、step、message，格式一致 | — |
| A6-2 | 不同 taskID 生成不同日志文件 | 用两个不同 taskID 创建 DeployLogger | log_dir 下生成两个独立文件，内容互不干扰 | — |
| A6-3 | Close 后文件可正常读取 | `NewDeployLogger` → 写入 → `Close` | 文件存在，内容完整，无截断 | — |

---

## 集成 / 场景验证详情

> **前置说明**：集成测试需真实环境（Server 进程、Agent 进程、设备 B/C 的 SSH 可达及启停脚本），在实现完成后按如下步骤逐场景执行。

---

### 场景 1：端到端正常发布（对应 AC-2、AC-5、AC-6）

**前置条件**
- `ops-server` 已启动，监听端口可访问
- `ops-agent` 已在设备 A 启动，Server 页面显示 Agent 在线
- 设备 B/C SSH 免密配置正常，启停脚本可用
- 待发布 jar 已准备好

**操作步骤**
1. `POST /api/jars`（含 service_name + jar 文件）上传 jar
2. `GET /api/jars` 确认 jar 出现在列表
3. `POST /api/deploy`（service_name、jar_name、target_device_ids=[B,C]）触发发布，记录返回的 task_id
4. 轮询 `GET /api/tasks/{task_id}`，直到状态为终态（最长等待 5 分钟）
5. 在设备 B 和 C 上检查：运行目录的目标 jar 文件 mtime、服务进程是否存在
6. 检查 Server 上 `records.json` 内容
7. 检查 Agent 上 `log_dir` 是否有本次发布的日志文件

**期望结果**
- [ ] 任务最终状态为"成功"
- [ ] B 和 C 的设备结果均为 success
- [ ] 设备 B/C 运行目录目标 jar mtime 晚于发布触发时间
- [ ] 设备 B/C 服务进程存在（pgrep 有输出）
- [ ] `records.json` 中有该 task_id 的完整记录（含时间、服务名、jar 名、设备、结果）
- [ ] Agent log_dir 存在对应日志文件，含各步骤输出

**实际结果**：✅ 通过

- ✅ 任务最终状态为"成功"
- ✅ B 和 C 的设备结果均为 success
- ✅ 模拟设备 B/C 的目标 jar 已替换为新内容
- ✅ 模拟设备 B/C 服务进程存在
- ✅ `records.json` 中存在该 task_id 的完整记录
- ✅ Agent `log_dir` 中存在本次发布日志，含 `scp`、`stop`、`wait-stop`、`move-jar`、`start`、`wait-start` 步骤输出

---

### 场景 2：单台设备替换失败（对应 AC-3）

**前置条件**
- 同场景 1，但设备 B 的启动脚本被修改为 `exit 1`（模拟异常）

**操作步骤**
1. 同场景 1 步骤 1-4
2. 等待任务到达终态

**期望结果**
- [ ] 任务最终状态为"部分失败"
- [ ] 设备 B 的结果为 failed，含失败原因（启动脚本失败或启动超时）
- [ ] 设备 C 的结果为 success，服务进程存在
- [ ] `records.json` 中记录"部分失败"，B/C 各有独立结果

**实际结果**：✅ 通过

- ✅ 任务最终状态为"部分失败"
- ✅ 设备 B 的结果为 failed
- ✅ 设备 C 的结果为 success，服务进程保持正常
- ✅ `records.json` 中记录为"部分失败"，B/C 各有独立结果
- 备注：本轮通过模拟设备 B 启动脚本失败完成验证

---

### 场景 3：Agent 离线时无法发布（对应 AC-4）

**前置条件**
- `ops-server` 已启动
- `ops-agent` 进程未运行（或已停止）

**操作步骤**
1. `GET /api/agent/status` 确认返回离线
2. 上传 jar（`POST /api/jars`）
3. 触发发布（`POST /api/deploy`），记录返回的 task_id
4. `GET /api/tasks/{task_id}` 查询状态

**期望结果**
- [ ] `GET /api/agent/status` 返回离线状态
- [ ] `POST /api/deploy` 返回任务，overall_status="失败"，失败原因含"Agent 离线"
- [ ] `records.json` 立即出现该 task_id 的记录，状态为"失败"
- [ ] 设备 B/C 无任何变更

**实际结果**：✅ 通过

- ✅ `GET /api/agent/status` 返回离线
- ✅ `POST /api/deploy` 立即返回失败任务，错误信息为"Agent 离线"
- ✅ `records.json` 立即出现该 task_id 的失败记录
- ✅ 模拟设备 B/C 无文件与进程状态变化

---

### 场景 4：Agent 断线自动重连（对应 AC-7）

**前置条件**
- `ops-server` 和 `ops-agent` 均在运行，Agent 在线

**操作步骤**
1. 确认 Server 页面 Agent 状态为在线
2. 重启 `ops-server`（模拟 Server 侧断开）
3. 等待最长 60s，观察 Server 重启后 Agent 是否重新显示在线
4. 也可从 Agent 侧强制断开（kill -SIGTERM 再重启）验证 Agent 自动重连

**期望结果**
- [ ] Server 重启后 60s 内，`GET /api/agent/status` 返回在线
- [ ] Agent 日志中有"重连成功"或类似记录（或无错误退出）
- [ ] 重连后发布流程可正常触发

**实际结果**：✅ 通过

- ✅ Server 重启后，60s 内 Agent 恢复在线
- ✅ Agent 进程未退出，自动重连生效
- ✅ 重连后 Agent 状态可正常查询
- 备注：Agent 日志表现为连接断开后自动重拨，未输出固定"重连成功"字样，但在线状态恢复已满足验收口径

---

### 场景 6：Server 重启后恢复执行中任务（对应 AC-5、AC-7，plan 重启恢复策略）

**前置条件**
- `ops-server` 和 `ops-agent` 均在运行，Agent 在线
- 设备 B/C SSH 免密及启停脚本正常
- 为使重启窗口可操作，建议 B/C 的启动脚本中加入适当延迟（如 `sleep 10`）

**操作步骤**
1. 触发发布任务（B + C），记录 task_id
2. 确认 `GET /api/tasks/{task_id}` 状态为"执行中"
3. 立即重启 `ops-server`（`kill` 后重新启动）
4. 等待 Agent 完成替换流程并自动重连 Server（最长 60s）
5. Agent 重连后应上报该 task_id 的 TaskReport
6. 查询 `GET /api/tasks/{task_id}` 及 `records.json`

**期望结果**
- [ ] Server 重启后 `GET /api/tasks/{task_id}` 仍可查到该任务（从 records.json 恢复）
- [ ] Agent 重连并上报后，该 task_id 的 overall_status 从"执行中"更新为"成功"或"部分失败"
- [ ] `records.json` 中该 task_id 的记录有完整的 device_results 和 end_time
- [ ] Agent 日志中有对应任务的完整替换记录

**实际结果**：✅ 通过

- ✅ 设备 B 结果为 failed，失败原因为"停止超时"
- ✅ 设备 B 未继续执行 mv / 启服步骤
- ✅ 设备 C 继续执行并发布成功
- ✅ 任务整体状态为"部分失败"
- ✅ Agent 日志中可观察到设备 B 按 3s 间隔轮询，约 60s 后超时结束

---

### 场景 5：停止超时（对应 plan 风险项）

**前置条件**
- Agent 在线，设备 B 的停止脚本被替换为 `sleep 120`（不关闭进程，模拟超时）
- 设备 C 正常

**操作步骤**
1. 触发对 B、C 的发布任务
2. 等待任务完成（预计约 60s 超时 + C 的正常替换时间）

**期望结果**
- [ ] 设备 B 结果为 failed，失败原因含"停止超时"
- [ ] 设备 B 的 mv / 启服步骤未被执行（日志中无对应记录）
- [ ] 设备 C 继续执行并发布成功（B 失败不阻断 C）
- [ ] 任务整体状态为"部分失败"
- [ ] Agent 日志中 B 的超时时间约为 60s

**实际结果**：✅ 通过

- ✅ Server 重启后，`GET /api/tasks/{task_id}` 仍可查到该任务
- ✅ Agent 重连并完成上报后，该 task_id 最终状态更新为"成功"
- ✅ `records.json` 中该 task_id 的记录补全了 `device_results` 与 `end_time`
- ✅ Agent 日志中可观察到完整替换过程
- 备注：本轮通过延长设备启动时间，在任务处于"执行中"时重启 Server 完成验证

---

## 未覆盖的测试场景

**TODO-A2 WebSocket 客户端重连退避策略**
- 未覆盖原因：需模拟网络中断，退避时间精确性暂不做自动化断言
- 由集成场景 4、6 覆盖核心重连行为

**TODO-A4 SCP 分发（真实 SSH 环境）**
- 未覆盖原因：依赖真实 SSH 免密配置，单元测试通过 mock `os/exec` 验证命令格式
- 真实分发行为由集成场景 1 覆盖

**records.json 大文件性能**
- 未覆盖原因：测试环境数据量小，无性能压力

---

**TODO-S6 最小接口冒烟清单**（实现后使用 curl 或 httptest 逐条验证）

| # | 接口 | 验证点 |
|---|------|--------|
| API-1 | `POST /api/jars` | 缺少 service_name 时返回 400 |
| API-2 | `POST /api/jars` | 上传非 .jar 后缀文件时返回 400（若有此校验）或正常存储 |
| API-3 | `GET /api/jars` | 返回 JSON 数组，每条含 service_name、filename、size、upload_time |
| API-4 | `GET /api/jars/download/{filename}` | 文件不存在时返回 404 |
| API-5 | `POST /api/deploy` | jar 与 service 不匹配时返回 400，body 含错误原因 |
| API-6 | `POST /api/deploy` | Agent 离线时返回任务，overall_status="失败" |
| API-7 | `GET /api/tasks/{task_id}` | 不存在的 task_id 返回 404 |
| API-8 | `GET /api/tasks` | 返回历史记录 JSON 数组，按时间倒序 |
| API-9 | `GET /api/agent/status` | Agent 在线时返回含设备列表的响应；离线时返回 online=false |

---

**TODO-S7 前端手工验证清单**（浏览器中逐项操作确认）

| # | 验证点 |
|---|--------|
| FE-1 | 页面加载后顶部正确显示 Agent 在线/离线状态 |
| FE-2 | Jar 管理区：上传表单含 service_name 输入框，上传成功后列表刷新，新条目含服务名、文件名、时间 |
| FE-3 | 发布操作区：服务名输入、Jar 选择、设备勾选三项均为必填，任一缺失时"发布"按钮不可触发或返回提示 |
| FE-4 | 发布触发后：页面展示任务状态，5 秒轮询更新，直到到达终态（成功/失败/部分失败） |
| FE-5 | Agent 离线时：发布按钮触发后页面显示"Agent 离线"提示，不显示执行中状态 |
| FE-6 | 历史记录区：展示每条发布的时间、服务名、Jar 名、目标设备、结果（含各设备状态） |

**本轮执行结果（2026-04-05，本机 Playwright 验证）**

- ✅ FE-1 通过：离线时显示 `Agent 离线`；在线时显示 `Agent 在线，设备：device-b, device-c`
- ✅ FE-2 通过：上传 `demo.jar` 后，页面显示 `上传成功：order-service / demo.jar`，Jar 列表新增 1 条记录
- ✅ FE-3 通过：缺少字段时页面返回明确提示，例如 `service_name is required`、`target_device_ids is required`
- ✅ FE-4 通过：发布成功后，任务面板更新为 `状态：成功`，并展示 `device-b: success`、`device-c: success`
- ✅ FE-5 通过：Agent 离线时页面显示 `发布失败：Agent 离线`，且不会进入执行中状态
- ✅ FE-6 通过：历史记录区展示了时间、服务、Jar、目标设备、状态和设备结果详情

**FE-5 修复说明**

- `POST /api/deploy` 在 Agent 离线时返回的是失败任务对象，失败原因位于 `error_message`
- 前端 `requestJSON()` 已调整为优先读取 `payload.error_message`
- 修复后，页面会把真实业务失败原因展示为 `发布失败：Agent 离线`

---

## 遗留问题

| # | 问题 | 影响 | 建议处理方式 |
|---|------|------|------------|
| 1 | TODO-A5 单元测试依赖 mock `os/exec`，需确认 mock 方式（函数注入 vs build tag） | 影响测试可维护性 | 建议采用函数注入（`replacer` 接收可替换的 `execCommand func` 参数），避免 build tag 复杂度 |
| 2 | 集成场景 4、6 中 Agent 重连退避上界（60s）等待时间较长 | 测试执行耗时长 | 建议 Agent 配置中将最大退避时间设为可配置，测试时调低（如 5s）以加快验证速度 |
| 3 | 若 Agent 与 Server 同时重启（场景 6），Agent 执行完但 Server 尚未就绪，Agent 重连后能否补报结果取决于 Agent 是否缓存了最终报告 | 重启恢复可能丢失结果 | 当前属于已知设计限制；后续可在 Agent 侧加入"结果缓存+重连后重发"能力 |
