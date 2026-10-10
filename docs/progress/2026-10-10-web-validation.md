# Web 独立验收 — 2026-10-10

验收时项目工作台已在本机正式 Web 容器运行。验收只新增/更新 Web；原 API、PostgreSQL、Milvus、Ollama 和 CLI 问答服务保持运行，未重启、改配置或操作原用户项目的索引。本记录已匿名化本机资源定位信息；尖括号内容为占位，历史结果未重新执行。

## 独立审查和修复

直接检查实际代码、接口契约及变更，没有以既有测试报告代替验收。工作区及父目录未发现适用 AGENTS.md；用户目录的 AGENTS.md 为空。

- 会话 generation 隔离迟到成功/401；刷新先验证 profile；注销/401 清理内存与 sessionStorage；任务按账号和项目恢复，无法伪装成完整服务端历史。
- 项目切换卸载并终止读取/问答等待；已接受任务的迟到响应仍记到正确项目。问答结果、源码、路径、错误按 React 文本渲染。
- 修复迟到 `running` 轮询覆盖 `canceling` 的竞态：Go 时间戳的亚毫秒精度经 JavaScript Date.parse 截断时，时间比较不足；已确认取消中的任务不回退到排队/执行中。新增对应回归测试。
- 修复窄屏固定导航占用大部分主内容的问题；导航自动折叠，提供可用键盘操作的导航与账号按钮，内容间距适配窄屏。触发器移入页头，避免盖住品牌。
- 缺陷候选卡标题改为“返回 N 条待复核候选”，避免直接复用后端“检测到 Bug”的旧 summary。保留低置信度与人工复核说明。
- 明确范围策略：扫描统计生产和测试 Go 文件；静态分析排除测试文件；索引包含测试源码。索引确认明确说明子目录/文件也替换整个项目快照。

## 检查结果

| 检查 | 实际结果 |
| --- | --- |
| Node 24 `npm test` | 17 项通过，0 失败，退出 0 |
| `npm run lint` / `npm run build` | 完成，退出 0；build 含 TypeScript 检查 |
| 空目录标准 `npm ci --prefix <独立验收目录>` | 275 packages 安装完成，退出 0；无 legacy-peer-deps/ignore-scripts |
| Linux 完整多阶段 Web 构建 | npm ci/tsc/Vite 构建成功；最终标准 Compose `build web` 退出 0 |
| 只更新 Web：`up -d --no-deps --wait web` | 退出 0，新容器 healthy |
| 正式 `/workspace/<project-id>` SPA 请求 | HTTP 200；浏览器刷新后账号及已知任务恢复 |

最后运行镜像与本地 Web `:local` 标签一致。API 容器身份及原启动时间在 Web 更新前后相同。

Host build 只有约 1.13 MB 单 chunk 的体积提示，未作为失败。DockerHub 认证网络曾拒绝连接；本机已从部署验收使用的公开镜像源缓存 Node/Nginx 基础镜像，并按相同 image ID 添加官方标签，最后标准无额外 ARG 的构建成功。没有改变全局 Docker 设置。新机器仍需要能访问基础镜像和 npm registry；本次不等于验证任意机器的网络条件。

## 真实浏览器与服务

使用专门 CUA 浏览器 API；已阅读 computer-use 技能及安全指引，没有控制终端窗口。

- 未登录工作台跳转登录；真实注册/登录、项目创建、刷新保持会话通过。
- 独立演示项目绑定目录说明正确；源码不存在时显示真实 source_unavailable。提供虚构 Go 源码后扫描：2 文件、1 生产、1 测试、28 物理行。
- 真实分析通过轮询完成，刷新恢复：1 生产文件、3 函数、0 复杂度/安全提示、1 低置信度缺陷候选；按文件报告和物理行可查看。
- 真实索引确认后完成：2 文件、8 片段；真实 Ollama 问答返回 Greeting 空 name 时的 `Hello, visitor`，引用相对文件、5–11 物理行及原始代码片段。
- 正式 Nginx Web 入口独立完成登录、扫描、分析、SPA 刷新和真实模型问答；取消第二次索引的最终状态为 canceled，已发布快照保留，后续正式问答仍成功。
- 切到独立空项目真实扫描得到 0 文件/0 行，分析/索引按钮禁用，与“未运行”区别明确；无串项目结果。该空项目及目录已清理。
- 桌面与 390px viewport 检查通过；实际内容宽度随滚动条约 375px，document scrollWidth 与 clientWidth 相同。长路径换行，源码片段内部滚动，主要控件可达。截图采用可靠 viewport 截图，工具的 full-page 捕获偶发失败。

