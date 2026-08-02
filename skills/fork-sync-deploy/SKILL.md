---
name: fork-sync-deploy
description: 先在 GitHub 将 fork 的 main 与上游同步，再将最新 origin/main 合并到本仓库的 subapi fork；解决冲突时以维护和部署记录逐项恢复全部非 fork 功能及线上非 Git 状态，完成独立的两次完整复查后才通过 SSH 部署到已配置的 VPS。用户要求同步、合并、审查、验证或部署 main 和 subapi，或提及 51tokens、fork 需求、非 fork 功能、合并冲突、线上发布时使用此技能。
---

# Fork 同步与部署

只能在仓库根目录使用本流程。不要创建 worktree。除非用户明确指定其他引用，否则将 `origin/main` 作为来源，将 `subapi` 作为部署目标。

## 必经关卡

不得跳过或调整下列关卡的顺序。任一关卡失败时，不要部署、强制推送、重置或丢弃改动。

1. 确认工作区干净，且没有进行中的 merge、rebase 或 cherry-pick。
2. 通过 GitHub Fork 的 `merge-upstream` 接口将 fork 的 `main` 同步到上游 `main`。重新拉取 `origin/main`，并验证其 SHA 与 GitHub 上游 `main` 相同；不能以本地旧的 `origin/main` 代替此关卡。
3. 拉取 `origin/subapi`，保存 fork 差异快照，快进本地 `subapi`，再将已验证的 `origin/main` 合并到其中。
4. 以完整部署记录集为唯一验收基线：`docs/FORK_MAINTENANCE_CN.md`、`docs/fork-maintenance/*.md`、`docs/VPS_DEPLOY_NOTES.md`、`deploy/README.md` 与实际部署脚本。将主文档清单、月度明细中尚未明确废止的记录，以及标记为“非 fork 需求”的记录逐项编入本次审查表后，才可解决冲突。不得盲目选择 `ours` 或 `theirs`。
5. 逐项修复审查表中缺失、被覆盖、行为不等价或验证失败的全部非 fork 功能。每项都要核对入口、涉及文件、预期行为、上游等价实现、修复提交和验证命令；不接受“文件仍在”或“没有冲突”作为通过证据。
6. 完成第一次复查：重新从部署记录集建立审查表，检查每项的代码和线上非 Git 状态，运行其规定验证。检查最终合并差异，排除意外删除、被不等价上游实现覆盖、或只恢复局部 UI 而未恢复 API/状态回写等情形。
7. 运行完整自动验证，再独立进行第二次复查。第二次复查必须重新阅读所有记录、重新比对 `origin/main...HEAD` 和审查表；不得复制第一次结论或只看冲突块。所有条目均已修复且两次复查均通过后，才可阅读部署方案并部署。
8. 阅读 `docs/VPS_DEPLOY_NOTES.md`、`deploy/README.md` 和 `deploy/local-gzip-binary-deploy.sh` 的部署方案。执行恢复 dry-run，依次部署测试、a1 和主环境，恢复已记录的非 Git 状态，并验证公开健康检查端点。

唯一可接受的部署结论是“所有关卡通过”。缺失、变化或无法确定的非 fork 需求，以及缺少目的、验证或复查说明的部署记录，均视为失败：部署前停止，说明受影响的清单条目，并先补齐记录、修复功能或取得用户决定。

## GitHub 上游同步、本地合并与快照

先执行 GitHub 上游同步；该命令会从 `origin` 解析 fork 仓库，确认它确实是 GitHub fork，调用 `POST /repos/{owner}/{repo}/merge-upstream` 更新 `main`，再拉取并核对 fork 与上游的提交 SHA。需要已登录且具备该 fork 写权限的 GitHub CLI。此步骤会修改 GitHub 上的 fork `main`，所以不能在仅需本地审查的场景执行。

```bash
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh github-sync --apply
```

正常同步合并时不必单独运行该命令：下面的 `sync --apply` 会强制先完成同一 GitHub 同步与 SHA 校验，无法跳过。只有成功更新且验证 `origin/main` 后，脚本才会继续本地 `subapi` 合并。

使用 `--apply` 运行辅助脚本。它会在修改 `subapi` 前创建本地 `refs/fork-sync-backups/...` 安全引用，并在已忽略的 `tmp/fork-maintenance/` 下生成二进制补丁快照。

```bash
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh sync --apply
```

如果因冲突停止，使用 `git diff --name-only --diff-filter=U` 查看所有未合并文件。编辑前阅读相关清单条目。兼容时应同时保留上游改动与已记录的 fork 结果。解决所有文件后：

```bash
git add <已解决文件>
git diff --cached --check
git commit
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh audit --review first
```

