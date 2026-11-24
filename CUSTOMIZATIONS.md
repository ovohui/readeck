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

## 13. 2026-11-24 第三轮反馈修复

### 正文可以编辑了

编辑弹窗原来只能改标题、描述、作者等元数据。侧栏新增一个铅笔按钮，打开 `/bookmarks/{id}/content`，里面是归档正文的 HTML，改完保存即生效（阅读视图、Markdown 导出、复制按钮都会用新内容）。

实现：`bookmarks.RewriteArticle()` 把归档 zip 里的 `index.html` 换掉，其余条目（`_resources/*`、`props.json`、`log`）用 `zip.File.OpenRaw()` + `zip.Writer.CreateRaw()` 原样搬运，不重新压缩，因此几 MB 的图片不会因为一次编辑而掉质量或变慢。

注意两点：
- 编辑框里的图片地址是 `./_resources/xxx.jpg`，改了名字图片就断，改文字或增删段落没问题。
- Windows 上不能一边读 zip 一边覆盖它（`Access is denied`），所以先写临时文件、关闭原归档后再 `os.Rename`。

### 小红书正文里的图片（根因找到并修好）

之前一直以为"服务端拿不到图"，其实是**内容脚本跑在了错误的阶段**。

Readeck 的 DOM 处理器有两个时机：

| 时机 | 说明 |
| --- | --- |
| `documentLoaded` | 刚解析完的原始页面，`<head>` 还在 |
| `documentReady` | `ExtractBody` 之后，`<head>` 已被丢弃，只剩正文容器 |

小红书的 `window.__INITIAL_STATE__`（图片、视频地址都在里面）在 `<head>` 的 script 里。脚本挂在 `documentReady` 上，那时 head 已经没了——日志里 `scripts="3"`（body 里的三个内联脚本）就是证据，而原始页面有 20 个 script。

现在脚本改挂 `documentLoaded`，从 head 读取 state，解析 `note.imageList[*].urlDefault`、`infoList[0].url`，以及 live photo 的 `stream.h264[0].masterUrl`（动图），再把结果拼成一段**平铺**的 `<figure><img>` / `<figure><video>` 追加到 `#noteContainer`（也就是站点配置里 `body` 选择器选中的节点），顺手删掉 "1/3" 那个空滑块占位。

因此现在保存小红书**不需要**额外操作：服务端带站点 Cookie 抓页面 → 脚本读 state → 图片本地化。前提是「站点 Cookie」里存在 `xiaohongshu.com` 的 `a1` 与 `web_session`（CookieCloud 同步过来的那份就够）。

### 浏览器扩展重写（中文、任意站点、抓渲染后 DOM）

官方的浏览器扩展在小红书、贴吧这类站点注入不了，界面也是英文的。仓库里现在带一个自己的 MV3 扩展：`tools/web-extension/`。

| 能力 | 说明 |
| --- | --- |
| 中文界面 | 弹窗、提示、错误信息都是中文 |
| 任意站点 | 直接对当前标签页 `executeScript` 抓 DOM，不依赖站点适配，小红书/贴吧/站酷都能用 |
| 等加载完成 | 可勾选"先滚动到底触发懒加载"，并等待图片 load 完（最多 6 秒）再抓 |
| 图片内嵌 | 图片转 `data:` URL；跨域图片由 service worker 代取（它有 host 权限，不受 CORS 限制） |

安装：`chrome://extensions` → 开发者模式 → 加载已解压的扩展程序 → 选 `tools/web-extension`，然后在弹窗里填服务器地址和 API 令牌。

### 粘贴的分享文案自动提取链接

小红书分享出来的是「小红书（25 【标题 - 作者】😆 token 😆 https://…）」，知乎是「问题标题 - 某某的回答 - 知乎\nhttps://…」。以前直接粘进输入框会报 URL 无效。

现在表单的 `url` 字段多了一个 `clean_url` 清洗器（`utils.ExtractURL`）：

1. 找到第一个 `http://` 或 `https://`；
2. 截断到第一个空白、控制字符或中文标点（`），。、；：！？【】「」` 等）；
3. 去掉结尾的英文标点，但括号做配对检查，`…/Foo_(bar)` 这种地址不会被误伤。

