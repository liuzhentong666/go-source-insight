# 本地源码目录

本项目是私人本机工具。部署时选一次源码目录，之后在 Web 新建或选择项目即可直接填写这个目录内的完整目录或 `.go` 文件路径。无需填写所有者/项目 ID，也无需单独执行绑定步骤。源码仍通过 Docker 只读挂载读取，项目保留独立索引和任务。

## 常用入口

需要 Docker Compose V2、Linux 容器引擎和与根 `go.mod` 兼容的 Go 工具链。在仓库根目录执行，把示例目录替换为已有源码目录：

```powershell
./scripts/deploy.ps1 -Mode web -SourceDirectory 'C:\code\project'
```

```bash
bash scripts/deploy.sh --mode web --source-directory '/workspace/project'
```

只用 API 时，将模式改为 `api` / `--mode api`。已有缓存模型需要启动/检查时，加 `-WithAI` / `--with-ai`；不会下载缺失模型。未传源码目录参数时，原 CLI/API/Web 部署行为保持兼容。

脚本验证目录、保存受限且已忽略的本地配置、生成只读挂载，再构建并更新服务。已有同一 Compose 项目的 API 时，只更新 API/Web，保留 PostgreSQL、模型服务、数据库、旧索引、凭据和原有项目映射。API 更新会进行有界关闭，可能中断在途问答或任务；CLI 窗口不受此入口控制。换源码根不自动索引，必须在正确范围下明确执行「建立索引」并确认整项目替换。

只接受存在、可读的完整目录，拒绝盘根、单文件、符号链接、junction 及祖先中的 reparse point。单文件在网页操作范围中选择，工具不会因输入文件而默默授权整个父目录。目录名中的空格、美元符号和 shell 字符作为数据传递。

## 保存与重复运行

首次配置默认保存到 `.runtime/source-bindings`。已有 API 挂载受控配置时，脚本仅检查其挂载元数据，自动找到正在使用的配置目录，包括曾经选择的非默认目录。它不会读取或打印完整容器环境，也不要求手查 JSON。新装需要其他专属状态目录时，可用 `-SourceStateDirectory PATH` / `--source-state-directory PATH`；已有活动配置仍以该配置目录为准。凭据仍由既有 `-ApiEnvFile` / `--env-file` 机制复用。

工具保存不可变 `source-bindings-<sha256>.json`，最后原子替换 `source-bindings.compose.json` 入口。旧代文件及旧 `bindings` 保留，运行中的容器继续读取原代文件；不会热更新。重复相同目录不改配置。构建或启动失败后，用相同目录重新执行原命令可继续应用相同待部署配置；如果另一个不同配置尚未应用，则拒绝覆盖，先重试原目录或审查待应用配置。无效配置、手工改过的代文件及原子提交失败保留原入口。异常留下锁文件时，确认没有配置进程运行后再人工核对处理，不自动删除其他进程的锁。

新 Unix 状态目录为 700，属当前用户；代 JSON 位于该目录内、权限为 644，使挂载后的 API UID 10001 可读，而其他宿主用户不可遍历目录。新 Windows 目录关闭继承，仅授予当前用户完整访问。已有目录必须属当前用户，Unix 不允许组/其他用户权限，Windows 不允许继承或其他主体的 Allow ACE；不符合时拒绝，不改已有目录及无关数据权限。源码权限不自动扩大；Linux 的实际源码目录和祖先必须允许容器 UID 10001 读取/遍历。

## 读取规则与配置契约

配置新增可选 `local_root`，字段为绝对 `host_root`、绝对 `container_root` 和可选相对 `exclude_paths`，无需身份 ID。Docker 工具的本地容器根固定为 `/bound-sources/local`，JSON 只读挂载到 `/app/source-bindings.json`，`SOURCE_BINDINGS_FILE` 指向它。旧 `bindings` 数组继续兼容，每项保留 `owner_id`、`project_id`、`host_root`、`container_root`、`exclude_paths`。原 `/sources` 只读挂载也保留。

配置本地根后，各项目的完整路径输入都使用本地根；Web 显示同一个本地根。相对 API 目标仍使用此前项目映射或 `SOURCE_ROOT/<owner-id>/<project-id>`，以保留已有调用和 CLI 使用方式。本地根缺失、不可读或输入越界时，完整路径操作明确失败，不退回旧根。来源引用、任务目标和问答文件过滤仍为相对路径。`ask.file` 不能用主机完整路径读取任意文件。

后端默认排除 `.git`、`node_modules`、`dist`、`.runtime`、`bin` 等运行目录。工具识别本仓库根布局后，按根相对位置额外排除 `sources`、`volumes`，避免本仓库运行源码副本和数据混入。其他业务项目中同名目录不因此被全局禁止。排除项禁止绝对路径、`..`、整个根 `.`、设备名与尾点/尾空格别名。高级工具 `scripts/bind-source.ps1` / `scripts/bind-source.sh` 可继续维护旧项目映射，但普通本地使用不需要它。

原生 API 可使用同一 JSON schema，将 `container_root` 配为实际可读的本机目录，并设置 `SOURCE_BINDINGS_FILE`；Docker 的内部路径不能直接作为原生宿主路径使用。配置来源始终是本地文件，浏览器不提交模型地址或任意挂载配置。

## 网页状态与错误

`GET /api/v1/projects/:id/source` 返回 `source_mode`（`local`、`bound`、`legacy`）、`binding_configured`、`absolute_input_allowed`、授权显示根 `host_root`、`available`、可选 `unavailable_code`、`excluded_paths` 和 opaque `binding_id`，不显示内部容器路径。仍需项目归属认证。`binding_id` 只表示当前配置身份，帮助网页清掉旧扫描上下文，不代表索引已更新。

- 尚未配置本地根或旧项目映射：使用上面的部署目录入口，完整路径才可操作。
- `invalid_target`（400）：格式不合法或越出当前显示根，核对范围。
- `excluded_target`（403）：命中排除目录，不能用路径变体绕过。
- `unsafe_path`（403）：目标或祖先为链接/reparse point，选择真实目录。
- `source_unavailable`（422）：目录不存在、挂载缺失或权限不足，核对本地目录与服务挂载；source 查询仍返回 200 和 `available=false`。

## 隔离验证

```powershell
go test -count=1 -timeout=120s ./scripts/source-binding
./scripts/deploy_test.ps1
./scripts/source_binding_test.ps1
```

```bash
bash scripts/deploy_test.sh
```

Go 临时 fixture 覆盖本地根与旧映射保留、非默认状态目录、相同待应用配置重试、不同待应用配置拒绝、原子失败、特殊目录字符、权限和链接拒绝。部署流程测试的 Docker 为模拟；目录验证和配置生成调用真实工具。Docker 挂载验证仅使用本次无网络非 root 临时容器及合成配置。真实 HTTP、浏览器和 AI 验收另行执行，不将模拟部署视为正式部署成功。
