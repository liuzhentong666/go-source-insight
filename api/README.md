# GoSource-Insight API

API 使用根 Go 模块中的扫描、静态分析和 Ollama RAG 服务。Web 工作台通过同源反代使用这些接口，见 [Web 说明](../web/README.md)；上传、Git 拉取和外部模型供应商 API 继续暂缓。扫描和分析不需要 AI 服务在线。

## 本机部署

需要 Docker Compose V2 和 Linux 容器。在仓库根目录运行：

```powershell
./scripts/deploy.ps1 -Mode api -SourceDirectory 'C:\code\project'
# 已有模型缓存时，另外启动/检查 AI 服务；不会下载模型：
./scripts/deploy.ps1 -Mode api -WithAI
```

```bash
bash scripts/deploy.sh --mode api --source-directory '/workspace/project'
bash scripts/deploy.sh --mode api --with-ai
```

本项目是私人本机工具。目录参数校验并只读挂载源码，新建/选择项目后可直接提交该根内完整路径；无需管理员或手填所有者/项目 ID。已有同项目 API 时，目录入口只更新 API 并保留数据库/模型服务。未传目录参数时，默认仅启动 PostgreSQL 和 API；API 就绪不依赖 Ollama/Milvus。`WithAI` / `--with-ai` 复用基础 Compose 的 AI 服务及命名卷，等待健康并通过现有 `doctor` 验证缓存模型、向量维度和 Milvus。缓存缺失时明确失败，不自动下载；需要首次准备模型时先使用原 CLI 部署入口。聊天模型预检仍暂缓。

首次运行生成随机数据库密码和 JWT 密钥到已忽略的 `.runtime/api.env`，重复运行复用文件，不重置凭据。Unix 文件权限为 600；PowerShell 尽量将凭据目录 ACL 限制为当前身份，失败时给出警告。缺失/弱的既有凭据文件会导致失败，不自动修复。也可在调用环境中同时提供 `API_DB_PASSWORD` 和 `API_JWT_SECRET`，此时不生成文件；不要将值放入命令历史、仓库或日志。环境变量优先于文件，必须与已有 PostgreSQL 卷中的凭据匹配。

API 镜像使用根模块构建，以 UID 10001 运行，根文件系统只读；`sources/` 只读挂到 `/sources`。`API_SOURCE_DIR` 可指定已有受控源码根，`API_PORT` 可改变默认 8087 端口。默认只绑定 `127.0.0.1`，PostgreSQL、Ollama、Milvus 均不发布主机端口。不要扩展为多副本；每个数据库及其 Milvus 索引空间仅运行一个 API 实例。

脚本等待核心健康后才宣告就绪。手动管理时始终使用相同配置和凭据文件：

```bash
docker compose -f docker-compose.yml -f docker-compose.api.yml --profile api --env-file .runtime/api.env config --quiet
docker compose -f docker-compose.yml -f docker-compose.api.yml --profile api --env-file .runtime/api.env up -d --wait postgres api
curl --fail http://127.0.0.1:8087/health
curl --fail http://127.0.0.1:8087/ready
docker compose -f docker-compose.yml -f docker-compose.api.yml --profile api --env-file .runtime/api.env stop api postgres
```

只使用环境注入时省略 `--env-file`。`config --quiet` 不打印配置中的密钥；避免将完整 Compose 配置或容器环境检查输出保存/分享。停止命令保留所有命名卷。不要删除凭据文件或重建数据库卷来处理密码不匹配；先核对原凭据。不要执行带卷删除选项的清理命令。

## 注册、登录和项目操作

没有预置用户。下面示例由你选择新账号、邮箱和密码，先注册再登录；已有账号可跳过注册。令牌仅保存在当前终端变量中。

PowerShell 示例：

