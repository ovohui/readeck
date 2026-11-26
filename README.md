# Readeck Chinese Custom Build / 中文定制版

> A self-hosted bookmark and read-later app for Chinese websites, based on [Readeck](https://readeck.org).
> 基于 [Readeck](https://readeck.org) 的自托管中文书签与“稍后读”定制版。

Upstream / 上游：<https://codeberg.org/readeck/readeck> · License / 许可证：AGPL-3.0

---

## Overview / 概述

**English** — This fork keeps Readeck's privacy-first, long-term archiving model and adds practical support for Chinese sites, a reading layout designed for long articles, optional AI tools, nested folders with password locks, and a companion browser extension.

**中文** — 这个分支保留 Readeck 隐私优先、长期归档的设计，补上了中文站点抓取、长文阅读布局、可选 AI 工具、多级加密文件夹，以及配套浏览器扩展。

---

## Features / 功能

| Area / 模块 | English | 中文 |
|---|---|---|
| Chinese sites / 中文站点 | Zhihu columns and answers, Xiaohongshu notes, WeChat articles, Discuz forums, 52pojie, Bilibili/AcFun video | 知乎专栏/问答、小红书、公众号、Discuz 论坛、52pojie、B站/AcFun 视频 |
| Reading layout / 阅读布局 | Title, site · author · date, labels, then article body | 标题、站点·作者·日期、标签、正文 |
| Images / 图片 | 1/2/3 columns, photo galleries, custom cover | 1/2/3 列、相册、自定义封面 |
| Organization / 整理 | Nested folders, batch actions, labels, archive/favorites | 多级文件夹、批量操作、标签、归档/收藏 |
| Folder lock / 文件夹锁 | bcrypt password + session unlock; app-level access control | bcrypt 密码 + 会话解锁；应用层访问控制 |
| AI assistant / AI 助手 | Summary, clean article, labels, chat, list assistant, optional vision model | 总结、整理正文、标签、问答、列表助手、可选视觉模型 |
| Video / 视频 | yt-dlp download, H.264/AAC transcode, Range playback | yt-dlp 下载、H.264/AAC 转码、Range 播放 |
| Export / 导出 | Markdown copy, EPUB, OPDS | Markdown 复制、EPUB、OPDS |
| Extension / 扩展 | MV3, full rendered DOM, lazy images, GIF, Zhihu video | MV3、渲染后 DOM、懒加载、GIF、知乎视频 |

---

## Quick start / 快速开始

### Docker

```bash
docker build -t readeck-custom .
docker run -d \
  --name readeck \
  -p 8000:8000 \
  -e READECK_SERVER_HOST=0.0.0.0 \
  -v readeck-data:/readeck \
  readeck-custom
```

Or use the image built by this repository's GitHub Actions:

也可以使用本仓库 GitHub Actions 构建的镜像：

```bash
docker pull ghcr.io/<owner>/readeck:custom
```

Create an account after the first start. AI is optional.

首次启动后创建账号；AI 功能是可选的。

### Build from source / 从源码构建

Requires Go 1.24+ and Node.js 20+.

需要 Go 1.24+ 和 Node.js 20+。

```bash
cd web
npm install
npm run build

cd ..
go build -o readeck .
./readeck serve
```

Templates use templ; after editing `.templ` files:

修改 `.templ` 后需要重新生成：

```bash
templ generate
```

### Configuration / 配置

| Variable / 变量 | Description / 说明 |
|---|---|
| `READECK_SERVER_HOST` | Listen address, default `127.0.0.1` / 监听地址 |
| `READECK_SERVER_PORT` | HTTP port, default `8000` / 端口 |
| `READECK_DATA_DIRECTORY` | Data directory / 数据目录 |
| `READECK_DATABASE_URL` | SQLite or PostgreSQL URL / 数据库连接串 |

See the [Readeck documentation](https://readeck.org/en/docs/) for all options.

---

## AI / AI 设置

**English** — Supports OpenAI, DeepSeek, Moonshot, SiliconFlow, Ollama, Anthropic, and OpenAI-compatible endpoints. Configure the provider, base URL, model, API key, and an optional vision model under Profile → AI. The vision model is only used for reading text from images; a text-only model such as DeepSeek cannot see images.

**中文** — 支持 OpenAI、DeepSeek、Moonshot、SiliconFlow、Ollama、Anthropic 和 OpenAI 兼容服务。在“个人资料 → AI”中填写服务商、Base URL、模型、API Key，以及可选的视觉模型。视觉模型只用于识图；DeepSeek 这类纯文本模型不能看图。

Example vision model:

```text
Qwen/Qwen3-VL-8B-Instruct
```

Keys are stored only in your instance database. They are never committed to this repository.

密钥只保存在你的实例数据库中，不会写入本仓库。

---

## Browser extension / 浏览器扩展

The MV3 extension in [`tools/web-extension/`](tools/web-extension/) saves the fully rendered DOM, which helps on sites that block server-side fetching.

[`tools/web-extension/`](tools/web-extension/) 里的 MV3 扩展会保存浏览器渲染后的完整 DOM，适合服务端抓不到的中文站点。

- Full-page scroll and lazy images / 整页滚动、懒加载图片
- GIF and WebP originals / 保留 GIF、WebP 原图
- Zhihu column videos, Xiaohongshu notes / 知乎专栏视频、小红书图文
- Saves to your own Readeck instance / 保存到你自己的 Readeck 实例

Install / 安装：

1. Download the latest `readeck-web-extension-*.zip` from Releases.
2. Extract it to a fixed directory.
3. Open `edge://extensions` or `chrome://extensions`.
4. Enable Developer mode, then “Load unpacked”.
5. Enter your Readeck URL and API token in the extension settings.

See [docs/browser-extension.md](docs/browser-extension.md) for details.

详细说明见 [docs/browser-extension.md](docs/browser-extension.md)。

---

## Android APK

The Android share target is optional. It adds a “Save to Readeck” entry to the Android share sheet and saves the rendered page.

Android 分享目标为可选功能，会在系统分享菜单增加「转存 Readeck」，保存渲染后的页面。

Release assets:

- `readeck-1.1.0-cn.apk` — patched app with the share target, Chinese UI, version `1.1.0-cn`.
- APK binaries are not committed to the repository; download them from Releases.

> The APK is signed with a project test key. Uninstall the official app before installing the patched build (signatures differ).

---

## Privacy / 隐私

**English** — Bookmarks, images, videos, and article bodies stay in your own Readeck instance. AI requests go only to the model service you configure. This repository contains no instance URL, API token, account, or personal data. Folder passwords are application-level access control, not encryption of the archive zip.

**中文** — 所有书签、图片、视频和正文都保存在你自己的 Readeck 实例；AI 请求只发送到你配置的模型服务。本仓库不包含实例地址、API Token、账号或个人信息。文件夹密码是应用层访问控制，不是对归档 zip 的加密。

---

## Upstream and license / 上游与许可证

This is a fork, not an official Readeck release. Upstream copyright belongs to the Readeck authors; new code in this repository is released under AGPL-3.0.

这是 Readeck 的 fork，不是上游官方版本。上游版权归 Readeck 作者所有；本仓库新增代码同样以 AGPL-3.0 发布。

- Upstream / 上游：<https://codeberg.org/readeck/readeck>
- License / 许可证：[LICENSE](LICENSE)
