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

## 8. 2026-11 第二轮反馈修复

### 复制 Markdown 全文

侧栏新增 **MD（复制图标）** 按钮：点击后前端 `fetch` 同源 `/api/bookmarks/<id>/article.md`，把返回的 Markdown 全文写入剪贴板，按钮文字短暂变成 `✓`。可直接粘进 Obsidian，不需要先下载再打开文件。

- 控制器：`web/src/controllers/markdowncopy_controller.ts`
- 按钮：`internal/bookmarks/routes/x-sidebar.templ`（紧挨原来的 MD 下载按钮）

### 论坛附件图（52pojie / Discuz!）

Discuz! 论坛把真实图片地址放在 `file` / `zoomfile` 属性里，`src` 指向 `static/image/common/none.gif` 占位图。懒加载识别表已加入 `file`、`zoomfile`、`data-file`，这类"图片丢一半"的问题解决。

### 小红书图片与动图

小红书正文由 JS 渲染，抓到的 HTML 只有 "1/15" 计数器，图片 URL 只存在于 `<script>` 里的 `window.__INITIAL_STATE__`。新增内容脚本 `xiaohongshu.js`：

1. 取所有内联 script 文本，定位 `"noteDetailMap"`，做花括号配对取出 JSON（容错 `undefined`）；
2. 递归收集 `xhscdn.com` 域下的图片 URL 与 `masterUrl`（live photo 的 mp4）；
3. 兜底用正则扫 script 文本里的媒体 URL；
4. 去重、过滤头像/logo，把 `<figure><img>` 与 `<figure><video>` 追加进 `#noteContainer`。

这样普通图文能拿到全部原图，live photo 会额外带一个可播放的视频。

### B 站等视频页真正离线

B 站书签被 Readeck 判定为 `video` 类型，提取出来的 HTML 里没有播放器，所以之前只有一行 "Video on 哔哩哔哩" 链接。现在：

- `Archiver.WithForcedVideo(pageURL)`：归档视频类书签时，若 HTML 里没有任何可用播放器 iframe，就在 `<body>` 末尾补一个指向原页面的 iframe；
- yt-dlp 钩子随后把这个 iframe 换成归档内的 `<video src="_resources/xxx.mp4">`。

### yt-dlp 新增两个配置

| 配置项 | 环境变量 | 说明 |
| --- | --- | --- |
| `ytdlp_cookies_file` | `READECK_YTDLP_COOKIES_FILE` | Netscape 格式 cookies.txt，YouTube 的机器人校验、会员视频必需 |
| `ytdlp_extra_args` | `READECK_YTDLP_EXTRA_ARGS` | 追加参数，例如 `--extractor-args youtube:player_client=android` |

YouTube 提示 "Sign in to confirm you're not a bot" 时，把浏览器导出的 cookies.txt 挂进容器并指向它即可：

```yaml
volumes:
  - ./data:/readeck
  - ./cookies.txt:/readeck/cookies.txt:ro
environment:
  - READECK_YTDLP_COOKIES_FILE=/readeck/cookies.txt
```

### 仍需要浏览器扩展的页面（贴吧、站酷等）

有些页面整个正文都由 JavaScript 渲染（新版贴吧 `fr=personalize_page`、站酷作品页），服务端只能拿到空壳，go-readability 的 `textLength` 一直是 0，日志里表现为连续 4 次 `parsed article length is too short` → `could not extract content`。

这不是选择器能解决的问题：HTML 里根本没有正文。正确做法是用 Readeck 的浏览器扩展（设置 → 应用密码 / Browser extension），在已登录的浏览器里点扩展，浏览器会把**渲染后的 DOM** 发给服务器再解析，图片和懒加载内容都在。

## 9. JS 渲染页面：用油猴脚本把「渲染后的 DOM」发给 Readeck

小红书、新版贴吧、站酷这类页面的正文完全由 JavaScript 渲染，服务端抓到的 HTML 是空壳（readability 的 `textLength` 一直是 0）。Readeck 官方浏览器扩展在部分站点（例如小红书）无法注入，这类页面就彻底存不下来。

Readeck 的 API 本身支持直接提交 HTML，不走服务端抓取：

```http
POST /api/bookmarks
Authorization: Bearer <API 令牌>
Content-Type: application/json

{"url": "页面地址", "title": "标题", "html": "<!DOCTYPE html>…完整 HTML…"}
```

仓库里附了一个油猴脚本 `tools/userscript/readeck-saver.user.js`，在页面里跑、抓的是**浏览器渲染完成后的 DOM**：

1. 安装 Tampermonkey → 新建脚本 → 粘贴该文件内容 → 保存；
2. 首次使用时会询问「服务器地址」和「API 令牌」（Readeck：右上角头像 → 设置 → API 令牌 → 新建）；
3. 页面全部加载完后，点油猴菜单 → **📥 保存到 Readeck（图片内嵌，最稳）**，或按 `Ctrl+Alt+Shift+R`；
4. 脚本会把懒加载图片解析成真实地址、把 `blob:`/防盗链图片转成 data URL，然后 POST 到 `/api/bookmarks`。

两个菜单项的区别：

