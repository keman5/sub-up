---
name: fork-sync-deploy
description: 将最新 origin/main 同步并合并到本仓库的 subapi fork，解决冲突时保护已记录的 fork 专属功能与线上非 Git 状态，完成两次完整复查后通过 SSH 部署到已配置的 VPS。用户要求同步、合并、审查、验证或部署 main 和 subapi，或提及 51tokens、fork 需求、合并冲突、线上发布时使用此技能。
---

# Fork 同步与部署

只能在仓库根目录使用本流程。不要创建 worktree。除非用户明确指定其他引用，否则将 `origin/main` 作为来源，将 `subapi` 作为部署目标。

## 必经关卡

不得跳过或调整下列关卡的顺序。任一关卡失败时，不要部署、强制推送、重置或丢弃改动。

1. 确认工作区干净，且没有进行中的 merge、rebase 或 cherry-pick。
2. 拉取 `origin/main` 和 `origin/subapi`，保存 fork 差异快照，快进本地 `subapi`，再将 `origin/main` 合并到其中。
3. 以 `docs/FORK_MAINTENANCE_CN.md` 为主索引，并连同 `docs/fork-maintenance/*.md` 的月度明细作为 fork 专属行为依据，逐一解决冲突。不得盲目选择 `ours` 或 `theirs`。
4. 完成第一次复查：检查主文档 `## 非上游功能恢复清单` 的每个条目，以及月度明细中尚未被后续记录明确废止的每项改动，核对涉及文件、恢复要点和验证步骤。检查合并差异，排除意外删除 fork 文件或被不等价的上游实现覆盖。
5. 运行完整自动验证，再独立进行第二次复查。重新打开清单并比较最终的 `origin/main...HEAD` 差异，不能只看冲突块。
6. 阅读 `docs/VPS_DEPLOY_NOTES.md`、`deploy/README.md` 和 `deploy/local-gzip-binary-deploy.sh` 的部署方案。执行恢复 dry-run，依次部署测试、a1 和主环境，恢复已记录的非 Git 状态，并验证公开健康检查端点。

唯一可接受的部署结论是“所有关卡通过”。缺失、变化或无法确定的 fork 需求均视为失败：部署前停止，说明受影响的清单条目，并先修复或取得用户决定。

## 同步与快照

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

## 两次独立复查

每次复查都要完整阅读 `docs/FORK_MAINTENANCE_CN.md` 的清单部分和 `docs/fork-maintenance/*.md` 的月度明细。对每个编号条目及每项尚未废止的月度改动，检查列出的文件并验证声明的行为。复查证据保存在已忽略的 `tmp/fork-maintenance/reviews/`；应查看其中保存的最终 `fork-delta-name-status.txt`，不能只依赖终端输出。在任务回复中记录简明结果：条目编号、状态、证据及考虑过的上游等价实现。现有辅助脚本仅为护栏，不能替代逐项人工复查。

```bash
# 第一次复查：解决冲突后、完整测试前。
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh audit --review first
make test
make secret-scan

# 第二次复查：测试通过后。重新阅读清单与最终差异。
skills/fork-sync-deploy/scripts/sync-main-into-subapi.sh audit --review second
```

还要运行所有涉及文件发生变化的清单条目所规定的验证命令。存在测试的需求必须运行测试。即使更窄的测试已经通过，前端 fork 行为仍必须运行 `pnpm --dir frontend run typecheck` 和 `pnpm --dir frontend run build`。

继续前必须全部满足：

- `git status --short` 没有未跟踪或已修改的源码、配置文件。
- `git diff --check origin/main...HEAD` 成功。
- `git ls-files -u` 为空。
- `tools/fork-maintenance/fork-maintenance.sh verify-after-upstream` 成功。
- 两份复查报告均覆盖每个已记录 fork 专属条目，且未发现意外删除或覆盖。

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
