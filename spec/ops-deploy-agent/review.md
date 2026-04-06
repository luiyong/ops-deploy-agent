# 运维环境轻量化自动发布系统 Plan 复审报告

> 关联 Spec：`spec/ops-deploy-agent/spec.md`
> 关联 Plan：`spec/ops-deploy-agent/plan.md`
> 审查日期：2026-04-05
> 审查范围：实施方案设计、TODO 拆解、需求覆盖与实现风险

## 审查总结

本次复审未发现新的 P0/P1 问题。上一轮提出的 4 个关键问题已基本收敛：Jar 与服务名绑定已补入上传与校验链路，替换步骤已显式引入 `target_jar_name`，SSH/SCP 认证链路已统一为系统 `ssh`/`scp`，任务创建时也已提前持久化并补充了 Server 重启恢复策略。按当前内容，`plan.md` 已可以进入实现阶段。

## 复审结果

未发现阻塞性问题。

## P2 - 可选优化（锦上添花）

### [P2-1] 路由参数写法仍有少量旧语法残留
- **类型**：文档补充
- **位置**：`spec/ops-deploy-agent/plan.md:51`，`spec/ops-deploy-agent/plan.md:188`，`spec/ops-deploy-agent/plan.md:200`
- **描述**：文档已明确约定使用 Go 1.22+ 路由参数写法 `{param}`，但后文仍残留 `GET /api/tasks/:task_id` 和 `GET /api/tasks/:id` 这种旧式占位符写法。方案本身没有问题，只是文档内部还不完全一致。
- **建议**：统一替换为 `GET /api/tasks/{task_id}`，避免实现阶段复制粘贴接口定义时产生歧义。

### [P2-2] 前端 Jar 管理区可以补一句展示服务名
- **类型**：文档补充
- **位置**：`spec/ops-deploy-agent/plan.md:136-142`，`spec/ops-deploy-agent/plan.md:198-201`
- **描述**：Server 侧已经明确 `GET /api/jars` 返回 `service_name`，但前端 Jar 管理区的描述仍只写“文件名、大小、时间”。这不会影响实现正确性，但会让前端是否展示服务名显得不够明确。
- **建议**：在 Jar 管理区描述里补上“服务名”字段，保持与 Spec 中“服务名 + 文件名”的包列表口径一致。

## 验收标准覆盖检查

| AC 编号 | 描述 | 状态 |
|---------|------|------|
| AC-1 | Agent 启动后 Server 页面显示在线 | ✅ 通过 |
| AC-2 | 正常发布成功 | ✅ 通过 |
| AC-3 | 单台设备失败不影响其他设备 | ✅ 通过 |
| AC-4 | Agent 离线时无法发布 | ✅ 通过 |
| AC-5 | 发布记录可查 | ✅ 通过 |
| AC-6 | 服务以新 Jar 包启动 | ✅ 通过 |
| AC-7 | Agent 断线自动重连 | ✅ 通过 |

## TODO 设计检查

| TODO | 描述 | 状态 |
|------|------|------|
| TODO-S2 | Jar 包上传与文件管理 | ✅ 通过 |
| TODO-S3 | 发布记录 JSON 持久化 | ✅ 通过 |
| TODO-S5 | 发布任务管理与状态机 | ✅ 通过 |
| TODO-S7 | Web 前端单页面 | ⚠️ 部分通过 |
| TODO-A1 | Agent 配置加载 | ✅ 通过 |
| TODO-A5 | SSH 远程替换 | ✅ 通过 |
| 其他 TODO | 方案与依赖拆解 | ✅ 通过 |

## 结论

当前 `plan.md` 已满足进入 `/implement` 阶段的条件。剩余事项都是文档表述级的小修，不影响按此方案落地。
