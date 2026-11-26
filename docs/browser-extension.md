# 浏览器扩展

配套扩展用于保存浏览器渲染后的完整页面，适合服务端无法直接抓取的中文站点（知乎、小红书、博客园、论坛等）。

## 功能

- 保存当前页面完整 DOM，而不只是服务端抓到的 HTML；
- 整页滚动，触发懒加载图片；
- 保留 GIF / WebP / 视频；
- 知乎专栏视频、小红书图文和视频；
- 保存到你自己的 Readeck 实例。

## 安装

1. 打开本仓库 Releases，下载最新的 `readeck-web-extension-*.zip`；
2. 解压到固定目录；
3. 打开 `edge://extensions` 或 `chrome://extensions`；
4. 打开“开发者模式”，点击“加载解压的扩展”；
5. 在扩展设置里填写 Readeck 地址和 API Token。

## 权限说明

| 权限 | 用途 |
|---|---|
| `activeTab` / `scripting` | 读取当前页面并执行整页滚动 |
| `tabs` | 获取当前标签页地址 |
| `storage` | 保存 Readeck 地址与设置 |
| `<all_urls>` | 在任意站点保存页面 |

扩展不会把内容发送到 Readeck 以外的服务器。

## 打包

```bash
cd tools/web-extension
zip -r readeck-web-extension.zip .
```

发布时把 zip 作为 GitHub Release 资产上传。
