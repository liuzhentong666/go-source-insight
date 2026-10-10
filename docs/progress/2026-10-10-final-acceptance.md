# 本地源码目录与 Web 全链路独立验收 — 2026-10-10

本记录补完 [Web 独立验收](2026-10-10-web-validation.md)、[Web 部署层验证](2026-10-10-web-deployment-validation.md) 和 [宿主源码绑定验证](2026-10-10-source-bindings-deployment-validation.md) 中留给独立验收的项目。被测对象是当日最终工作区代码，包括本地源码目录入口（`local_root`）。它不是已提交版本，验收时工作区仍有未提交改动。本记录已匿名化本机资源定位信息；尖括号内容为占位，历史结果未重新执行。

## 环境与隔离

- 验收使用独立 Compose 项目 `<acceptance-project>`，Web 地址 `127.0.0.1:<web-port>`，API 地址 `127.0.0.1:<api-port>`，均只绑定 loopback。
- PostgreSQL 和 Milvus 使用本次新建的独立命名卷。
- 凭据为新生成的独立文件，Compose 状态目录和源码目录均为本次专用。
- 镜像使用本次验收专属的独立标签，构建不会移动正式环境的 `:local` 标签。
- Ollama 以只读方式挂载既有模型缓存卷，验收期间没有下载、修改模型，也没有并发写缓存。
- 正式栈未被停止、重启或重新配置，它的数据库、索引和凭据也没有被读写。
- 源码目录由真实 `scripts/source-binding --local-root` 工具生成，执行结果为 exit 0。
- 测试源码是虚构的 Go 项目：一个生产文件，一个测试文件。另在 `.runtime/leak.go` 放了一个排除探针，内含标记字符串。

## 检查结果

| 检查 | 方式 | 结果 |
| --- | --- | --- |
| Go `build`、`vet`、`test -count=1`（api/cmd/internal/scripts，17 个包） | 本机 | 全部 exit 0 |
| 隔离栈构建与健康 | 真实 Docker | api/web 镜像构建 exit 0；5 个服务 healthy |
| Compose 合并配置 | `config` 预览 | 端口、卷、镜像标签均隔离；源码和配置只读挂载 |
| HTTP 全链路（经 Web Nginx 代理） | 真实服务 + 真实 Ollama | 25/25 通过 |
| 浏览器全链路 | 真实 headless Chromium（本机缓存的 Playwright，未下载） | 16/16 通过，浏览器控制台无错误 |
| 默认镜像源 | `docker pull` node:24-alpine、nginx-unprivileged:1.29-alpine、golang:1.25.5-alpine3.22、alpine:3.22 | 均 exit 0 |
| 生产依赖复审 | `npm audit --omit=dev` | 处理见下文 |
| 依赖升级后 Web | `npm test`、`lint`，Node 24 容器内干净 `npm ci` + `tsc -b` + `vite build` | 31/31 通过；lint 和构建 exit 0；新容器 healthy |

### HTTP 全链路覆盖

- 注册和登录正常。另一个账号访问本账号的项目或任务时返回 404。
- `source` 接口返回 `source_mode=local`、`available=true`，不暴露容器内部路径。
- 输入宿主机完整路径的目录时，服务端会规范成相对目标 `greet`。
- 扫描整个根目录得到 2 个文件、21 个物理行，`.runtime` 探针不出现在结果中。
- 越界和排除路径被拒绝：`<root>/.runtime` 返回 403 `excluded_target`，`/etc` 和 `../..` 返回 400 `invalid_target`。
- 分析任务完成。索引任务完成，用时 14–18 秒，生成 6 个片段，collection 为 `api_project_<id>`。
- Ollama 真实问答用时 52–89 秒，正确回答空名字时返回 `Hello, visitor`。
- 问答引用仅含相对路径和物理行号，例如 `greet/greet.go` 第 5–12 行，不出现探针内容。
- `ask.file` 指向排除路径时不返回任何来源；指向绝对路径时返回 400。
- 运行中的索引在取消后先进入 `canceling`，最终变为 `canceled`；已发布的快照仍能继续问答。

### 浏览器覆盖