前端、API、扩展、油猴脚本走的是同一个 `createForm`，所以全都生效。

单测：`pkg/utils/url_test.go`（8 组用例，含你给的两条真实文案）、`internal/bookmarks/container_test.go`（归档重写后其余条目字节不变）。

## 14. 2026-11-24 第四轮反馈修复

### 动图（GIF）通过扩展保存后不动的真正原因

B 站、部分图床会在浏览器支持 WebP 时把图片地址重写成：

```
https://i0.hdslb.com/bfs/article/028f2…cdb.gif@880w_880h.webp
                                                          ↑ 这里的后缀
```

CDN 对 `.webp` 请求返回的是**单帧静态 WebP**，所以归档里就成了一帧静止图。用无头浏览器抓渲染后 DOM 可以直接看到这个差异：服务端抓的 HTML 是 `.gif@880w_880h`（curl 验证返回 `image/gif`，67 KB 动图），浏览器里的 DOM 是 `.gif@880w_880h.webp`（24 KB 静态图）。

扩展现在在提交前把这个后缀去掉（`restoreOriginal`，只处理 `….gif/.jpg/.png@参数.webp` 这种形态），服务端拿到的就是原始 GIF。同时：

- 抓取前先等 `document.readyState === "complete"`，再滚动；
- service worker 代取图片时带上 `referrer`，CDN 的防盗链校验才通过。

### 小红书图片：注入位置错了，不是取不到

上一版把媒体块追加到 `#noteContainer` 末尾。这篇笔记（胶带种类）的正文容器是 `div.note-scroller`，它是 `#noteContainer` 的**子节点**，而 readability 保留的是它——追加在外层等于把图片扔出正文，最后被整块丢弃。日志里 `injected media images="1"` 但正文 0 张图，就是这个原因。

现在改为插在正文描述节点（`#detail-desc` / `.note-content`）的**后面一个兄弟位置**，并把 state 里的 `width`/`height` 一起写到 `<img>` 上（readability 对无尺寸图片更倾向丢弃）。

本地用两篇真实笔记页面跑了 `documentLoaded → ExtractBody → Readability` 全链路验证：

| 页面 | 修复前 | 修复后 |
| --- | --- | --- |
| 胶带种类（note-scroller 容器） | 0 张图 | 1 张图（原笔记只有 1 张） |
| 新时代三幻神（3 图） | 0 张图 | 3 张图 |

另外扩展里也加了同样的小红书适配：抓取时把 slider 里的图片摊平成 `<figure>` 列表插到描述后面，避免浏览器路径下图片跟着 slider 一起被丢掉。

### 「Content missing」

侧栏按钮和编辑弹窗都在 Turbo Frame 里（`bookmark-actions-{id}`、`bookmark-update-{id}`）。点链接时 Turbo 会做 frame 内导航，响应里没有同名 frame 就显示 `Content missing`。

给这两个入口加 `data-turbo="false"`（正文编辑页的表单也加），走整页导航即可。

## 15. 2026-11-24 第五轮：扩展会混进推荐笔记

### 现象

同一篇笔记，两种入库方式结果不同：

| 入库方式 | 正文图片 | 额外内容 |
| --- | --- | --- |
| 粘贴链接（服务端抓） | 8 张 | 无 |
| 浏览器扩展 | 41 张 | 33 张 640×853 封面、作者头像、点赞/收藏/评论数、`说点什么…` |

### 根因

笔记页 `/explore/<id>` 是**信息流页面**，不是单篇文章。扩展克隆 `documentElement` 之前还会滚动到 30000px 触发懒加载，这一路滚下来把后面几十条推荐笔记全部加载进了 DOM（封面 640×853、3:4），一起被克隆进去；readability 又因为原文正文很短，把容器外的作者栏、互动栏一并保留。

笔记本身的数据只在 `window.__INITIAL_STATE__.note.noteDetailMap[<id>].note` 里：8 张图、尺寸 1080×1920、作者、时间、IP 属地。用服务端 SSR HTML 里的同一份 state 可以核对：

```
notes: ['6b03f08f000000002500a314']
6b03f08f000000002500a314 title= 觅秋 images= 8 user= Fafa
```

### 修复