除非用户要求放弃本次同步，否则不要使用 `git merge --abort`。不要推送 `subapi`；部署使用本地已验证的合并提交。

## 非 fork 功能审查表

“非 fork 功能”指本地产品、运营、兼容性或线上部署需求，不因其位于 fork 代码、部署配置、数据库状态或静态资源而改变验收标准。例如管理端用户备注隔离、账号列表刷新真实用量后回写运行态、路由/计费行为、Pages 回源拓扑和 VPS 共享数据层都必须按记录验收。

在第一次复查前，在 `tmp/fork-maintenance/reviews/<timestamp>-first/` 建立并填写 `non-fork-requirements.tsv`。每行必须含有：唯一编号、记录文件与行号、需求摘要、入口/涉及文件、预期行为、上游等价性结论、修复状态、验证命令及结果、复查证据路径。不得合并多个独立行为到一行。

处理每一行时：

1. 从记录重新理解需要保留的结果，而非按旧文件名选边。
2. 在最新上游结构中定位等价实现；上游仅有相似字段、组件或测试不代表等价。
3. 功能缺失、覆盖或不等价时，先在本次合并中修复，再运行该条目规定的测试和必要的端到端/线上前置检查。
4. 标记为废弃的能力只能在记录明确说明废弃范围、清理位置及替代/无替代结论后移除；不得把未确认功能当作废弃。

记录包含 `TODO`、缺少验证命令、没有复查方式，或不能判断是否已被后续记录废止时，该条目不合格并阻断部署。先补齐记录和实现，再重新开始该条目的两次复查。

## 两次独立复查

每次复查都要完整阅读部署记录集，并逐行检查审查表。第一次复查在完整测试前完成；修复全部失败项并通过完整测试后，第二次复查必须由记录重新建立审查表后独立完成。对每个编号条目及每项尚未废止的月度改动，检查列出的文件、API/运行态边界、部署状态并验证声明的行为。复查证据保存在已忽略的 `tmp/fork-maintenance/reviews/`；应查看其中保存的最终 `fork-delta-name-status.txt`、`deployment-record-sources.txt` 与 `non-fork-requirements.tsv`，不能只依赖终端输出。在任务回复中记录简明结果：条目编号、状态、修复、验证证据及考虑过的上游等价实现。现有辅助脚本仅为护栏，不能替代逐项人工复查。

```bash
# 第一次复查：解决冲突后、完整测试前。先逐项填写并修复审查表。
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh audit --review first
make test
make secret-scan

# 第二次复查：测试通过后。重新从部署记录建立审查表，再复查最终差异。
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh audit --review second
```

还要运行所有涉及文件发生变化的清单条目所规定的验证命令。存在测试的需求必须运行测试。即使更窄的测试已经通过，前端 fork 行为仍必须运行 `pnpm --dir frontend run typecheck` 和 `pnpm --dir frontend run build`。

继续前必须全部满足：

- `git status --short` 没有未跟踪或已修改的源码、配置文件。
- `git diff --check origin/main...HEAD` 成功。
- `git ls-files -u` 为空。
- `tools/fork-maintenance/fork-maintenance.sh verify-after-upstream` 成功。
- 两份复查报告均覆盖部署记录集中的每个非 fork 条目；不存在 `TODO`、未定义验证或未判定废止的记录。
- 审查表的每项均为已修复且验证通过，并且第二次复查独立确认没有意外删除、覆盖或行为降级。

## 部署与恢复

先检查 dry-run 方案。除非用户指定其他主机，否则保持默认 SSH 别名 `51tokens`。不要打印密钥、`.env`、数据库凭据或 API 令牌。

```bash
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --host 51tokens
deploy/local-gzip-binary-deploy.sh --host 51tokens
```

只有两份方案都确认目标主机、镜像和发布顺序符合预期时，才能部署已验证的本地提交：

```bash
deploy/local-gzip-binary-deploy.sh --apply --deploy --host 51tokens
tools/fork-maintenance/fork-maintenance.sh reapply-production-state --host 51tokens --apply
```

部署脚本会备份 Compose 文件，按测试、a1、主环境的顺序部署，并检查本地和公开健康检查端点。恢复非 Git 状态后，再次检查全部已报告端点。测试、健康检查或恢复任一步骤失败时，停止发布并保留已生成的备份路径用于恢复；不要继续下一个环境。

## 最终报告

说明拉取的来源和目标 SHA、合并提交 SHA、冲突文件及解决结论、每项 fork 清单结果、自动检查、部署镜像标签、发布顺序和公开健康检查结果。明确说明是否推送了 `subapi`；除非用户另行要求，本流程不会推送。
