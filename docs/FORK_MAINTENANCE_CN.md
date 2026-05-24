# Fork 维护说明

本文档用于记录本 fork 相对官方仓库的本地修复，并在后续同步官方版本时逐项复查这些修复是否仍然需要保留。

## 维护原则

- 本 fork 只做当前部署必须的 bugfix，不主动扩展功能。
- 每个本地 bugfix 都要记录问题现象、修改文件、验证命令和后续复查方式。
- 同步官方版本后，先检查官方是否已经修复同类问题，再决定保留、移除或重新实现本地补丁。
- VPS 部署应使用本 fork 构建出的镜像或二进制，避免使用官方 `latest` 镜像覆盖本地修复。

## 更新检查注意事项

当前代码里的在线检查更新默认指向官方仓库：

- `backend/internal/service/update_service.go` 中的 `githubRepo = "Wei-Shaw/sub2api"`
- `deploy/install.sh` 中的 `GITHUB_REPO="Wei-Shaw/sub2api"`

如果 VPS 运行的是本 fork，但保留上述默认值，后台“检查更新”和一键更新会以官方 release 为准。对于保留本地补丁的部署，不建议直接点击后台一键更新；应先在本 fork 中同步官方代码、复查补丁、验证通过后再重新构建并部署。

如果希望检查更新也跟随本 fork，需要将上述仓库名改为自己的 GitHub 仓库，并在 fork 中发布对应 release。

## 本地补丁记录

| 日期 | 问题 | 本地修复 | 验证 | 后续同步复查 |
| --- | --- | --- | --- | --- |
| 2026-05-23 | 账号状态为历史值 `disabled` 时，后台账号列表显示翻译 key：`admin.accounts.status.disabled`。 | 在 `frontend/src/components/account/AccountStatusIndicator.vue` 中将旧值 `disabled` 的显示 key 规范化为 `inactive`，复用现有“停用 / Inactive”文案；在 `frontend/src/components/account/__tests__/AccountStatusIndicator.spec.ts` 增加回归测试。 | `pnpm vitest run src/components/account/__tests__/AccountStatusIndicator.spec.ts`；`pnpm typecheck`。 | 同步官方后搜索 `admin.accounts.status.disabled` 和 `normalizeAccountStatusLabelKey`。如果官方已在 API 或组件层将 `disabled` 规范化为 `inactive`，可移除此本地补丁；如果仍可能返回旧值，需要保留或按新结构重做。 |
| 2026-05-24 | Chrome 中后台账号编辑弹窗快速滚动或频繁切换时，偶发出现弹窗内容消失、背景表格露出或关闭时弹窗残影二次出现；Edge 不易复现。 | 在 `frontend/src/components/common/BaseDialog.vue` 中将关闭时的 body 滚动解锁和焦点恢复延后到 Vue leave transition 的 `after-leave`，避免关闭动画期间提前移除 `html/body.modal-open`；在 `frontend/src/style.css` 中保留弹窗进入/关闭动画，同时在 `html.modal-open` 期间禁用背景 glass/table header 的 `backdrop-filter`，规避 Chrome 对固定弹窗、滚动容器和背景模糊层组合时的合成闪烁。 | 基线版本可复现闪烁；仅加入 modal-open 期间禁用背景 `backdrop-filter` 后，3 分钟系统录屏逐帧分析未发现稳定闪烁候选帧；`pnpm --dir frontend run typecheck`；`git diff --check`；浏览器侧确认 ESC 关闭时 `modal-open` 与 leave DOM 同步保留到动画结束。 | 同步官方后搜索 `pendingUnlockAfterLeave`、`handleAfterLeave`、`html.modal-open .glass` 和 `html.modal-open .table-scroll-container thead`。如果官方已重构弹窗或移除了背景 `backdrop-filter` 闪烁根因，需要在 Chrome 中重新执行“打开编辑账号弹窗、快速滚动、频繁切换弹窗、ESC 关闭”的回归测试；确认无闪烁后可删除本地补丁，否则按新结构补回。 |

## 同步官方版本后的复查流程

1. 记录当前 fork 状态：

   ```bash
   git status --short
   git log --oneline -5
   ```

2. 拉取官方仓库更新：

   ```bash
   git remote -v
   git fetch upstream
   ```

3. 合并或 rebase 官方分支前，先确认本地补丁清单：

   ```bash
   sed -n '/## 本地补丁记录/,$p' docs/FORK_MAINTENANCE_CN.md
   ```

4. 合并官方更新：

   ```bash
   git merge upstream/main
   ```

   如果你的维护方式偏向线性历史，也可以使用 `git rebase upstream/main`。遇到冲突时，优先保留官方实现；只有官方仍未解决本地问题时，才保留 fork 补丁。

5. 对每条本地补丁做复查：

   ```bash
   rg -n "admin\\.accounts\\.status\\.disabled|normalizeAccountStatusLabelKey" frontend/src
   pnpm vitest run src/components/account/__tests__/AccountStatusIndicator.spec.ts
   pnpm typecheck
   ```

6. 根据复查结果更新“本地补丁记录”：

   - `官方已修复`：删除本地重复补丁，记录已由官方覆盖。
   - `仍需保留`：保留本地补丁，确认测试仍通过。
   - `需要重做`：官方代码结构已变化，按新结构重新实现最小修复，并补充新的测试路径。

7. 重新构建并部署 fork 版本：

   ```bash
   docker build -t your-registry/sub2api:fork .
   ```

   或使用当前项目既有的发布流程构建二进制。不要直接切回 `weishaw/sub2api:latest`，否则本地补丁会被官方镜像覆盖。

## 新增补丁记录模板

新增本地 bugfix 后，复制下面模板追加到“本地补丁记录”表格或单独展开为小节：

~~~~markdown
### YYYY-MM-DD: 问题标题

**现象：**
描述用户看到的问题、页面、接口或日志。

**原因：**
描述定位到的根因，尽量引用具体文件和 key。

**修改：**
- `path/to/file`
- `path/to/test`

**验证：**
```bash
command
```

**同步官方后的复查：**
说明搜索什么、跑什么测试、什么情况下可以删除本地补丁。
~~~~