- 未登录访问 `/workspace` 会跳转到登录页。通过表单登录后，可新建项目并跳转到 `/workspace/:id`。
- 页面显示当前本地源码目录。扫描结果列出相对文件，不包含探针文件。
- 刷新 SPA 后会话保持。刷新后 `?tab=tasks` 能恢复到任务页。
- 页面上完成了静态分析、建立索引（含整项目替换确认）和真实问答，用时 48–103 秒。
- 问答引用可展开，显示相对路径和物理行号。任务记录显示本浏览器提交的任务。
- 在 390px 宽度下，`scrollWidth - clientWidth = 0`，没有横向溢出。
- 通过账号菜单退出后，sessionStorage 中的会话令牌被清除；再访问受保护路由会回到登录页。

截图保存在受限且已忽略的 `<evidence-dir>` 下。

### 测试脚本自身的修正

首轮有 3 项失败，原因都在验收脚本，没有改动产品代码：

- 引用路径字段应为 `source`，脚本误读成了其他字段名。
- 任务页状态保存在 URL 的 `?tab=` 中，刷新后源码面板不显示，脚本等待的元素有误。
- 退出入口位于「账号菜单」下拉中，脚本定位错了按钮。

修正脚本后完整重跑，全部通过。

## 依赖复审与升级

升级前，`npm audit --omit=dev` 报告 8 个受影响包条目：4 个 high、4 个 moderate。处理如下：

| 依赖 | 原版本 | 处理 | 理由 |
| --- | --- | --- | --- |
| axios | 1.15.0 | 升级到 1.20.0（同一主版本） | 源码实际 import；它及其传递依赖 follow-redirects、form-data 是 high 告警来源 |
| react-router-dom / react-router | 7.14.0 | 升级到 7.18.4（同一主版本） | 源码实际 import；含 open redirect 等告警 |
| echarts | 6.0.0 | 暂不处理 | `src` 中没有 import，不进入构建产物 |
| monaco-editor / dompurify | 0.55.1 / 3.2.7 | 暂不处理 | 经 `@monaco-editor/react` 引入，`src` 中没有 import |

升级后报告为 0 high、3 moderate，剩余 3 个都属于上表中未被 import 的依赖。改动只涉及 `web/package.json` 和 `web/package-lock.json`。升级前的这两个文件备份在受限且已忽略的 `<evidence-dir>` 下。

不带 `--omit=dev` 的完整 `npm audit` 失败：npm 的旧审计接口返回 400 Bad Request，并提示该接口正在下线。因此开发与构建依赖本次没有完成复审。这些依赖只存在于 Web 构建阶段，最终 Web 容器是静态 Nginx。

## 发现与剩余限制

- 正式栈当日用同一工作区代码更新 API/Web（`deploy.ps1 -Mode web -SourceBindingsComposeFile <既有覆盖文件>`），PostgreSQL、Milvus、Ollama 未重建；容器镜像与 `:local` 标签一致，`/ready`、`/web-health`、SPA 刷新与未认证 401 正常，原只读挂载保留。在 WSL 中用 `deploy.sh --bindings-file` 应用同一覆盖文件失败（exit 1，容器未替换）：该文件由 Windows 侧工具生成，挂载源为 Windows 盘符路径，WSL 侧 Docker CLI 判定为非绝对路径。覆盖文件需在生成它的同一侧 shell 中使用。
- 正式栈当时把整个仓库目录挂载为一个既有项目的源码，这是旧映射。本次验收证实新代码默认排除 `.runtime`、`bin` 等运行目录，但仍建议不要把本仓库当作业务源码目录。
- 当时本机其他服务占用了默认 API 端口，正式栈通过本地 `API_PORT` 覆盖改为空闲端口。部署时应根据实际占用情况选择端口。
- 本次只验证了这一台机器的网络与 Docker 环境，不代表任意机器都可用。没有验证 GPU、远程访问或多副本。

## 留存与清理

后续正式环境已用新的 `-SourceDirectory` 入口启用各项目共用的本地源码根，保留此处记录的旧映射及数据；当前版本独立检查和单次真实网页问答结果见[本地入口部署复核](2026-10-10-current-local-deployment.md)。

以下内容保留，均位于已忽略目录：

- 本次隔离栈及其两个命名卷
- 已忽略的 `<acceptance-dir>` 下的源码、配置、证据、截图和验收脚本
- 测试账号凭据：只存于权限为 600 的已忽略文件，没有写入文档或日志

确认不再需要后，先根据本地资源记录核对归属，再依次清理本次隔离栈的容器、两个测试命名卷和验收目录。不要根据本记录中的占位定位资源；保留正式栈的数据库、索引卷和 Ollama 模型缓存。

本次没有执行 commit、push、reset 或 stash。