```powershell
$base = 'http://127.0.0.1:8087'
$username = Read-Host 'New username'
$email = Read-Host 'Email'
$password = [Net.NetworkCredential]::new('', (Read-Host 'New password' -AsSecureString)).Password
$headers = @{}
function Invoke-Api($method, $path, $body) {
    $options = @{ Method=$method; Uri="$base$path"; Headers=$headers }
    if ($null -ne $body) { $options.ContentType='application/json'; $options.Body=($body | ConvertTo-Json -Compress) }
    Invoke-RestMethod @options
}
$null = Invoke-Api Post '/api/v1/users/register' @{username=$username; email=$email; password=$password}
$session = Invoke-Api Post '/api/v1/users/login' @{username=$username; password=$password}
$headers = @{Authorization="Bearer $($session.token)"}
$project = Invoke-Api Post '/api/v1/projects/' @{name='sample-project'; description='Local Go source'}
$projectID = $project.id
$sourceDirectory = Read-Host '已通过部署目录参数挂载的完整源码目录'

Invoke-Api Post "/api/v1/projects/$projectID/scan" @{target=$sourceDirectory}
$analysis = Invoke-Api Post "/api/v1/projects/$projectID/analyze" @{target=$sourceDirectory}
Invoke-Api Get "/api/v1/jobs/$($analysis.job_id)" $null

function Wait-Job($id) {
    for ($attempt=0; $attempt -lt 600; $attempt++) {
        $job = Invoke-Api Get "/api/v1/jobs/$id" $null
        if ($job.status -eq 'completed') { return $job }
        if ($job.status -in @('failed','canceled')) { throw "Job ended: $($job.failure_code)" }
        Start-Sleep -Seconds 2
    }
    throw 'Job is still pending; query it later.'
}
$null = Wait-Job $analysis.job_id
$index = Invoke-Api Post "/api/v1/projects/$projectID/index" @{target=$sourceDirectory}
$null = Wait-Job $index.job_id
Invoke-Api Post "/api/v1/projects/$projectID/ask" @{question='What does Hello return?'; file='main.go'}
# 取消排队/运行任务；已完成任务保持其终态：
Invoke-Api Post "/api/v1/jobs/$($index.job_id)/cancel" @{}
```

Bash 示例需要 `curl` 和 `jq`：

```bash
set -euo pipefail
base=http://127.0.0.1:8087
read -r -p 'New username: ' username
read -r -p 'Email: ' email
read -r -s -p 'New password: ' password; printf '\n'
jq -n --arg u "$username" --arg e "$email" --arg p "$password" '{username:$u,email:$e,password:$p}' |
  curl --fail --silent --show-error -H 'Content-Type: application/json' --data-binary @- "$base/api/v1/users/register" >/dev/null
session=$(jq -n --arg u "$username" --arg p "$password" '{username:$u,password:$p}' |
  curl --fail --silent --show-error -H 'Content-Type: application/json' --data-binary @- "$base/api/v1/users/login")
token=$(printf '%s' "$session" | jq -r .token)
api() { curl --fail --silent --show-error -H "Authorization: Bearer $token" -H 'Content-Type: application/json' "$@"; }
project=$(api --data '{"name":"sample-project","description":"Local Go source"}' "$base/api/v1/projects/")
project_id=$(printf '%s' "$project" | jq -r .id)
read -r -p 'Already mounted absolute source directory: ' source_directory
target_json=$(jq -n --arg target "$source_directory" '{target:$target}')

api --data "$target_json" "$base/api/v1/projects/$project_id/scan"
analysis=$(api --data "$target_json" "$base/api/v1/projects/$project_id/analyze")
analysis_id=$(printf '%s' "$analysis" | jq -r .job_id)
api "$base/api/v1/jobs/$analysis_id"
wait_job() {
  local job status
  for ((attempt=0; attempt<600; attempt++)); do
    job=$(api "$base/api/v1/jobs/$1") || return 1
    status=$(printf '%s' "$job" | jq -r .status)
    case "$status" in completed) printf '%s\n' "$job"; return 0 ;; failed|canceled) printf '%s\n' "$job"; return 1 ;; esac
    sleep 2
  done
  printf '%s\n' 'Job is still pending; query it later.' >&2; return 1
}
wait_job "$analysis_id"
index=$(api --data "$target_json" "$base/api/v1/projects/$project_id/index")
index_id=$(printf '%s' "$index" | jq -r .job_id)
wait_job "$index_id" && api --data '{"question":"What does Hello return?","file":"main.go"}' "$base/api/v1/projects/$project_id/ask"
api --data '{}' "$base/api/v1/jobs/$index_id/cancel"
```

