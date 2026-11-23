# Readeck 自定义改造说明

本仓库在官方 Readeck（Codeberg: `codeberg.org/readeck/readeck`）基础上增加了以下功能，用于满足小红书/知乎等需要登录的站点剪藏、视频下载、自定义存储与 Obsidian 工作流需求。

## 1. 站点 Cookie 管理（修复小红书/知乎剪藏失败）

**问题根因**：小红书等站点在未登录时会将请求 302 重定向到 `/login`，页面正文为空，go-readability 报 `could not extract content`。

**解决方案**：抓取时按域名自动注入已保存的 Cookie 与自定义 Header。

### 使用方式

1. 打开 `https://<你的域名>:15351/cookies`（或在设置侧栏点 "Site Cookies"）。
2. 在 "Add a cookie" 表单中填写：
   - **Domain**：如 `xiaohongshu.com`（自动匹配其子域名）。
   - **Name**：备注名，可留空。
   - **Cookie value**：从浏览器开发者工具 / 抓包工具复制的完整 Cookie，例如 `web_session=xxx; a1=yyy; webId=zzz`。
3. 保存后重新剪藏小红书文章即可。

Cookie 以 0600 权限保存在 `<data_directory>/site-cookies.json`，不会写入数据库，升级时不会丢失。

### 从浏览器获取 Cookie

1. 在浏览器登录小红书。
2. F12 打开开发者工具 → Network 面板 → 刷新页面。
3. 点击任意 `xiaohongshu.com` 请求 → Headers → 复制 `Cookie` 请求头的完整值。
4. 粘贴到上面的 "Cookie value" 输入框。

### CookieCloud 同步

在页面底部 CookieCloud 区域填写服务器地址、UUID、端到端加密密码，点 "Save settings" 后点 "Sync now"。同步会按域名导入 CookieCloud 中的全部 Cookie，并替换之前从 CookieCloud 导入的条目（手动添加的 Cookie 不受影响）。

环境变量方式（可选）：`READECK_STORAGE_PATH`、`READECK_DOWNLOAD_VIDEOS`、`READECK_VIDEO_QUALITY`。

## 2. 视频离线下载

- 配置 `[bookmarks] download_videos = true`（或环境变量 `READECK_DOWNLOAD_VIDEOS=true`），归档 HTML 时会同时下载 `<video>`/`<source>` 中的 mp4/webm 等媒体文件到书签 zip 的 `_resources/` 目录。
- `video_quality`（`READECK_VIDEO_QUALITY`）可选值：`best`（默认）、`1080p`、`720p`、`480p`、`worst`。抓取时会解析 HLS(m3u8) master playlist 并把视频源替换为所选清晰度的地址。

### 播放器类视频（B 站 / YouTube / Vimeo …）用 yt-dlp 真正离线

自定义镜像内已内置 `yt-dlp` + `ffmpeg`（`/usr/local/bin/yt-dlp`、`/usr/bin/ffmpeg`），归档时可以把 iframe 播放器里的视频下载成本地 mp4，并把 `<iframe>` 直接替换成 `<video controls src="_resources/xxx.mp4">`，断网也能播放。

| 配置项 | 环境变量 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `download_embedded_videos` | `READECK_DOWNLOAD_EMBEDDED_VIDEOS` | `false` | 开启播放器视频下载 |
| `ytdlp_path` | `READECK_YTDLP_PATH` | `yt-dlp` | 可执行文件路径 |
| `ytdlp_max_height` | `READECK_YTDLP_MAX_HEIGHT` | `720` | 最大高度，`0` 表示取最佳画质 |
| `ytdlp_timeout` | `READECK_YTDLP_TIMEOUT` | `600` | 单个视频下载超时（秒） |

```toml
[bookmarks]
download_videos = true
download_embedded_videos = true
ytdlp_path = "/usr/local/bin/yt-dlp"
ytdlp_max_height = 720
ytdlp_timeout = 600
```

行为说明：

- 支持 `youtube.com` / `youtu.be` / `youtube-nocookie.com`、`bilibili.com` / `b23.tv`、`vimeo.com`、`dailymotion`、`youku`、`weibo`、`ixigua`、`douyin`、`tiktok`、`twitch`、`ted`、`nicovideo`、`rutube`、`vk` 等 yt-dlp 能解析的站点。
- 每篇文章最多下载 **3** 个播放器视频（`maxEmbeddedVideosPerPage`），避免一篇文章里有几十个视频时把归档队列堵死。视频按顺序下载，不并发。
- 下载失败（地区限制、需要登录、超时）不会影响整篇文章归档：iframe 原样保留，日志里有一条 `yt-dlp download failed` 的告警。
- 需要登录才能看的视频，可把浏览器导出的 Netscape 格式 cookies.txt 放进容器，然后在 yt-dlp 参数里加 `--cookies`（当前实现预留了 `YtdlpConfig.CookiesFile` 字段）。
- YouTube 常见的 “Sign in to confirm you're not a bot” 属于 yt-dlp 自身的风控，重建镜像拿到最新 yt-dlp 或提供 cookies 即可，与 Readeck 无关。
- 运行时基于 Alpine 3.22：`apk add ffmpeg python3`，yt-dlp 用官方 zipapp（2.9MB，跑在系统 python3 上）。整体压缩体积 **95MB**，对比 Debian 版的 247MB（其中 `apt install ffmpeg python3 python3-venv` 单层就有 179MB，NAS 上要拉十几分钟）减少 62%。
- yt-dlp 每次构建都取最新 release（当前 2026.10.26）。需要锁定版本时给 Containerfile 传 `--build-arg YTDLP_VERSION=2026.10.26`。
- ffmpeg 是 Alpine 仓库版本（6.1.2），负责 DASH 流的音视频合并与转封装，够用。

