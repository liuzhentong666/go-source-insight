# 宿主源码绑定：部署入口隔离验证

本记录只覆盖配置准备工具、包装脚本和挂载边界。正式 API/Web 的更新、项目归属 HTTP、浏览器完整路径流程及真实索引/问答由独立验收负责，不能由本记录替代。测试不修改正式映射、凭据、数据库、模型缓存或索引，不操作交互 CLI 窗口。

## 实现与入口

- `scripts/bind-source.ps1` / `scripts/bind-source.sh` 调用标准库 Go 工具；输入为实际 API 返回的 owner/project ID 和宿主机绝对目录。目录含空格、美元符号及 shell 字符按数据传递。
- 默认状态目录 `.runtime/source-bindings`；可显式选择其他受限、已忽略的专属子目录。每项目固定一个只读容器根，保留 legacy `/sources` 及其他项目。
- 不可变代 JSON 完成写入后，原子替换单个 `source-bindings.compose.json` 入口。中断或替换拒绝不会混合两代；运行容器继续读取原代，新配置需安排 API 更新。
- Windows 新目录仅授予当前用户；已有额外 Allow ACE、未关闭继承或其他所有者拒绝且不改权限。Unix 检查 700 类权限和当前 UID，root 也不接受他人的受限目录。
- 排除项与 host 根拒绝后端不接受的保留名/尾点空格等路径语法。检测本仓库布局时只排除根相对 `sources`、`volumes`，不禁用所有业务同名目录。

## 实际验证

| 检查 | 结果 | 真实边界 |
| --- | --- | --- |
| Windows `go test -count=1 -v ./scripts/source-binding` | exit 0 | 明确 Everyone ACE 拒绝且原 ACL/映射不变；Windows 持有无 delete-share 文件句柄时原入口保留；旧代、凭据、幂等、无效配置、提交失败 |
| Windows 最终 host 语法/幂等专项 | exit 0 | `TestBindingHostSyntaxMatchesAPI`、`TestPreserveBindingsCredentialsAndIdempotency`；正常目录不受新增检查影响 |
| `./scripts/source_binding_test.ps1` | exit 0 | 包装参数、包含特殊字符的目录、合成凭据 Compose 合并、legacy/新增只读挂载；实际 UID 10001 无网络临时容器可读配置/源码且不能写入 |
| Linux 非 root 全工具测试 | exit 0 | 缓存 `golang:1.25.5-alpine3.22`，只读工具挂载，UID 10001；真实符号链接拒绝、不可读目录拒绝、既有宽权限目录不修改 |
| Linux UID 不匹配专项 | exit 0 | 独立 root 临时容器，仅 fixture `chown`；拒绝他人 700 目录且保持数据/权限 |
| Linux 最终 host 语法/幂等/无效配置专项 | exit 0 | 针对最终路径校验修改，不扩大到正式服务 |
| PS/Bash 部署参数回归 | exit 0 | 假 Docker；覆盖绑定文件参数、空格路径、CLI 拒绝、缺文件拒绝、既有凭据复用；默认流程不要求绑定 |
| PS 语法、Bash `-n`、`git diff --check` | exit 0 | 配置与文档静态检查 |

Windows 的符号链接 fixture 因主机创建权限明确跳过，Linux 非 root 的同场景实际通过。Linux 非 root 的 UID 不匹配 fixture 明确跳过，并在独立 root 容器专项实际通过。Windows ACL 测试需要普通用户权限上下文；沙箱 AppContainer 不能读写工具收紧后的目录，不能为此放宽权限。

## 修复时发现的环境问题

早期新 ACL helper 和对应测试曾 exit 1：Go 子进程继承的模块环境未自动解析 Windows PowerShell ACL cmdlet。最终固定从系统 `$PSHOME` 导入 Security 模块。已有目录的测试 ACL 设置曾遇到 `PrivilegeNotHeldException`，fixture 改用系统 `icacls` 加入 Everyone ACE；产品工具仍不改已有目录 ACL。最终 fresh 检查见上表，失败不以旧日志遮盖。

首次 Linux 测试临时文件系统未允许执行测试二进制，exit 1；随后仅为隔离测试临时目录启用 exec，fresh exit 0，未改变宿主或正式容器设置。

## 交接

正式配置生成与最小 API/Web 更新命令见 [源码绑定说明](../SOURCE_BINDINGS.md)。同一 Compose 项目沿用原 `api.env`，新增覆盖文件放在 API/Web 文件之后。只更新 `api web` 并使用 `--no-deps`；不重建 PostgreSQL、Ollama 或 Milvus。换根或排除变化不自动重建索引，必须由用户明确建立索引确认替换。

本轮临时 Docker 容器均以 `--rm` 运行；宿主工具 fixture 留在已忽略的 `bin/`，没有清理用户目录。正式配置生成及实际服务更新交独立验收，部署工具/文档所有权在交接后释放。
