# Readeck 中文定制版

> 基于 [Readeck](https://readeck.org) 的定制分支，专注于中文网站收藏、长文阅读和离线归档体验。
> 上游项目：<https://codeberg.org/readeck/readeck>，许可证 AGPL-3.0。

这是一个自托管的书签与“稍后读”应用，保留了 Readeck 的隐私和长期归档设计，并针对中文站点补了一批实用功能。

## 主要改动

### 中文站点

- 知乎专栏 / 问答：使用知乎 App UA 取正文，专栏视频通过 lens API 直链下载为本地 MP4；
- 小红书：图文笔记、视频笔记、封面与话题标签；
- 微信公众号、博客园、开源中国、52pojie 等站点的正文与编码处理；
- Discuz 论坛（含 `sehuatang.net`）从 `td.t_f` 提取帖子正文；
- B 站、AcFun、小红书视频：yt-dlp 下载、H.264/AAC 兼容转码、Range 播放。

### 阅读与整理

- 标题 / 站点·作者·日期 / `#标签` / 正文的阅读页布局；
- 标签行同时展示书签标签和正文中的 `#话题#`；
- 图片按 1 / 2 / 3 列切换，普通相册和 AI 整理后的图片网格都支持；
- 原地编辑正文、复制 Markdown、自定义封面、图片网格；
- 批量选择、标签、归档、删除、移动文件夹；
- 多级文件夹、文件夹密码锁定（应用层访问控制）。

### AI 助手（可选）

支持 OpenAI、DeepSeek、Moonshot、SiliconFlow、Ollama、Anthropic 和 OpenAI 兼容服务。

- 文章侧栏：总结、按层级整理正文、生成标签、自由问答；
- 列表侧栏：总结当前筛选、给选中书签打标签、提问并返回匹配书签；
- 整理正文时保留标题层级、列表、引用和图片，支持“每行 N 张图”；
- 可配置独立视觉模型，识别归档图片中的文字；
- AI 结果可配合筛选和删除操作，删除前需要用户确认。

### 浏览器扩展

`tools/web-extension/` 是配套的 MV3 扩展：

- 保存浏览器渲染后的完整 DOM，适合服务端抓不到的中文站点；
- 整页滚动、懒加载图片、GIF 原图、UTF-8；
- 知乎专栏 `video-box`、小红书图文/视频；
- 保存后由服务端继续归档图片和视频。

安装方式：

1. 从本仓库 Releases 下载最新 `readeck-web-extension-*.zip`；
2. 解压到固定目录；
3. 打开 `edge://extensions` 或 `chrome://extensions`；
4. 开启“开发者模式” → “加载解压的扩展”；
5. 在扩展设置中填写自己的 Readeck 地址和 API Token。

> 扩展只把页面内容发到你填写的 Readeck 实例，不经过第三方服务器。

详细说明：[docs/browser-extension.md](docs/browser-extension.md)。

## 快速开始

### Docker

```bash
docker build -t readeck-custom .
docker run -d \
  --name readeck \
  -p 8000:8000 \
  -v readeck-data:/readeck \
  readeck-custom
```

也可以使用本仓库 GitHub Actions 构建的镜像，把 `<owner>` 替换为实际仓库所有者：

```bash
docker pull ghcr.io/<owner>/readeck:custom
```

首次启动后创建账号，在“个人资料 → AI”里按需配置模型。

### 从源码构建

需要 Go 1.24+、Node.js 20+：

```bash
cd web
npm install
npm run build

cd ..
go build -o readeck .
./readeck serve
```

前端使用了 templ 生成代码；修改 `.templ` 后运行：

```bash
templ generate
```

## 配置

常用环境变量：

| 变量 | 说明 |
|---|---|
| `READECK_SERVER_HOST` | 监听地址，默认 `127.0.0.1` |
| `READECK_SERVER_PORT` | 端口，默认 `8000` |
| `READECK_DATA_DIRECTORY` | 数据目录 |
| `READECK_DATABASE_URL` | 数据库连接串（SQLite/PostgreSQL） |

完整配置见 [Readeck 文档](https://readeck.org/en/docs/)。

### AI 设置

在“个人资料 → AI”中填写：

- 服务商（OpenAI / DeepSeek / SiliconFlow / Ollama / Anthropic / 自定义）；
- Base URL；
- 模型名；
- API Key；
- 可选：视觉模型，例如 `Qwen/Qwen3-VL-8B-Instruct`。

保存后可用“测试连接”验证。密钥只保存在实例数据库中，不会写入仓库。

## 浏览器扩展开发

```bash
cd tools/web-extension
# 修改 popup.js / manifest.json
# 重新打包：
zip -r readeck-web-extension.zip .
```

扩展权限只有 `activeTab`、`scripting`、`storage`、`tabs` 和 `<all_urls>`，用于读取当前页面并提交到你配置的实例。

## 隐私

- 所有书签、图片、视频和正文保存在你自己的 Readeck 实例；
- AI 功能只向你在设置中配置的模型服务发送必要文本/图片 URL；
- 仓库不包含任何实例地址、API Token、账号密码或个人数据；
- 文件夹密码是应用层访问控制，不是对归档 zip 的加密；如需磁盘级加密，请自行加密数据目录。

## 与上游的关系

本项目是 Readeck 的 fork，不是上游官方版本。上游版权归 Readeck 作者所有；本仓库新增代码同样以 AGPL-3.0 发布。

- 上游：<https://codeberg.org/readeck/readeck>
- 问题与改动记录：`docs/` 或提交历史

## License

AGPL-3.0-only. See [LICENSE](LICENSE).