小红书改走独立抓取路径（`tools/web-extension/popup.js`）：

1. 从小红书页面内嵌的 `__INITIAL_STATE__` 取标题、正文、作者、时间、IP 属地、
   `imageList[*].urlDefault/urlPre/infoList[0].url`（统一 https）、live photo 与视频流；
2. 在页面里拼一个干净的 `<article>`（h1 → 元信息行 → 正文段落 → 每图一个带尺寸的
   `<figure><img>`），**不滚动、不克隆整页**；
3. state 取不到时退回 `#noteContainer` 范围内抓取：排除头像、图标（宽 ≤160 的元素）、
   容器外的推荐封面，按去掉查询串的 URL 去重。

顺带修掉 `background.js` 里一处失效的 `Buffer` 构造（service worker 代取图片时会抛错，
导致跨域图片只能留原地址、由服务端再抓）。

### 验证

- jsdom 载入该笔记的真实 SSR HTML，跑 `capturePage()`：`title=觅秋 imgs=8 figures=8`，
  文本里没有 `说点什么`、`716`、`avatar`、`recommend`。
- 把这份 HTML `POST /api/bookmarks` 到实例，几秒后读 `/bookmarks/<uid>/content`：
  正文 8 个 `<img>`、8 个 `<figure>`，图片全部落进 `_resources/`（服务端无需 Cookie 也能下载），
  文本为「觅秋 / 作者：Fafa · 2026-11-23 15:51 · 江苏 / 正文 6 段 + 话题」。测试书签已删除。

## 16. 正文编辑「Content missing」：部署滞后，不是功能缺失

实例上 `/bookmarks/<uid>/content` 返回 200、正文 textarea 里有完整 8 KB 正文，说明
构建里已经有编辑器；但页面里的「编辑正文」链接**没有** `data-turbo="false"`，即部署的镜像
仍停在 `62783dc6`，而这项修复在 `2fb881ff`（构建 run 19 已成功）。

- 眼下可用：链接用**新标签页**打开（Ctrl/⌘+点击，或直接访问 `/bookmarks/<uid>/content`），
  编辑后保存走普通表单提交。实测 `POST` 返回 303，正文与归档 zip 改写后完全一致（8218 字节、
  44 张图不变）。
- 彻底解决：拉新镜像 `ghcr.nju.edu.cn/ovohui/readeck:custom`（或 `ghcr.io/ovohui/readeck:custom`）
  后 `docker compose up -d`。

## 17. 2026-11-24 第六轮：服务端小红书图片回退（插入位置）

### 现象

同一篇笔记（`6b03f08f000000002500a314`），两种入库方式：

| 入库方式 | 结果 |
| --- | --- |
| 浏览器扩展（1.1.0） | 8 张图，正常 |
| 粘贴链接（服务端抓） | 0 张图，只剩正文文字 |

而日志里明明有 `xiaohongshu: injected media images="8"`。

### 根因

上一版把媒体块插在 `#detail-desc` 的**下一个兄弟位置**，这个策略依赖 readability 保留
`.note-content` 或 `.note-scroller`。这篇笔记正文很短、没有评论区，readability 最终选的
容器就是 `div#detail-desc` 本身 —— 兄弟节点整块被丢弃，图片全没了。

用真实页面做了 A/B 对照（把两种插入结果分别 POST 到实例，再读归档正文）：

| 插入位置 | 归档图片数 | 正文大小 |
| --- | --- | --- |
| `#detail-desc` 的兄弟位置 | 0 | 2431 B |
| `#detail-desc` 的子节点 | 8 | 3875 B |

### 修复

`pkg/extract/contentscripts/assets/scripts/xiaohongshu.js` 改为
`anchor.appendChild(media)`：无论 readability 最终保留的是 `note-scroller` 还是
`detail-desc`，媒体块都在它的子树里。

## 18. 正文可视化编辑（所见即所得）

原来的 `/bookmarks/{uid}/content` 只有一个 28 行的 `<textarea>`，显示的是 HTML 源码。

现在这个页面是**所见即所得**：