## 明确采用受控依赖的场景

独立 loopback mock 服务提供故障响应，其余账号/项目/扫描请求转发真实 API。没有停止 Ollama来模拟故障，没有把合成结果当作真实模型或真实任务。

- 409/project_busy 和 503/ai_unavailable 显示具体可恢复提示。
- 合成任务的 canceling 显示“正在取消，等待服务端确认”，禁用重复取消；刷新先显示恢复占位，查询后到 canceled。
- profile 401 刷新清会话并跳转登录。跨账号迟到成功/401、注销后迟到 profile 不复活、owner/project 任务隔离采用 Axios adapter/控制器回归测试；没有将这些状态回归称为真实 HTTP。
- 注入 HTML 形式的模型答案与源码仍显示字面文本；答案的 img 节点与片段的 script 节点均为 0。

服务端仍无完整 jobs list/流式问答/多轮记忆，界面如实说明。上传、Git 导入和外部模型 API 不在本轮实现范围。

## 依赖审计的边界

标准安装报告的 18 是 npm 的受影响**包条目**（含传递与重复影响），不是证明了 18 个可利用产品漏洞。进一步保存并分类了 npm audit 输出：10 个开发工具包条目、8 个 production 标记条目；severity 汇总为 1 low、6 moderate、11 high。

直接依赖 axios 1.15.0、react-router-dom/react-router 7.14.0、echarts 6.0.0、Vite 8.0.8 与 HEAD 原锁版本相同；本轮没有新增这些依赖或升级版本。锁修复补入原 node_modules 隐藏锁已存在的 Monaco 0.55.1 / DOMPurify 3.2.7 等 peer 元数据，它们原 package.json 已声明，补锁使干净安装也能完整记录/审计。

生产标记条目包括 axios、react-router/dom、echarts、monaco-editor、dompurify、follow-redirects、form-data。当前应用实际使用浏览器 Axios 的固定同源 JSON API 和 BrowserRouter；没有 Node Axios HTTP/proxy/流式 multipart 请求路径、SSR/RSC 反序列化/服务端 Router actions。Monaco、ECharts、DOMPurify 在当前 src 中没有导入。Vite及其相关构建/lint依赖留在 builder，最终容器为静态 Nginx。Axios 的原型污染类与 Router 通用路由类公告仍需后续专门依赖复审；本次没有证明可利用，也没有宣称完全无风险，未自动大范围升级依赖。

> 后续：专门复审已在 [最终验收记录](2026-10-10-final-acceptance.md) 完成，axios 升至 1.20.0、react-router-dom 升至 7.18.4，生产 high 条目清零。

## 留存与清理

保留一个独立演示账号、演示项目、虚构源码及该项目已发布的 8 片段快照；账号凭据仅在受限 ACL 的已忽略文件，没有写入聊天、URL、源码或文档正文。临时空项目及空目录、空目录安装副本已清理；验收用 Vite 和 mock 服务已停止。保留正式 Web 容器，未动其他服务/卷。未 reset/stash/commit 或创建 PR。

桌面、分析、问答及移动端截图保存在受限且已忽略的 `<evidence-dir>` 下；另留上轮真实问答来源与受控取消证据。浏览器会话及登录页面定位不写入维护文档。

## 本机仅更新 Web 的命令

```powershell
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env build web
docker compose -f docker-compose.yml -f docker-compose.api.yml -f docker-compose.web.yml --profile api --profile web --env-file .runtime/api.env up -d --no-deps --wait --wait-timeout 300 web
```

完整新部署使用部署脚本 `-Mode web`，见 Web 部署文档；对现有正式栈，本次特意只操作 Web 服务。部署脚本/隔离 proxy smoke 的独立结果见同日期部署验收记录。
