# 风控中心种子关键词

这些文件是一行一个关键词或短语，方便复制到后台“风控中心 > 内容审计设置 > 关键词”。

优先使用下面两份整合版：

```text
copy-main-multilingual.txt
copy-all-with-political-multilingual.txt
copy-main.zh.txt
copy-all-with-political.zh.txt
```

推荐主环境直接复制 `copy-main-multilingual.txt`。如果需要包含政治类，再复制 `copy-all-with-political-multilingual.txt`。

`copy-main.zh.txt` 和 `copy-all-with-political.zh.txt` 是中文单语版，适合只想先测试中文命中的环境。

分类文件保留用于后续维护：

```text
csam.zh.txt
csam.en.txt
csam.ja.txt
violence-weapons.zh.txt
violence-weapons.en.txt
violence-weapons.ja.txt
cybercrime.zh.txt
cybercrime.en.txt
cybercrime.ja.txt
fraud-privacy.zh.txt
fraud-privacy.en.txt
fraud-privacy.ja.txt
prompt-bypass.zh.txt
prompt-bypass.en.txt
prompt-bypass.ja.txt
```

`political-sensitive.zh.txt`、`political-sensitive.en.txt`、`political-sensitive.ja.txt` 误杀风险更高，建议先在 a1/a2 或观察模式验证。