- 正文用 `contenteditable` 的 `div.prose` 渲染，与阅读页同一套排版（宽度/字号/行高偏好也生效）。
- 顶部工具条：粗体、斜体、下划线、H2/H3/段落、引用、无序/有序列表、链接、图片、分隔线、
  清除格式、撤销/重做，最右边 `HTML` 按钮切到源码模式（源码模式里改完再切回来会重新渲染）。
- 粘贴统一转成纯文本，避免带入原站的行内样式和字体。
- `Ctrl/⌘+S` 直接保存。
- 提交时由控制器把 `innerHTML` 写回隐藏的 textarea，服务端仍是原来的 `content` 字段。

图片在编辑器里也能看到：归档里存的是 `./_resources/xxx.jpg`，编辑页渲染前换成
`/bm/<FilePath>/_resources/xxx.jpg`（`Bookmark.ServedArticle`），保存时再换回去
（`Bookmark.StoredArticle`，同时兼容粘进来的绝对地址）。不做这层转换，编辑器里所有图片都是坏图。

入口也补上了：阅读页左侧图标栏（返回、铅笔、Aa、书、全屏）里新增一个「编辑正文」按钮，
点它直接进编辑页，不用再手动在地址栏加 `/content`。

新增文件：`web/src/controllers/contenteditor_controller.ts`、`web/ui/styles/editor.scss`。

## 19. 手机端分享链接：跟随客户端跳转

很多 App 的分享链接不是 301/302 跳转，而是一个「中转页」：只有
`<meta http-equiv="refresh" content="0;url=...">` 或 `location.replace("...")`。
Go 的 http.Client 只跟 HTTP 重定向，遇到这种页面会直接把中转页归档，正文自然是空的。

`pkg/extract/drop.go` 现在会在满足全部条件时跟一次：

- 页面 ≤ 8 KB；
- 正文文字 < 400 字符（真正的文章不会被一个 `location.href=` 带走）；
- 目标与当前 URL 不同。

最多跟 2 跳，跳转后重新按新 URL 抓取，`Domain` 也重算，站点配置和 Cookie 才匹配。

纯 SPA（小黑盒 `/app/bbs/link/<id>`、抖音分享页等）服务端仍然拿不到正文：这些页面的 HTML
是空壳（`textLength=0`），正文来自**带签名的接口**。以小黑盒为例，
`/bbs/app/link/tree` 必须带 `hkey`/`nonce`/`_time`/`_rnd` 四个参数，缺一个就
`缺少必要参数[_time]`，参数齐了但签名不对则是 `非法请求`。这类站点用浏览器扩展抓即可。

## 20. 2026-11-24 第七轮：容器 DNS、Discuz 论坛图片、空白书签

### 日志里的真凶是 DNS，不是抽取

两个站点的失败日志完全一样：

```
dial tcp: lookup sehuatang.net on 127.0.0.11:53: server misbehaving
dial tcp: lookup tu.djhdhs.us on 127.0.0.11:53: server misbehaving
```

`127.0.0.11` 是 Docker 的内嵌 DNS，它把查询转发给宿主。宿主（路由器或运营商 DNS）
对这类域名直接拒绝，容器里就一个域名也解析不了——页面打不开、图片也下不来。

`docker-compose.yml` 里固定公共 DNS 即可：

```yaml
    dns:
      - 223.5.5.5
      - 119.29.29.29
      - 1.1.1.1
```

图片本身不需要额外配置：`tu.djhdhs.us` 只校验 Referer，而归档器抓图片时会自动带上
`Referer: https://<站点>/`（实测：带 Referer 返回 200 / 1.3 MB，不带返回 403）。

### Discuz 论坛：图片被 readability 丢掉

色花堂这类 Discuz 站，帖子正文在 `td.t_f`（`id="postmessage_<pid>"`）里，外面套着
布局表格。readability 这次的容器选到了外层 `table`，`cleanConditionally` 把表格里的
图片当成版面垃圾清掉了：

```
determined the content container ... node="<table>" ... textLength="245"
```

归档后正文 1.3 KB、0 张图（原帖 20+ 张）。

新增 `pkg/extract/contentscripts/assets/site-config/sehuatang.net.json`：正文直接定位到
`//*[starts-with(@id,'postmessage_')]`，并把签名、评分、广告、`attach_nopermission` 之类
剥掉；`strip_image_src` 里排掉 `none.gif` / `emp.gif` 这类占位图和表情。

