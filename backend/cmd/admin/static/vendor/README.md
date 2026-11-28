# 管理后台浏览器依赖

以下浏览器发行文件由 `cmd/admin` 同源提供。升级时用对应版本发布包中的文件和许可替换，并运行后台浏览器回归测试。

| npm 包 | 版本 | 发行文件 | 本地文件 |
| --- | --- | --- | --- |
| `htmx.org` | `2.0.11` | `dist/htmx.esm.js` | `htmx.js` |
| `axios` | `1.21.1` | `dist/esm/axios.min.js` | `axios.js` |
| `dompurify` | `3.4.18` | `dist/purify.es.mjs` | `purify.js` |
| `quill` | `2.0.3` | `dist/quill.js`、`dist/quill.snow.css` | `quill.js`、`quill.snow.css` |

许可随各库保存在同目录的 `*.LICENSE*.txt` 文件中。省略了源映射文件及其引用。