| 菜单 | 行为 | 适用 |
| --- | --- | --- |
| 图片内嵌 | 逐张抓取并转成 `data:` URL，再提交 | 图片有防盗链、`blob:`、需要登录才能看（最稳，体积大） |
| 保留图片地址 | 只写回真实 `src`，由服务器自己下载 | 图片可匿名访问（体积小） |

**已实测**：把站酷那篇作品页用同样方式保存出的单文件 HTML（18MB，图片已内联）通过 `POST /api/bookmarks` 上传，服务端正常提取，归档里 **155 张图片**全部落到 `_resources/`，正文标题、图标、日志都正常。

也可以用命令行上传已经保存好的 HTML 文件：

```bash
curl -X POST "https://<你的域名>:15351/api/bookmarks" \
  -H "Authorization: Bearer <API令牌>" \
  -H "Content-Type: application/json" \
  --data-binary @payload.json
```

其中 `payload.json` 形如 `{"url":"原始地址","title":"标题","html":"<完整HTML>"}`（HTML 需 JSON 转义，文件大时用脚本生成更稳妥）。

### 小红书图片位置修正

之前把图片追加到 `#noteContainer` 末尾，但 readability 选中的正文容器往往是它内部的 `.note-content`，追加在外层等于被丢弃。现在按 `.note-content` → `#detail-desc` → `.note-scroller` → `#noteContainer` → `article` 的顺序选最深可用容器，再注入图片与 live photo 视频。

## 10. 视频类书签为什么之前完全没归档（B 站）

线上日志把原因说得很清楚：B 站页面被判定为 `video` 类型，抽取流程在 `step finish` 就结束了，**`ex.HTML` 是空的**；归档代码的条件是 `len(ex.HTML) > 0 && IsHTML()`，于是 `index.html` 根本没生成——之前的 `WithForcedVideo` 连执行机会都没有，Markdown 里自然只剩 `[Video on 哔哩哔哩](原链接)`。

修复：类型为 `video` 且开启了 `download_embedded_videos` 时，先生成一个最小文档再交给归档器，归档器补上播放器 iframe，yt-dlp 把视频存进 `_resources/`：

```go
if len(ex.HTML) == 0 && ex.Drop().DocumentType == "video" &&
    configs.Config.Bookmarks.DownloadEmbeddedVideos {
    ex.HTML = []byte("<!DOCTYPE html>\n<html><head></head><body></body></html>")
}
```

日志里会多一行 `synthesized empty document for video page`，之后应看到 `embedded video archived`。

## 11. 小红书图片：服务端为何无解（已实测确认）

在实例上重新归档一篇小红书笔记并查看日志：

```
[DEBU] content script ... script="builtin/xiaohongshu.js" function="documentReady"
[DEBU] xiaohongshu: no media in server side HTML
```

脚本确实执行了，但服务端拿到的 HTML 里**一张笔记图都没有**：`div.note-scroller` 里只有空占位 `elementtiming="note-cover"`，正文文字是 SSR 出来的，图片列表由页面用 `xsec_token` 调 XHR 接口后才插入 DOM。这类内容在服务端不可达（接口还要求 JS 生成的 `x-s` 签名）。

因此小红书的正确入口是第 9 节的油猴脚本：它在浏览器里读取渲染后的 DOM，图片已经在页面上了。针对性地又加了一个兜底：若仍取不到图，就退回 `og:image` 封面。

**结论对照表**

| 站点类型 | 方式 |
| --- | --- |
| 普通文章（知乎、公众号、新闻、论坛） | 直接丢链接给 Readeck |
| 小红书、站酷、新版贴吧、滚动才出图的页面 | 浏览器油猴脚本（`Ctrl+Alt+Shift+R`） |
| B 站/YouTube 等视频页 | 丢链接即可，`download_embedded_videos` 会调 yt-dlp 存 mp4 |

## 12. 导出的 Markdown 带上离线视频

`converter/markdown.go` 在 `DocumentType == "video"` 时原来只输出一行 `[Video on 站点](原链接)`。现在会去归档里找 yt-dlp 下载的媒体文件（`.mp4/.webm/.m4v/.mkv/.mov`），找到就额外输出引用：

**单文件 Markdown**（`/api/bookmarks/{id}/article.md`，复制按钮、下载按钮）

```markdown
[Video on 哔哩哔哩](https://www.bilibili.com/video/BV1GXaA6jEoK/)

<video controls preload="metadata" src="https://<你的实例>/bm/5h/5hKe.../_resources/xxx.mp4"></video>
```

Obsidian 的阅读视图会直接渲染 `<video>` 标签，可以在笔记里播放。

**zip 导出**（`article.md.zip`）

```markdown
[Video on 哔哩哔哩](https://www.bilibili.com/video/BV1GXaA6jEoK/)

![[xxx.mp4]]
```

zip 里 mp4 与 `index.md` 同目录，`![[ ]]` 是 Obsidian 的嵌入语法，视频会随笔记一起走。

实现要点：`resourceURL()` 直接用请求的 scheme/host 拼 `/bm/<FilePath>/<name>`，不用 `urls.AbsoluteURLContext`——后者会相对导出请求路径解析，得到 `/api/bookmarks/{id}/bm/...` 这种坏地址。

单测：`internal/bookmarks/converter/markdown_video_test.go` 构造一个带 `_resources/*.mp4` 的书签，分别断言两种导出的输出。
