# Check Markdown Links

We use [markdown-link-check](https://github.com/tcort/markdown-link-check) to check all the links in the markdown files.

To install on your Mac

```bash
npm install -g markdown-link-check
```

To test a single file, enter the following in the project root directory.

```bash
$ markdown-link-check doc/run_with_maxcompute.md -c scripts/markdown_link_check/config.json
```

We can see the following example output.
```
FILE: doc/run_with_maxcompute.md
[✓] https://www.alibabacloud.com/product/maxcompute
[✓] https://usercenter.console.aliyun.com/#/manage/ak
[✓] https://workbench.data.aliyun.com/console?#/
[✓] https://www.alibabacloud.com/help/doc-detail/34951.htm
[✓] https://workbench.data.aliyun.com/console#/
[✓] https://www.alibabacloud.com/help/doc-detail/74293.htm
```

To test all markdown files, enter the following in the project root directory.

```bash
find . -name \*.md -exec markdown-link-check -c scripts/markdown_link_check/config.json {} \;
```