> 当媒体文件因跨域鉴权或体积过大无法下载时，节点不再被删除，而是保留原始远端 URL，因此在线查看归档页时播放器仍然可用。

## 3. 自定义资源存储路径

在 `config.toml` 中配置：

```toml
[bookmarks]
storage_path = "/mnt/readeck-assets/bookmarks"
```

或使用环境变量 `READECK_STORAGE_PATH`。书签 zip 与离线资源会保存到该目录（相对路径基于 `data_directory`）。在 Docker 中可配合新增 volume 挂载：

```bash
-v readeck-assets:/mnt/readeck-assets
```

## 4. 一键转 Markdown（Obsidian）

阅读页侧栏 "Export and Share" 菜单新增两项：

- **Download Markdown**：下载单篇 `.md` 文件，含 YAML front matter，直接拖入 Obsidian vault 即可。
- **Download Markdown (with images)**：下载 zip，包含 `.md` 与离线图片资源，适合完全离线使用。

底层复用 Readeck 自带的 `converter/markdown.go`，输出兼容 Obsidian 的 `==高亮==` 与脚注。

## 配置参考

```toml
[main]
data_directory = "data"

[bookmarks]
# 自定义书签/资源存储目录（可选）
storage_path = ""
# 是否下载文章中的视频文件
download_videos = false
# HLS 清晰度: best / 1080p / 720p / 480p / worst
video_quality = "best"
```

## 重新构建与部署

本项目使用 Go 1.27.4 + Node 22。构建命令：

```bash
make generate   # templ generate + 前端资源 + 文档
make build      # go build
```

或在容器中基于 `tools/Containerfile` 重新构建镜像。

---

功能实现文件索引：

- `internal/sitecookies/` — Cookie 存储、CookieCloud 同步、注入传输层、API 与设置页
- `web/src/controllers/sitecookies_controller.ts` — 设置页交互
- `internal/httpclient/client.go` — 可插拔的传输层包装钩子
- `internal/bookmarks/bookmarks.go` — 可配置存储路径
- `internal/bookmarks/archive.go` — 视频资源下载开关
- `pkg/extract/contentscripts/assets/scripts/video-quality.js` — HLS 清晰度选择
- `pkg/archiver/ytdlp.go` — 播放器视频（B 站/YouTube）用 yt-dlp 下载并替换为本地 `<video>`
- `tools/Containerfile` — 运行层改用 Debian，内置 `ffmpeg` 与 `yt-dlp`
- `pkg/extract/contentscripts/assets/site-config/xiaohongshu.com.json` — 小红书正文提取规则
- `internal/bookmarks/routes/x-sidebar.templ` — Markdown 下载按钮

## 6. 已知问题修复记录

### CookieCloud 同步报 `invalid padding`

CookieCloud 服务端有两个版本的加密格式，之前只实现了旧版，遇到新版就解不开：

| 版本 | 载荷格式 | 算法 |
| --- | --- | --- |
| 旧版（CryptoJS `AES.encrypt`） | base64(`"Salted__"` + 8 字节 salt + 密文) | EVP_BytesToKey(MD5) → AES-256-CBC |
| 新版（CookieCloud ≥ 0.3.0） | base64(裸密文) | key = `md5(uuid-password)[:16]`，零 IV → AES-128-CBC |

现在按 `Salted__` 标记自动识别两种格式，并同时解析 `cookie_data`（新版）与 `cookies`（旧版）两种 payload，Cookie 的 domain 优先取条目自身的 domain。

### 小红书正文能保存但图片全是空白

三个原因叠加，均已修复：

1. **Referer 不匹配**：原先用 `scheme://host/` 作为 Referer，`xhscdn.com` 会校验具体文章 URL，直接 403。现改为完整页面 URL。
2. **懒加载属性未被识别**：小红书图片走 `data-src`，且 URL 无扩展名（`...!nd_dft_wlteh_webp_3`），旧逻辑靠扩展名判断导致识别失败。现改为按属性名识别（`data-src`/`data-original`/`data-lazy-src` 等），并支持无扩展名 URL 与 `//host/path` 协议相对地址。
3. **下载失败就删节点**：旧逻辑把下载失败的 `<img>` 从正文里删除，页面看起来"没有图"。现在保留原始 URL。

另外 `internal/sitecookies` 增加了 CDN 域名回映（`xhscdn.com`/`xhsimg.com` → `xiaohongshu.com`，`zhimg.com` → `zhihu.com`，`hdslb.com` → `bilibili.com`），站点 Cookie 与自定义 Header 会自动应用到 CDN 图片请求上；站点 Cookie 中配置的 `Referer` 也可以覆盖归档器默认值。

### 知乎带视频的回答没有播放器

归档器在未开启 `EnableEmbeds` 时会删除所有 `<iframe>`，于是视频区直接消失。开启 `download_videos` 后现在会同时开启 `EnableEmbeds`，iframe 播放器保留。

### 找到 Markdown 下载入口

分享下拉菜单里本来就有 `article.md`，但不易发现。现在书签侧栏工具栏新增独立的 **MD** 按钮，直接下载 `article.md`，方便拖进 Obsidian。

## 7. 更新已部署的容器

仓库镜像为 `ghcr.io/ovohui/readeck:custom`（国内可用南大镜像 `ghcr.nju.edu.cn/ovohui/readeck:custom`）。每次 `main` 分支推送会自动构建 amd64/arm64 双架构镜像，标签 `custom` 会被覆盖。

```bash
cd /vol1/1000/docker/readeck-12233   # 你放 docker-compose.yml 的目录
docker compose pull
docker compose up -d
```

数据目录 `./data` 保持不变，Cookie、书签、设置都保留。
