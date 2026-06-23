# 风控中心关键词库维护指南

本文记录如何为“风控中心 > 内容审计设置 > 关键词”维护本地关键词库，用于在请求到达上游模型前做前置拦截。

## 一、推荐策略

不要直接全量导入网上几万条敏感词。关键词匹配是强规则，误拦截会直接影响正常用户。建议按下面三层启用：

1. **硬拦截词库**：高置信、明确不希望发送给上游的词或短语，直接放入风控中心。
2. **观察词库**：政治、舆情、历史事件、平台规则绕过等容易误伤的词，先在内测环境或观察模式验证。
3. **语义审计**：关键词无法判断上下文时，再使用“关键词 + API”策略接入内容审计 API。

线上建议配置：

```text
开启内容审计：开启
模式：前置拦截
关键词拦截模式：仅关键词
违禁关键词：一行一个词或短语
```

如果需要语义审计，关键词拦截模式可以改为“关键词 + API”，但会增加审计 API 调用。

## 二、本仓库内置种子词库

种子词库位于：

```text
docs/risk-control-keywords/
```

可直接复制的整合版：

| 文件 | 用途 | 建议 |
| --- | --- | --- |
| `copy-main-multilingual.txt` | 中文 + 英文 + 日语，不含政治类 | 主环境优先复制 |
| `copy-all-with-political-multilingual.txt` | 中文 + 英文 + 日语，含政治类 | a1/a2 或确认策略后使用 |
| `copy-main.zh.txt` | 不含政治类的主环境建议包 | 主环境优先复制 |
| `copy-all-with-political.zh.txt` | 含政治类的全量包 | a1/a2 或确认策略后使用 |

分类维护文件：

| 文件 | 用途 | 建议 |
| --- | --- | --- |
| `csam.zh.txt` | 未成年人性内容、相关交易和诱导 | 主环境硬拦截 |
| `csam.en.txt` / `csam.ja.txt` | 英文/日语未成年人性内容、相关交易和诱导 | 主环境硬拦截 |
| `violence-weapons.zh.txt` | 暴恐、武器、爆炸物、攻击教程 | 主环境硬拦截 |
| `violence-weapons.en.txt` / `violence-weapons.ja.txt` | 英文/日语暴恐、武器、爆炸物、攻击教程 | 主环境硬拦截 |
| `cybercrime.zh.txt` | 撞库、盗号、木马、钓鱼、绕过风控 | 主环境硬拦截 |
| `cybercrime.en.txt` / `cybercrime.ja.txt` | 英文/日语网络攻击和滥用自动化 | 主环境硬拦截 |
| `fraud-privacy.zh.txt` | 诈骗、洗钱、伪造证件、社工库、隐私泄露 | 主环境硬拦截 |
| `fraud-privacy.en.txt` / `fraud-privacy.ja.txt` | 英文/日语诈骗、隐私泄露和身份滥用 | 主环境硬拦截 |
| `prompt-bypass.zh.txt` | 绕过规则、越狱、隐藏真实目的 | 建议启用 |
| `prompt-bypass.en.txt` / `prompt-bypass.ja.txt` | 英文/日语绕过规则、越狱、隐藏真实目的 | 建议启用 |
| `political-sensitive.zh.txt` | 政治敏感、政治动员、国家安全、分裂极端等 | 建议 a1/a2 先观察或灰度 |
| `political-sensitive.en.txt` / `political-sensitive.ja.txt` | 英文/日语政治敏感、政治动员、国家安全、分裂极端等 | 建议 a1/a2 先观察或灰度 |

所有 `.txt` 都是一行一个词或短语，不包含注释，方便直接复制到后台。日常复制优先使用 `copy-main-multilingual.txt` 或 `copy-all-with-political-multilingual.txt`，分类文件主要用于后续增删维护。

## 三、合并去重

可以用脚本按需合并多个分类：

```bash
node scripts/build-risk-control-keywords.mjs \
  docs/risk-control-keywords/csam.zh.txt \
  docs/risk-control-keywords/violence-weapons.zh.txt \
  docs/risk-control-keywords/cybercrime.zh.txt \
  docs/risk-control-keywords/fraud-privacy.zh.txt \
  docs/risk-control-keywords/prompt-bypass.zh.txt
```

输出默认写到标准输出，可直接重定向：

```bash
node scripts/build-risk-control-keywords.mjs docs/risk-control-keywords/*.zh.txt \
  > /tmp/sub2api-risk-keywords.txt
```

如果政治类只想在 a1/a2 测试，不要把 `political-sensitive.zh.txt` 合并进主环境。

主环境多语言建议包可用：

```bash
node scripts/build-risk-control-keywords.mjs \
  docs/risk-control-keywords/csam.zh.txt docs/risk-control-keywords/csam.en.txt docs/risk-control-keywords/csam.ja.txt \
  docs/risk-control-keywords/violence-weapons.zh.txt docs/risk-control-keywords/violence-weapons.en.txt docs/risk-control-keywords/violence-weapons.ja.txt \
  docs/risk-control-keywords/cybercrime.zh.txt docs/risk-control-keywords/cybercrime.en.txt docs/risk-control-keywords/cybercrime.ja.txt \
  docs/risk-control-keywords/fraud-privacy.zh.txt docs/risk-control-keywords/fraud-privacy.en.txt docs/risk-control-keywords/fraud-privacy.ja.txt \
  docs/risk-control-keywords/prompt-bypass.zh.txt docs/risk-control-keywords/prompt-bypass.en.txt docs/risk-control-keywords/prompt-bypass.ja.txt
```

## 四、开源词库参考

可参考以下开源项目，但不要未审核全量导入：

- <https://github.com/SpaceGather/worldwide-sensitive-word-collection>：多语言敏感词集合，中文覆盖较多，适合作为人工筛选来源。
- <https://github.com/LDNOOBW/List-of-Dirty-Naughty-Obscene-and-Otherwise-Bad-Words>：老牌多语言脏词列表，适合作为低俗/辱骂类补充。
- <https://github.com/censor-text/profanity-list>：多语言 profanity 词库，可补充英文和部分中文。
- <https://github.com/wordpress/openverse-sensitive-terms>：Openverse 搜索敏感词，偏媒体搜索场景，可参考。
- <https://citizenlab.ca/repository-censored-sensitive-chinese-keywords-13-lists-9054-terms/>：中文敏感关键词研究集合，政治类和历史类很多，误杀极高，只建议人工挑选。

## 五、维护原则

- 优先写短语，不写过短单字。
- 优先写“行为 + 意图”组合，例如“教程、脚本、模板、交易、绕过、批量、生成”等。
- 政治类词库单独维护，避免和暴恐、诈骗等硬拦截词混在一起。
- 每次新增词后，先在 a1/a2 看拦截日志，确认误杀率再推广到主环境。
- 后台最多支持约 10000 个关键词，单个关键词最长约 200 字符；数量够用，但不代表应该塞满。