源码 `target` 默认为相对 `.`，相对 API 调用继续使用旧项目映射或 `<SOURCE_ROOT>/<owner-id>/<project-id>`。本地目录入口启用后，各项目可提交该根内完整目录或 `.go` 文件路径，后端转换为相对范围并受控读取；本地根失效或输入越界明确失败，不回退旧根。Linux 实际源码根和祖先需允许 UID 10001 读取/遍历。问答来源和文件过滤保持相对路径；源目录移走仍可查询已发布索引。

### 使用既有本地目录

常用入口为 `./scripts/deploy.ps1 -Mode api -SourceDirectory 'C:\code\project'` 或 `bash scripts/deploy.sh --mode api --source-directory '/workspace/project'`，不需要独立绑定或身份 ID。脚本自动复用活动配置目录和旧项目映射，保留凭据与旧索引；相同待应用配置可直接重试，不同待应用配置拒绝覆盖。完整保存、权限与重试规则见 [本地源码目录](../docs/SOURCE_BINDINGS.md)。

`GET /api/v1/projects/:id/source` 返回 `source_mode`（`local`、`bound`、`legacy`）、有效显示根、可用性、排除项和 opaque `binding_id`，不返回内部容器根。未配置完整路径或越界返回 `invalid_target`；不可用时查询为 `available=false`，操作为 `source_unavailable`。链接/reparse point 为 `unsafe_path`，排除为 `excluded_target`。运行产物按实际根位置排除，不全局禁业务同名目录。配置变化不自动更新索引，需用户明确确认建立新索引。

## 运行语义和配置

`index` 每次替换整个项目的索引：`target: pkg` 发布只包含 `pkg` 的项目快照，之后再次索引 `.` 才恢复整项目范围。同项目 index/ask（含并发 ask）忙时返回 409；不同项目独立。旧快照保留在 Milvus 卷中，逐次索引会增加磁盘占用，历史回滚和清理仍在 TODO。

取消已确认发布的索引不会回滚；发布状态不明返回 `index_publication_unknown`。进程重启将遗留任务标记 `interrupted`，不自动重做，因为提交可能已发生。详细状态、错误、关闭及兼容接口见 [CONTRACT.md](CONTRACT.md)。变更 JWT 密钥使旧令牌失效，需要重新登录；更改数据库密码需同步真实数据库凭据，不能仅编辑环境文件。

原生运行入口为根目录下 `go run ./api`。按 [环境示例](.env.example) 注入 PostgreSQL、强 JWT 密钥和受控源码根；不要从 `api/` 建立嵌套模块。容器中的 `/health` 为存活检查，`/ready` 检查本实例仍接收操作、必需 PostgreSQL 和专用实例会话，不检查可选模型或源码目录。

API 覆盖文件不参与默认 CLI Compose，因此未配置 API 密钥时 `docker compose run --rm insight ...` 仍可使用。环境示例不含真实凭据，源码及本机部署状态均不进入镜像。部署只支持本机单实例；远程访问和多副本不属于当前范围。

## 隔离验收边界

新 PostgreSQL 会从低位项目 ID 开始，API collection 固定为 `api_project_<id>`。不要将全新测试数据库接到已有 Milvus；这可能覆盖相同项目 ID 的索引。独立验收使用另一个 Compose 项目名和独立 PostgreSQL/Milvus 命名卷、源码根及凭据文件。通过临时覆盖文件将测试 Ollama 服务只读挂载到已有 `ollama_data` 卷以复用模型缓存，禁止并发下载/变更缓存；不要让隔离测试生成新的模型下载。复用现有 Ollama 网络时，为测试 Milvus 配置唯一端点，避免两个网络中同名 `milvus` 别名误连。测试卷/容器清理只操作明确属于测试栈的数据，不处理用户已有卷。真实注册、索引问答、信号、会话丢失及重启验证结果见 [独立验收记录](../docs/progress/2026-10-09-api-validation.md)。
