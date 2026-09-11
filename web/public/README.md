# public/

这个目录下的文件会被原样服务到网站根路径，**不经过打包**。

## 换 logo

把图片放在这里，命名为 `logo.png`：

```
web/public/logo.png
```

访问地址是 `/logo.png`。放上去之后无需改任何代码，以下位置会自动生效：

| 位置 | 尺寸 |
|------|------|
| 侧边栏顶部 | 26px |
| 登录页 | 42px |
| 欢迎页 | 60px |
| 分享页顶部 | 26px |
| 浏览器标签页图标（favicon） | — |

### 说明

- **支持格式**：`.png` 最稳妥。若要改用其它格式（如 `logo.svg`），需要同时修改：
  - `web/index.html` 里的 `<link rel="icon">`
  - `web/src/components/BrandMark.vue` 里的 `LOGO_SRC`
- **文件不存在时**会自动退回「配」字方块，不会显示裂图。
  注意 dev server 对未命中的路径会回落到 `index.html`，所以此时
  `/logo.png` 返回的是 **200 + `Content-Type: text/html`** 而不是 404 ——
  浏览器解码失败会触发 `<img>` 的 `error`，仍然会正确退回字块。
  排查「logo 没显示」时**看 Content-Type，不要看状态码**，200 不代表图片被拿到了。
- **建议尺寸**：正方形，512×512 或更大。显示时会按容器大小等比缩放（`object-fit: contain`）。
- **背景**：用图片时容器背景是透明的，所以 logo 自带底色或做成透明底都可以。
  如果读起来在深色模式下看不清，改 `BrandMark.vue` 里的 `.brand-mark` 样式即可。
- **深色模式**：如果明暗两种主题需要不同的 logo，告诉我，我给 `BrandMark` 加两套图。

### 其它静态资源

需要对外暴露的其它文件（robots.txt、验证文件等）同样放这里，按 `/文件名` 访问。
不需要对外暴露的图片请放在 `src/assets/`，那样会走打包并带 hash。