### 空白书签给个说法

SPA 页面（小黑盒、抖音分享页）抓下来是空壳，readability 反复 4 遍都得到 `textLength=0`，
最后书签就是空白的，什么提示都没有。现在这种情况会写进书签的 errors：

```
No content could be extracted: this page is rendered by JavaScript.
Save it with the Readeck browser extension.
```

侧栏的错误区块本来就有「用浏览器扩展」的入口，现在有具体原因了。

## 21. 2026-11-24 第八轮：安卓 App 登不上，是服务端拒了客户端注册

### 现象

Readeck 官方的安卓客户端（`Readeck_1.1.0.apk`，包名 `com.mydeck.app`，源码
`github.com/jensomato/ReadeckApp`）填好地址、账号、密码，登录直接失败，只留下一句
`Login error (code=400)`。

### 客户端用的是 OAuth 设备码，不是账号密码

从 dex 里能读出完整的登录链路：

```
GET  /.well-known/oauth-authorization-server     ← 读服务端能力
POST /api/oauth/client                            ← 动态注册一个客户端
POST /api/oauth/device                            ← 拿 user_code 和 verification_uri
     （用户在浏览器里输码、点授权）
POST /api/oauth/token  grant_type=urn:ietf:params:oauth:grant-type:device_code
```

也就是说**它根本不提交用户名密码**：App 拿到的是一枚 Bearer token，用户在浏览器里
（已经登录的会话里）替这个设备点一次「授权」。这条路要求服务端接受动态客户端注册，
而注册的第一步就把请求打回来了。

### 根因：client_uri 的 DNS 校验，把「解析失败」当成「URL 非法」

`internal/auth/oauth2/forms.go` 里的 `isValidClientURI` 会 `net.LookupIP(client_uri 的
域名)`，解析失败就判定 URL 非法。这台机器的容器 DNS 是坏的（第 20 节：Docker 内嵌
`127.0.0.11` 转发的上游 resolver 直接拒绝），于是**任何带域名的 client_uri 都注册不上**。
在实例上直接复现：

| client_uri | 注册结果 |
| --- | --- |
| `https://1.1.1.1`（字面量 IP，不用查 DNS） | `201 Created`，拿到 client_id |
| `https://readeck.org` | `400 invalid_client_metadata：invalid client URI` |
| `https://github.com` | 同上 |
| `https://example.com` | 同上 |

App 报的 `code=400` 就是这个 400。

### 修复

`isValidClientURI` 改成**尽力而为**：字面量 IP 直接判公私网；`localhost` / `.local` /
`.internal` / `.invalid` / `.home.arpa` 这类本地名一律拒绝；其余名字做一次 2 秒超时的
解析，**解析失败就放行**，只有解析成功且落在私有/回环地址时才拒绝。

理由：解析失败说明这台机器的 DNS 有问题，不说明客户端身份有问题。用一个 DNS 故障去否决
客户端注册，结果是「服务在线、密码正确、就是登不上」，排查成本极高。

### 修复后服务端全链路验证（不依赖 App）

用 curl 完整跑了一遍设备码流程：

```
POST /api/oauth/client          201  client_id=urn:uuid:...
POST /api/oauth/device          200  user_code=HFWVRVLQ  verification_uri=/device
POST /api/oauth/token           400  {"error":"authorization_pending"}     ← 未授权时
POST /device  (会话 Cookie)      303  授权
POST /api/oauth/token           201  access_token=..., scope=bookmarks:read bookmarks:write profile:read
```

再用这枚 token 打 App 登录后会调用的接口，全部 200：

```
/api/profile                                  200
/api/bookmarks?limit=1                        200
/api/bookmarks/sync?since=1970-01-01T00:00:00Z  200
/api/bookmarks/<uid>/article                  200
```

所以 App 不需要改一行代码，也不该去改：它签的是官方签名，改了就装不上、也收不了更新。

### 顺带说明

App 的 `ShareActivity` 注册了 `ACTION_SEND` + `text/plain`，所以手机里任何 App 的
「分享 → Readeck」都能用，这也正是解决小黑盒这类 SPA 分享链接的正路（服务端抓不到，
客户端带着已经渲染好的页面来存）。
