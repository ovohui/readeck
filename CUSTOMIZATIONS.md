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

## 22. 2026-11-24 第九轮：手机端分享目标（自建 Android APK）

### 为什么必须要自己做一个

官方客户端（`com.mydeck.app`，1.1.0）的分享入口只把 **URL** 传给服务端：

```
ShareActivity  ->  ACTION_SEND / text/plain  ->  POST /api/bookmarks {url}
```

而小黑盒、抖音分享页这类站点，那个 URL 打开是 3917 字节的空壳，正文由带签名的接口拉取
（见第 19 节）。**分享只带 URL 这件事，决定了服务端再怎么改都拿不到内容**。桌面端能解决是
因为浏览器扩展把渲染后的 DOM 一起提交了——手机上缺的就是这一步。

### 做法：把浏览器扩展那套搬到手机上

新增 `tools/mobile-share/`，一个 28 KB 的独立 APK，包名 `org.ovohui.readeckshare`：

```
分享 -> ShareActivity
         WebView.loadUrl(url)          站点自己的 JS 跑起来（含它的签名请求）
         onPageFinished
           自动滚动到底（260ms 一屏，最多 60 次）   <- 触发懒加载图片
           滚回顶部，等 2 秒
           evaluateJavascript -> JSON.stringify({title, url, html})
         POST /api/bookmarks {url, title, html}   <- 服务端只做 readability
```

关键点只有一个：**提交 html，服务端不再请求原页面**。这与扩展走的是同一个 API 字段，服务端
一行代码都不用改。

### 登录复用设备码，不存密码

`MainActivity` 走和服务端一致的 OAuth 设备码流程：

```
POST /api/oauth/client -> client_id
POST /api/oauth/device -> user_code / verification_uri
（浏览器里授权）
POST /api/oauth/token  -> access_token（存 SharedPreferences）
```

也支持直接粘一个 API 令牌（`/profile/tokens` 里新建），服务端还没升级时用这条路。

### 不用 Gradle

机器上没有 Android Studio，也不想为一个 28 KB 的程序装 SDK Manager。构建脚本
`build.py` 直接用四件套：

```
javac -source 11 -target 11 -classpath android.jar
d8 --min-api 24 --lib android.jar
aapt2 compile --dir res && aapt2 link -I android.jar --manifest AndroidManifest.xml
zipalign -p -f 4 && apksigner sign（v2 + v3）
```

两个 Windows 上的坑：

- `aapt2` 打不开中文目录（本仓库路径含「自定义改造」），所以所有中间文件都放在系统临时目录
  的 ASCII 路径下，产物再拷回来；
- `apksigner.bat` 要求 `JAVA_HOME`，直接 `java -jar build-tools/lib/apksigner.jar` 更稳。

图标是 `make_icon.py` 用 zlib 手写 PNG 画出来的（192×192，圆角、书签丝带），不依赖 Pillow。

### 验证

- `aapt2 dump badging`：`org.ovohui.readeckshare`，minSdk 24，targetSdk 34，
  label「转存 Readeck」，`ShareActivity` 的 `SEND` + `text/plain` 齐全；
- `apksigner verify`：v2、v3 均通过，证书 `CN=Readeck Share`；
- 分享文本的链接提取（小黑盒、小红书、中文标点粘连、多链接）用纯 JVM 断言跑过，7 例全过；
- 端到端：用设备码换到的 Bearer token 直接 `POST /api/bookmarks {url,title,html}`，
  服务端 `202`，几秒后 `/api/bookmarks/<uid>/article` 里就是提交的那段正文（证明服务端
  用的是我们递过去的 HTML，而不是自己去抓 `example.com`）。

## 23. 2026-11-24 第十轮：把抓取功能塞进官方 App（而不是另装一个）

### 为什么要改官方 APK

用户的两句反馈把需求说清楚了：

1. 「原 app 可以直接查看我收藏的内容」——独立的小工具只能存，不能看；
2. 「很多 APP 识别不到这个软件，而且只能复制分享链接，这个 app 不能粘贴链接」。

第 2 条尤其关键：**很多应用只把分享发给它认识的、或者是自家清单里的应用**，一个陌生包名
的分享目标经常根本不出现。而官方客户端的分享入口是系统里早就注册好的，一直在。

所以方向改成「官方 APK + 我们的抓取 Activity」。

### 官方 APK 的结构（先摸清楚再动）

```
package        org.readeck.apps.android（类名还是 com.mydeck.app）
version        1.1.0 (1001000)
minSdk 32 / targetSdk 36 / compileSdk 37
组件           com.mydeck.app.MainActivity           MAIN + LAUNCHER
                                                   VIEW readeck://oauth-callback
               com.mydeck.app.ui.share.ShareActivity SEND + text/plain   ← 只传 URL 的那个
split          base.apk + split_config.arm64_v8a（两个 .so）+ en / zh（语言资源）
```

它本来就有分享目标，但只做 `POST /api/bookmarks {url}`。要改的就是这一处。

### 补丁做了什么

`tools/mobile-share/patch_official.py`：

1. `apktool d -s` 反编译（`-s` 保留原 `classes.dex` / `classes2.dex`，不解 smali）；
2. 清单三点改动：
   - 删掉 `requiredSplitTypes="base__abi"` 和 `splitTypes=""`（合并成单 APK 后不能还要求
     ABI split，否则安装报 `INSTALL_FAILED_MISSING_SPLIT`）；
   - 摘掉官方 `ShareActivity` 的 SEND 过滤（避免分享面板出现两个同名条目）；
   - 加两个 Activity：`org.ovohui.readeckshare.ShareActivity`（SEND + PROCESS_TEXT）、
     `org.ovohui.readeckshare.MainActivity`（LAUNCHER，粘贴框 + 登录 + 设置）；
3. `apktool b` 重建，追加 `classes3.dex`（我们那 5 个类，`--min-api 32`，原生 multidex）；
4. 把 ABI split 里的 `lib/arm64-v8a/*.so` 以 **stored** 塞回 —— 清单里
   `extractNativeLibs="false"`，系统直接从 APK mmap，压缩了会崩；
5. `zipalign -p -f 4` + `apksigner` 签我们的证书。

### 两个 Windows 上的坑

- **aapt2 打不开中文路径**：本仓库叫「Readeck 自定义改造」，apktool 调 aapt2 时直接崩
  （`could not exec ... aapt2 compile --dir D:/.../Readeck ?Զ???`）。所有中间文件必须放在
  系统临时目录的 ASCII 路径下，产物再拷回仓库。
- **apktool 2.11.1 太旧**：这个 App 用 compileSdk 37 构建，2.11 不认识新属性。换
  **apktool 3.0.3** 之后重建干净。

### 验证（静态，能验的都验了）

| 项 | 结果 |
| --- | --- |
| 资源表 | 原包 1392 个 resource id，补丁包 1392 个，**缺失 0 / 新增 0**；归一化 diff 只剩同一 bag 内条目顺序差异（值完全一致） |
| 清单 | `requiredSplitTypes` 已消失；新 Activity 的 SEND / PROCESS_TEXT / LAUNCHER 齐全；官方 `ShareActivity` 已无 SEND 过滤；`@style/Theme.MyDeck` 仍是 `0x7f11035b`（id 没漂） |
| dex | `classes3.dex` magic `dex\n039`，含 `Api`、`Api$ApiException`、`MainActivity`、`Prefs`、`ShareActivity` |
| 原生库 | `lib/arm64-v8a/*.so` 两条，`compress_type=0`（stored），`zipalign -c -p` 全 OK |
| 签名 | `apksigner verify` 通过（v2 + v3），证书 `CN=Readeck Share` |
| 残留 | 无 `original/META-INF` 旧签名残留 |

### 代价，必须提前说

- 签名换了 → **不能覆盖安装**，得先卸载官方 App（书签在服务端，本地只是缓存）；
- 打补丁版沿用官方的 `network_security_config`（`cleartextTrafficPermitted="false"`），
  所以实例与目标页都得是 HTTPS；
- 官方更新后要重新跑一次脚本。

## 24. 2026-11-24 第十一轮：把补丁包翻成中文

### 为什么界面是英文的

官方把翻译做成了 **split**：`split_config.zh.apk` 里只有 `resources.arsc`（33 KB）。
合并成单 APK 时这个 split 被丢掉了，base 里一条中文都没有，所以整机英文。

### 先量一下有多少

把 split 反编译出来数：

```
dec_zh/res/values-zh-rCN/strings.xml   339 条
base   res/values/strings.xml          671 条
缺失                                   332 条（约一半）
复数（plurals）  17 个，其中 8 个没有中文
```

即：**官方自己的中文包只覆盖一半**。只把 split 并进去，界面会变成中英混排。

### 做法

1. `apktool d split_config.zh.apk` → `res/values-zh-rCN|zh-rHK|zh-rTW/`，直接拷进 base 的资源目录。
   字符串名在 base 里已存在，aapt2 会沿用同一批 resource id，只是多一个 config 分支
   （实测合并后 1392 个 id 一个没漂）。
2. 缺的 332 条字符串写在 `tools/mobile-share/locale/values-zh-rCN/strings_extra.xml`，
   8 条复数写在 `plurals_extra.xml`。**独立成文件**是刻意的：资源名在同一个 config 内唯一，
   不会和 split 里那份冲突，补丁脚本只要把 `locale/` 整个拷进 `dec/res/`。
3. 加 `res/xml/locales_config.xml` 与 `<application android:localeConfig="@xml/locales_config">`：
   Android 13+ 的「设置 → 应用 → Readeck → 语言」里就能手动选中文——手机系统语言是英文时，
   这是唯一的入口。
4. 占位符逐个对齐（`%1$s` `%2$d` `%%`），有脚本比对，0 处不一致。

### 验证

```
strings   671 / 671 有 zh-rCN
plurals    17 /  17 有 zh-rCN
resource id 1392 → 1393（只多出 locales_config 这一个新 id，原 id 缺失 0）
zipalign -c -p 4 通过，apksigner v2+v3 通过
```

抽样：

```
string/about_credits_title     () "Credits"   (zh-rCN) "致谢"
string/welcome_message         (zh-rCN) "Readeck for Android 是 Readeck 的 Android 客户端。…"
plurals/detail_words           (zh-rCN) other="%1$d 字"
```

## 25. 2026-11-24 第十二轮：没有 og:image 的站点，书签列表没有头图

### 现象

安卓 App 的列表里，小红书那条有封面，知乎、小黑盒那几条只有一块渐变占位。

### 原因

头图只来自页面声明的社媒卡片：

```go
// pkg/extract/meta/picture.go
href := d.Meta.LookupGet("x.picture_url", "graph.image", "twitter.image",
                         "oembed.thumbnail_url", "ld.image")
if href == "" {
    return next          // ← 这里就结束了
}
```

知乎、小黑盒、Discuz 论坛这类站点一个都没声明，于是 `Pictures["image"]` 不存在，
`b.Files["image"]` 也就没有，客户端只能画占位图。而正文里其实一屏一张图。

### 修复

新增 `pkg/extract/meta/picture_content.go`，处理器 `ExtractContentPicture`，放在
`contents.Text` 之后、`saveBookmark` 之前（此时 `d.Body` 已是 readability 渲染后的正文）：

- 已经有 `image` 就不动，声明过的站点行为完全不变；
- 只处理 `article` / `photo`，视频和图片页跳过；
- 从正文里取**第一张像照片的**图：跳过 `data:`/`blob:`、`.svg`、宽或高 < 200 的属性、
  以及 URL/class/alt 里带 avatar、emoji、icon、logo、spacer、placeholder、gravatar 的；
- 懒加载的 `<img src="data:..." data-src="真地址">` 会回退到 `data-src` / `data-original` /
  `data-actualsrc` / `file`；
- 按 `ImageSizeRegular`（800px）抓下来，同时生成 380px 缩略图，进 zip 的 `img/`，
  和社媒卡片那条路产物一致。

`pkg/extract/meta/picture_content_test.go` 覆盖四种情况：普通首图、跳过头像/表情/logo 后取正文图、
懒加载回退、全是不合格图时返回空。

注意：**已存在的书签不会自动补图**，重新剪藏一次才有。

## 26. 2026-11-24 第十三轮：安卓补丁包界面仍是英文

### 上一版错在哪

上一版把 `split_config.zh.apk` 并进了资源表，实测 `671 条 strings 有 663 条带 zh-rCN`，
dump 出来也确实有中文值。但装在手机上仍是全英文，系统设置里也没有语言入口。

两个原因叠在一起：

1. **资源匹配只认精确语言**。`values-zh-rCN` 只对 `zh-CN` 生效；系统 locale 是 `zh-Hans`、
   `zh-SG` 或某些 ROM 上报的 `zh` 时，匹配不到 rCN，直接回退到默认 `values/`，也就是英文；
2. **per-app language 是 Android 13 才有的入口**，且要求 `localeConfig` 中的语言真的存在资源。
   系统低于 13 时，根本没有「应用语言」这一项——用户看到的“没有中文选项”就是这个。

### 这一版：直接把默认值换成中文

`patch_official.py` 新增 `force_default_chinese()`：合并 split 与自写 overlay 之后，
把中文值**写回 `res/values/strings.xml` 和 `res/values/plurals.xml`**（默认配置）。

```
default strings translated: 655
default plurals translated:  17
```

剩下 16 条不翻是有意的：`Android %1$s (API %2$d)`、`URL：%s`、`Readeck URL`、
`https://readeck.example.com`、`Readeck`、`%1$d/%2$d`、`100 MB`、`25/50/100/200/500`、
`androidx_startup`。这些是数字、地址和品牌名。

另外 `versionName` 改成 `1.1.0-cn`，装完在「关于」里能直接看出是哪个包。

### 验证

```
dump 默认配置（无 locale 限定的那行）
  archive     "归档"      articles  "文章"     favorites "收藏"
  videos      "视频"      pictures  "图片"     labels    "标签"
  settings    "设置"
不再存在任何值为 "Deck" / "Highlights" / "Collections" / "User Guide" / "About" 的默认字符串
resource id 1392 → 1393（只多 locales_config），原有 id 缺失 0
zipalign -c -p 4 通过，apksigner v2+v3 通过，versionName=1.1.0-cn
```

## 27. 2026-11-24 第十四轮：小红书视频存不下来

### 现象

一篇视频笔记（`6af2c3ed000000002b0198ff`），两条路都失败：

| 入库方式 | 结果 |
| --- | --- |
| 浏览器扩展 | 只有一张封面图，正文 351 字节，没有视频 |
| 服务端抓取 | 正文只有一个 `<iframe>`，浏览器里显示「myreadeck.zh.ovohui.com 拒绝连接」 |

服务端那条的归档正文是：

```html
<iframe src="./_resources/JfWxduAY1q13oGToAAGBKE.html" data-forced-video="1">
```

而 `_resources/JfWxduAY1q13oGToAAGBKE.html` 在归档里根本不存在（404）。

### 三个独立的原因

1. **内容脚本没取到流**。日志 `xiaohongshu: injected media images="1" videos="0"`：SSR 里
   `note.video.media.stream.h264[0].masterUrl` 是有的（未登录抓取时能读到），但脚本在某些
   响应下读不到，只拿到封面。
2. **yt-dlp 不认识这个站**。`supportedEmbedHosts` 里没有 `xiaohongshu.com`，于是
   `injectForcedVideo` 把页面 URL 塞进 iframe 之后，`processEmbeddedVideos` 直接 `continue`，
   没有任何人接手。
3. **失败后留下了坏 iframe**。archiver 把 iframe 的 `src` 当作要抓的资源，抓不到就改写成一个
   不存在的本地路径，用户看到的就是「拒绝连接」。

### 修复

**a. yt-dlp 支持小红书**（`pkg/archiver/ytdlp.go`）

```go
"xiaohongshu.com",
"xhslink.com",
```

实测（本机 yt-dlp 2026.11.23，无 cookies）：

```
[XiaoHongShu] 6af2c3ed000000002b0198ff: Downloading webpage
ID EXT RESOLUTION FPS CH | FILESIZE   TBR PROTO | VCODEC
0  mp4 720x1282    30  2 |  8.53MiB 3380k http  | h264
```

`embeddedVideoPageURL` 的默认分支会把完整 URL（含 `xsec_token`）交给 yt-dlp，不带 token 的
私密笔记要的就是这个参数。

**b. 下载失败不再留坏 iframe**

只在 `data-forced-video="1"`（我们自己注入的那种）时生效：把 iframe 换成
「视频：打开原页面」的链接。页面原本就有的播放器不动。

**c. 扩展在登录态拿不到流时，用匿名请求再取一次**

登录后小红书的 SSR 只给封面，播放器再去带签名的接口要流。同一个笔记对**未登录访客**仍然
把流写在 SSR 里，所以插件在 `note.type === "video"` 且流为空时：

```js
fetch(location.href, {credentials: "omit", cache: "no-store"})
  → 从返回的 HTML 里取 noteDetailMap
  → note.video.media.stream.h264[0].masterUrl
  → 插入 <video src>
```

只对视频笔记发这一次请求，图片笔记零开销。

### 验证

```
pkg/archiver: TestEmbeddedVideoPageURL 增加小红书用例（URL 与 query 原样保留）       PASS
pkg/archiver: TestIsSupportedEmbedHost 增加 www.xiaohongshu.com / xhslink.com        PASS
pkg/extract/contentscripts: TestXiaohongshuVideoNote
    用真实 SSR HTML 跑真实脚本 → 归档正文含 <video> 与 .mp4                          PASS
扩展：从 popup.js 抽出 xhsAnonymousVideo + streamURL，mock fetch 返回真实 HTML
    logged-in streamURL: ""（确认登录态拿不到）
    anonymous URL: https://sns-video-v6.xhscdn.com/...258.mp4?sign=...&t=...
    credentials: omit，4 项断言全过                                                   PASS
```

## 28. 2026-11-24 第十五轮：视频其实存下来了，但阅读页是空的

### 日志里的证据

新笔记（`6af6ffaf000000002a00c907`）服务端那条：

```
[INFO] synthesized empty document for video page
[DEBU] save resource name="./_resources/FsfvzVhjcaArhNi3v1VNko.mp4"
[INFO] embedded video archived ... resource="./_resources/FsfvzVhjcaArhNi3v1VNko.mp4"
```

而且这个 mp4 **真的能访问**：`GET /bm/Fd/.../_resources/FsfvzVhjcaArhNi3v1VNko.mp4`
→ `200 video/mp4`，16,999,626 字节。也就是说上一轮的 yt-dlp 修复**已经生效**。

用户看到空白，是因为归档 zip 里**没有 index.html**（`x/index.html` 返回 404），
`/api/bookmarks/<uid>/article` 返回 0 字节。`createZipFile` 里：

```go
if len(ex.HTML) > 0 && ex.Drop().IsHTML() {
    ... ArchiveDocument(...)
    b.Files["article"] = ...
}
```

合成空文档时只写了 `ex.HTML`，没改 `ContentType`。这次 xsec_token 过期导致 302，
最终 ContentType 不再是 `text/html` → `IsHTML()` 为 false → 整段被跳过。
上一轮那条恰好 ContentType 还是 text/html，所以有 103 字节的 iframe 存下来，问题被掩盖了。

### 修复

`internal/bookmarks/tasks/tasks.go`：合成空文档的同时 `ex.Drop().ContentType = "text/html"`。
否则下载好的视频进了一个没有 index.html 的 zip，等于白下。

### 扩展这一侧

插件保存的那条正文是 3 张 640×1387 的竖图，没有 `<video>`，而且标题是 `<h2>`、
没有 `xhs-note` class —— 说明 `xhsArticle()` 返回 null，走的是 `xhsArticleFromDOM()` 兜底，
之前那个匿名取流只加在主路径里，兜底路径没覆盖到。

补上：

- `xhsArticleFromDOM` 改 async；
- 新增 `xhsVideoFromDOM`：从 `video[src]` / `source[src]` 取 http mp4，
  再从内联 script 文本里正则找 `.mp4?sign=...`（先把 `\/` 与 `u0026` 还原）；
- 兜底路径在插入图片后，若还没有 `<video>`，依次尝试 DOM 直链与匿名请求；
- manifest 1.2.1。

验证：`xhsVideoFromDOM` 的 video / source / 内联 script 三种来源都能取到正确的
`https://sns-video-v6.xhscdn.com/...mp4?sign=...`，blob URL 正确忽略。

### 已存在的书签

`FdRJBCHyVeo2MT3Qh3AS6d` 的 mp4 已经在归档里，可以不用重存，直接打开
`/bookmarks/FdRJBCHyVeo2MT3Qh3AS6d/content` → 点 HTML 源码 → 写入：

```html
<video controls preload="metadata" src="./_resources/FsfvzVhjcaArhNi3v1VNko.mp4"></video>
```

保存后就能播。新保存的书签不需要这一步。

## 29. 2026-11-24 第十六轮：视频下载成功，正文却还是空的

### 现象

服务端粘贴链接保存的那条（`85svuqQXQpfvx5LqVVmuMc`）：

```
/api/bookmarks/<uid>/article  →  200, 0 字节
/bm/85/<uid>/index.html       →  404
/bm/85/<uid>/_resources/AjpKMvXdi3xqzB57ytpEzt.mp4  → 200, 64,113,751 字节 video/mp4
```

视频**已经完整下载**（64 MB），但归档 zip 里没有 `index.html`，阅读页只能空白。
插件保存的那条（`2dBnaqXomPxkeiT7UMhx8c`）则只有 7 张 640 宽的缩略图，没有视频。

### 根因一：mp4 占用了页面 URL 的槽位

`downloadWithYtdlp` 把下载结果登记成 `Resource{url: pageURL, Name: ...mp4}`，
collector 的键就是页面 URL。随后 `saveHTML` 开头有一段：

```go
if _, ok := arc.collector.Get(uri); ok {
    return nil          // ← 以为页面已经存过了
}
```

`uri` 就是页面 URL，而这时的条目是那个 mp4，于是 `index.html` 从未被写入。
第 15 轮补的 `ContentType = "text/html"` 只解决了「要不要归档」，
没解决「归档时被自己的视频挡住」。

**修复**：`videoResourceURL()` 给视频一个独立键
（`<pageURL>?__readeck_video=1` 或 `&__readeck_video=1`），页面 URL 的槽位留给文档本身。
另外三处判断都收紧成「只跳过已保存的 HTML」：

- `saveHTML`：`res.Saved() && res.ContentType == "text/html"`
- `ArchiveDocument` / `ArchiveReader`：同上
- `downloadWithYtdlp`：重复归档时直接复用已有视频，不再二次下载几百 MB

### 根因二：扩展读不到页面的 state

`chrome.scripting.executeScript` 不指定 `world` 时默认是 **ISOLATED**，与页面共享 DOM
但不共享 JS 全局变量。`window.__INITIAL_STATE__` 在页面主世界，所以：

- `xhsArticle()` 永远返回 `null`（读不到 note）；
- 一直走 `xhsArticleFromDOM()` 兜底，拿到的是 `#noteContainer` 里 640 宽的缩略图；
- 补流的 `xhsAnonymousVideo()` 现在也失效了——匿名请求被 302 到登录页：

```
GET /explore/6afabf31... (无 Cookie) → 302
Location: https://www.xiaohongshu.com/login?redirectPath=...
```

**修复**：新增主世界探针 `xhsProbe()`，由 popup 先注入：

```js
chrome.scripting.executeScript({target: {tabId}, world: "MAIN", func: xhsProbe})
```

它只搬运需要的东西，不返回整棵 DOM：

1. `__INITIAL_STATE__` 里的 note（type / title / desc / 作者 / 图片 / video.stream）；
2. 页面上 `<video>` 的 `currentSrc`（登录态播放器拿到的是签名 mp4 直链）；
3. `performance.getEntriesByType("resource")` 里 `sns-video*` 的 mp4 请求。

拿到后作为参数传给隔离世界的 `capturePage`，`xhsNote()` 优先用这份副本，
视频选取顺序变成：`state.stream` → 探针 `videoURL` → DOM 扫描 → 匿名请求（保留作最后兜底）。
manifest 升到 1.2.2。

### 验证

```
go test ./pkg/archiver/
  TestArchiveDocumentWritesHTMLWithYtdlpVideo    PASS   （预置 mp4 后仍写入 index.html）
  TestArchiveDocumentSkipsSavedHTML              PASS   （重复归档仍幂等）
jsdom probe-test.js    3 例：有 state+播放器 / 只有 state / 无 state      ALL PASS
jsdom capture-test.js  端到端：探针 → article 含
                       <video src="https://sns-video-v6.xhscdn.com/...mp4?sign=1">   ALL PASS
go vet + go build ./...                                                       通过
```

### 升级后要做的

- 拉新镜像后**重新保存**那条笔记（旧归档里本来就没有 index.html，不会自愈）。
- 扩展重新加载，确认版本 **1.2.2**；再打开笔记页剪藏，正文里应直接出现 `<video>`。
- 若插件那条仍无视频：说明播放器还没起播（`currentSrc` 为空）且资源时间线里没有 mp4。
  先在页面上点一下播放，再剪藏即可。

## 30. 2026-11-24 第十七轮：服务端有视频没正文，插件有正文没视频

### 现象

同一条笔记，两条书签互补：

| | 服务端粘贴链接（`QYZvT1mh7m4H1jkrUBTePj`） | 插件剪藏（`MsmhHenrkGQCav9MATbXDj`） |
|---|---|---|
| 视频 | 有（64 MB 在 zip 里） | 无 |
| 正文 | 只有一个播放器 | 标题行、作者行、正文分段、话题，排版好 |
| 类型 | `video` | `article` |

用户的原话是「插件端的排版比服务端更好看」。所以这里其实是两个 bug 撞在一起。

### 根因一：媒体页的正文被整段丢掉

`pkg/extract/contents/contents.go` 的 Readability 处理器开头有一条：

```go
// Immediate stop on a media where readability is not explicitly set.
if m.Extractor.Drop().IsMedia() && !readabilityForced {
    m.ResetContent()          // m.Dom = nil; d.Body = []byte{}
    return next
}
```

小红书视频笔记的 `og:type` 是 `video.other`，`DocumentType` 就是 `video`，
于是**整页 DOM 连同正文一起被清零**，`ex.HTML` 为空，`createZipFile` 打出
` synthesized empty document for video page`，最后只剩一个 yt-dlp 下载回来的播放器。

日志里的对照很清楚：服务端那条走到 `document type video` 就直接 `step finish`，
没有任何 `go-readability` 记录；插件那条是 `article`，readability 正常跑了四轮。

### 根因二：`<video>` 和 `<iframe>` 本来就会在抽取阶段被删掉

`contents.go` 里还有：

```go
func removeEmbeds(top *html.Node) {
    dom.RemoveNodes(dom.GetAllNodesWithTag(top, "object", "embed", "iframe", "video", "audio"), nil)
}
```

播放器元素过不了抽取关，架构上就是「抽取阶段删干净，归档阶段按页面 URL 重建」。
所以插件前几轮往正文里塞 `<video src="sns-video-...mp4">` 一直是无效功
（上一轮之所以还能看到视频，是因为服务端那条走的是 yt-dlp 重建路径）。

另外 bleach 默认策略会删掉除 `data-readeck-*` 之外的所有 `data-*` 属性，
所以 `data-iframe-params` 这种「缩略图代替播放器」的标记必须先在策略里放行，
否则归档阶段找不到它。

### 修法

1. **让带正文的媒体页保住正文**：内容脚本重建完文章后写入
   `<meta name="readeck:keep-content" content="1">`，Readability 见到这个标记就不再
   `ResetContent()`。没标记的媒体页（B 站等）行为完全不变。
2. **播放器改由「封面图占位」表达**：服务端脚本和插件都把笔记 URL 写进封面图的
   `data-iframe-params`，`poster` 也用同一张封面。归档时 `processEmbeddedVideos`
   把这张图换成 `<video src="./_resources/xxx.mp4" poster="./_resources/yyy.jpg">`，
   yt-dlp 在服务端下载（时间预算几分钟，不是弹窗的一秒）。
   `injectForcedVideo` 增加一条判断：文档里已有占位图就不再塞 iframe，避免下载两次、
   出现两个播放器。
3. **bleach 放行 `data-iframe-params`**，否则标记会在 CleanDom 阶段被清掉。
4. **服务端直接重建文章**（`xiaohongshu.js` 重写）：读 `noteDetailMap` 里的
   note，输出

   ```html
   <article class="xhs-note">
     <p class="xhs-meta">作者：证人筠筠 · 2026-11-16 16:30 · 福建</p>
     <p>还有我的联名靴哦哦哦！</p>
     <p>这期更是精彩吧？</p>
     <p>#鞋控の日常 #通勤穿搭 …</p>
     <figure><img src="封面" width height poster="封面" data-iframe-params="src=笔记URL&w=&h="></figure>
   </article>
   ```

   和插件端一字不差，区别只是多了视频。标题顺带修好：`og:title` 改成 note 标题
   （去掉「 - 小红书」），`og:description` 改成正文，读者页不再显示
   「决戦美鞋… - 小红书」这种带站点后缀的标题。
   时间统一按 UTC+8 渲染，容器时区变了也不会漂。

### 顺带清掉的死代码

插件里的 `xhsAnonymousVideo()`（匿名再请求一次页面取签名流）、`xhsVideoFromDOM()`
（从 DOM/内联脚本里捞 mp4）、以及 live photo 的 `<video>`，全部删除——
它们产出的元素都会被 `removeEmbeds` 删掉。扩展升到 **1.3.0**。

### 验证

```
go test ./pkg/extract/contentscripts/
  TestXiaohongshuNoteLayout      PASS  （正文分段、作者行、话题、标题去后缀）
  TestXiaohongshuVideoNoteMarker PASS  （封面带 data-iframe-params + poster，只有一个 img）
  TestXiaohongshuVideoPipeline   PASS  （真实视频页走完 readability+bleach 后正文仍在）
  TestExtensionNotePipeline      PASS  （插件产出的 HTML 走同一管线，占位图存活）
go test ./pkg/extract/contents/
  TestReadabilityKeepsFlaggedMediaContent PASS （有标记保留 / 无标记仍清零）
go test ./pkg/archiver/
  TestInjectForcedVideoSkipsExistingPlaceholder PASS （占位图存在时不再塞 iframe）
jsdom capture-test.js / probe-test.js   ALL PASS
go build ./... / go vet                通过
```

### 升级后要做的

```bash
cd /vol1/1000/docker/readeck-custom
sudo docker compose pull && sudo docker compose up -d
```

扩展 `chrome://extensions` 点 ↻，版本应显示 **1.3.0**。

然后**重新保存**那条笔记：服务端粘贴链接、插件剪藏各一次。两条应该长得一样——
作者行、正文分段、话题行，下面一个带封面的播放器，zip 的 `_resources/` 里有 mp4。
旧书签不会自愈（旧归档里没有正文），需要重存。

## 30. 2026-11-24 第二十轮：每个镜像都自称 dev，没法确认更新到底有没有生效

### 现象

拉完新镜像，打开界面和以前一模一样；`/api/info` 永远返回：

```json
{"version":{"canonical":"dev","release":"dev","build":""},"features":["oauth"]}
```

这跟镜像是新的还是旧的毫无关系——**所有镜像都这样**。

### 根因

版本号是编译期用 ldflags 写进二进制的，取值来自 Makefile：

```make
ifeq (, $(shell which git && git status))
VERSION ?= dev
```

`tools/Containerfile` 的编译阶段是 Debian slim + 手工装的 Go，**既没有 git
命令，也不存在 `.git` 目录**（`COPY ./` 不带版本库），所以 `which git` 为空，
`VERSION` 恒等于 `dev`，`DATE` 同理为空。

也就是说 `/api/info` 里的 `dev` 是一个常量，不是"开发版"的意思。用它判断
"镜像有没有更新"永远得到同一个答案。

### 修复一：让二进制自己说它是哪一次构建的

`/api/info` 原来只有 `canonical` / `release` / `build`，全部来自 ldflags，没有
ldflags 就全是占位符。新增 `built` 字段，取运行中可执行文件的修改时间：

```go
// configs/configs.go
func BinaryBuildTime() time.Time {
	if buildTime.IsZero() {
		if exe, err := os.Executable(); err == nil {
			if st, err := os.Stat(exe); err == nil {
				return st.ModTime().UTC()
			}
		}
	}
	return buildTime.UTC()
}
```

镜像里 `/bin/readeck` 是编译阶段的产物，`COPY --from=build` 保留它的 mtime，
所以这个时间就是**镜像构建时间**，和 registry 上 config 的 `created` 完全对得上。
不依赖 CI 传参，也不依赖 git。

顺带把 `/api/system` 的 `build_date` 也换成这个值——它原来在无 ldflags 时回退成
进程启动时间，等于每次重启都"刚构建"。

之后：

```bash
curl -s https://myreadeck.zh.ovohui.com:15351/api/info
```

```json
{"version":{"canonical":"dev","release":"dev","build":"","built":"2026-11-24T12:48:40Z"},"features":["oauth"]}
```

`built` 和下面这条命令的输出一比对，就知道本地容器有没有跟上：

```bash
docker image inspect ghcr.nju.edu.cn/ovohui/readeck:custom --format '{{.Created}}'
```

### 修复二：把 commit 也写进版本号（需要 workflow scope，暂未推）

只靠时间只能判断"新不新"，判断不了"是哪一份代码"。要让 `canonical` 变成
`custom-79d848d-42`（短 SHA + run number），需要在两个地方传参。

`tools/Containerfile` 编译阶段已经接受这两个参数（本仓库已改）：

```dockerfile
ARG BUILD_VERSION=dev
ARG BUILD_DATE=

RUN make setup generate build VERSION="${BUILD_VERSION}" DATE="${BUILD_DATE}"
```

命令行变量优先级高于 Makefile 里的 `VERSION := $(shell git describe --tags)`，
`VERSION_FLAGS` 是立即展开的，所以 `-X` 拿到的就是传进来的值。

workflow 还差一段（**本仓库尚未包含**，因为推送用的令牌没有 `workflow` scope，
GitHub 直接拒绝写入 `.github/workflows/`）。照下面改 `docker-image.yml`，
在 `Build and push` 之前插入一个步骤，并给该步骤加 `build-args`：

```yaml
      - name: Compute build stamp
        id: stamp
        run: |
          echo "version=custom-${GITHUB_SHA:0:7}-${GITHUB_RUN_NUMBER}" >> "$GITHUB_OUTPUT"
          echo "date=$(git log -1 --format=%cI)" >> "$GITHUB_OUTPUT"

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          # ... 原有字段保持不变，只增加下面两行 ...
          build-args: |
            BUILD_VERSION=${{ steps.stamp.outputs.version }}
            BUILD_DATE=${{ steps.stamp.outputs.date }}
```

（GitHub Actions 的表达式里**没有 `substr`**，短 SHA 只能在 shell 里切。）

也可以直接在网页上改：打开仓库 → `.github/workflows/docker-image.yml` → 铅笔图标
→ 粘贴上面两段 → Commit。网页提交走浏览器会话，不受令牌 scope 限制。

`run_number` 单调递增，比 SHA 更适合判断新旧：走 API 通道重放提交时 SHA 会变，
内容却可能是同一份。

### 现在怎么确认更新有没有生效

四条判据，从弱到强：

```bash
# 1. 本地镜像的时间与 digest
docker image inspect ghcr.nju.edu.cn/ovohui/readeck:custom \
  --format '{{.Created}} {{index .RepoDigests 0}}'

# 2. 和 registry 上的比对（这是"有没有新版本"的唯一权威来源）
docker buildx imagetools inspect ghcr.nju.edu.cn/ovohui/readeck:custom \
  --format '{{json .Manifest}}' | head -c 400
#   或直接看南大镜像站：
curl -sI -H 'Accept: application/vnd.oci.image.index.v1+json' \
  https://ghcr.nju.edu.cn/v2/ovohui/readeck/manifests/custom | grep -i docker-content-digest

# 3. 容器是不是真的在用这个镜像（pull 了但没重建容器时这里会对不上）
docker compose ps -q | xargs docker inspect \
  --format '{{.Name}} {{.Image}} {{.State.StartedAt}}'
docker image inspect ghcr.nju.edu.cn/ovohui/readeck:custom --format '{{.Id}}'

# 4. 应用自报的构建时间（本地修复之后才有这个字段）
curl -s https://myreadeck.zh.ovohui.com:15351/api/info
```

第 3 条最容易踩坑：`docker compose pull` 拿到新镜像后，如果 `up -d` 认为容器
配置没变，可能不重建，旧进程继续跑旧二进制。判据是 `{{.Image}}` 的 ID 必须等于
镜像 ID，且 `StartedAt` 晚于 pull 的时刻。真不放心就强制重建：

```bash
docker compose up -d --force-recreate
```

### 本次之前那批镜像

`sha256:0ac2247e…`（构建于 2026-11-24T12:48:40Z / 20:48:40 +08）是这次修复之前
最后一个镜像，它对应远端提交 `f551874c`（第 19 轮：小红书正文保留 + 服务端下视频）。
它内部的 `/api/info` 仍是老样子（无 `built`），所以它自己没法自证。
从下一个镜像起才有 `built`。

## 31. 2026-11-24 第二十一轮：知乎 403、书签头图、卡片排版

### 现象

服务端粘贴知乎链接，日志只有一行错误：

```
[ERRO] cannot load resource err="Invalid status code (403)"
```

书签标题空、没有正文、没有封面，卡片显示「未命名」加一块字母渐变。插件
保存的那条有图但标题是 `(99+ 封私信 / 80 条消息) 神舟二十四号…… - 知乎`。

### 一、403 不是 UA 问题，是登录墙

配置里钉着一个 2018 年的 UA：

```json
"user-agent": "Mozilla/5.0 (Windows NT 6.1; ...) Chrome/70.0.3538.110 Safari/537.36"
```

换 UA 再试，仍然是 403，但响应体说明了原因：

```html
<meta id="zh-zse-ck" content="2mIcG+OGMT+a7HXakxY/NO25...">
<script src="https://static.zhihu.com/zse-ck/v4/....js">
```

```json
{"error":{"need_login":true,"code":40353,"message":"请您登录后查看更多专业优质内容。"}}
```

知乎对匿名请求一律返回 `zse-ck` 的 JS 挑战页，三个入口都一样：

| 入口 | 结果 |
|---|---|
| 桌面 UA 抓 answer 页 | 403，zse-ck 挑战 |
| 移动 UA 抓同一页 | 403 |
| `/appview/answer/<id>` | 403 `code 40362` |
| `/api/v4/answers/<id>` | 403 `code 40353`，`need_login: true` |

所以**唯一的解法是给它一份登录态 Cookie**，Readeck 已经有这个能力
（`设置 → 站点 Cookie`，`/cookies` 页面，第 2 轮做的）。抓取时通过
`httpclient.TransportWrapper` 注入到请求里。

本次的三处改动：

1. `zhihu.com.json` 删掉那个老 UA，改走 `internal/httpclient` 的默认值
   （Chrome 144），另外补上 `accept-language` 和 `referer`。
2. 错误信息带上原因。原来日志记录器只留 message、丢掉属性，
   `errors` 里永远是 `cannot load resource`，等于什么都没说：

   ```go
   m.Log().Error("cannot load resource: "+err.Error(), slog.Any("err", err))
   ```

   现在书签详情里直接是
   `cannot load resource: Invalid status code (403): the site refused the request, it usually needs a login cookie (Settings > Site cookies)`。
3. `drop.go` 对 401/403 单独给提示，其它状态码保持原样。

### 二、知乎视频：yt-dlp 认得，但缺 Cookie

`yt-dlp` 有 `Zhihu` 提取器，接受 `/video/<id>`、`/zvideo/<id>` 和
`/question/<qid>/answer/<aid>`。于是：

1. `supportedEmbedHosts` 加 `zhihu.com`，`embeddedVideoPageURL` 里补一个
   `case`，把 `/video/<id>` 规范化成 `https://www.zhihu.com/video/<id>`，
   回答页原样交给 yt-dlp。
2. 插件把页面里的 `<video>` 换成「封面 + 宿主页 URL」的占位图
   （`data-iframe-params="src=<urlencoded>"`）。抽取阶段本来就会删掉所有
   `<video>`/`<iframe>`，留着播放器等于留一个死元素；换成占位图后归档阶段
   会把它换成 `<video src="./_resources/xxx.mp4" poster="...">`。
3. **Cookie 打通**。yt-dlp 是独立进程，看不到 HTTP 客户端的 Cookie 注入，
   知乎连视频也一起拒。新增 `sitecookies.NetscapeFile()`，把「站点 Cookie」
   里的内容写成 Netscape cookies 文件交给 yt-dlp：

   ```go
   cookiesFile := configs.Config.Bookmarks.YtdlpCookiesFile
   if cookiesFile == "" && SiteCookiesFile != nil {
       if f, err := SiteCookiesFile(); err == nil {
           cookiesFile = f
       }
   }
   ```

   `SiteCookiesFile` 这个钩子在 `internal/app/serve.go` 里赋值——直接
   import 会成环（`sitecookies → internal/server → bookmarks`）。
   显式配置的 `ytdlp_cookies_file` 仍然优先。

   换句话说：**在「站点 Cookie」里存一次知乎，页面和视频都能用**，不用再去
   容器里放一份 cookies.txt。

### 三、标题里的通知角标

`document.title` 在登录状态下会带未读计数，站点再拼上自己的名字：

```
(99+ 封私信 / 80 条消息) 神舟二十四号载人飞船瞄准11月25日8时18分发射 - 知乎
```

服务端（`pkg/extract/meta/title.go`）和插件（`cleanTitle`）各实现一份同样
的规则：

- 开头的括号横幅，**要么点名收件箱**（私信/消息/未读/inbox/unread…），
  **要么就是个纯计数**（`3`、`99+`、`1.2k`），才当作噪音删掉；
- 四位以上数字当成年份/编号保留，`(2026) 年度总结` 不变；
- 结尾的站点名按 ` - `、`|`、`_`、`·` 剥，最多剥两层；带空格或标点的不剥，
  `A - B 两种方案的对比` 原样保留。

### 四、书签头图是怎么来的

卡片上那张图是 `thumbnail`（380px 宽），来源按优先级：

1. `og:image` / `twitter:image` / JSON-LD `image` —— 页面自己声明的封面；
2. 没有声明 → 正文第一张「像图片」的 img：跳过 `data:`、`blob:`、
   `.svg`、小于 200px 的，以及 URL/class/alt 里带
   `avatar|icon|logo|emoji|spacer|pixel|...` 的（`picture_content.go`）；
3. 本次新增：声明了但**宽度小于 400px** 的封面当作 logo 处理，同样回退到
   正文首图，且只在新图更大时才换。很多站没有真封面，`og:image` 就指到
   站点 logo，卡片于是整片红（小红书就是这样）；
4. 都没有 → `rndSVG`，按域名哈希生成的渐变块加首字母（截图里的紫色 `Z`
   和绿色 `X`）。

所以「有没有头图」取决于三件事：页面能不能抓到（知乎必须先配 Cookie）、
正文里有没有图、`og:image` 是不是真图。

### 五、卡片排版

只动了三处，都是呼吸感的调整，没碰配色和栅格：

```scss
&--img   { @apply block overflow-hidden bg-gray-100; }   // 浅色/透明图不再融进白底
&--title { @apply px-2.5 pt-2.5 pb-1; a { @apply line-clamp-3 leading-snug; } }
&--meta  { @apply px-2.5 pb-0.5 …; }
```

标题仍是三行截断，行高收紧到 `leading-snug`，让三行标题和封面之间的
距离看起来是刻意留白而不是挤压。

### 六、验证

```
go test ./pkg/extract/meta/ -run TestCleanTitle
  11 例：知乎角标 / 站点后缀 / 括号年份 / 驼峰连字符 / 纯计数徽标      PASS
go test ./pkg/extract/ -run TestExtractorRun                            PASS
go test ./pkg/archiver/ ./pkg/bleach/ ./pkg/extract/contentscripts/     PASS
jsdom title-test.js（插件 cleanTitle 同一批用例）                        ALL PASS
go build ./...                                                          通过
```

既有失败与本次无关，在 HEAD 上同样失败：`TestExtractor_StripHeadingAnchors`、
`TestExtractor_ConvertMathBlocks`、`TestStrftime`、`internal/acls` 的两个
策略测试（期望值还没跟上站点 Cookie 权限）、`TestSqliteDSN`、
`internal/docs` 与 `internal/profile` 的 `TestPermissions`。

### 七、升级后

1. 拉镜像 `docker compose pull && docker compose up -d`。
2. 扩展重载到 **1.3.1**。
3. **先配 Cookie**：浏览器登录知乎 → 复制 `Cookie` 请求头 → `设置 → 站点
   Cookie` 新增一条，域名填 `zhihu.com`，保存。
4. 重新保存那条知乎回答。成功后：标题不带角标、正文有作者行、卡片有图；
   如果该回答带视频，归档里会有 `<video src="./_resources/xxx.mp4">`，
   日志里能看到 `yt-dlp` 那次下载。

没配 Cookie 之前，知乎只会给出上面那句带 `403` 的错误，这是预期行为。

## 32. 2026-11-24 第二十二轮：站酷的 WAF、知乎的 Cookie 顺序、小红书那条进度条

### 现象

1. 站酷作品页（`https://www.zcool.com.cn/work/....html`）服务端和插件都存不下来。
2. 知乎还是 403；插件存知乎回答仍只有图没有视频。
3. 小红书视频上面压着一条原生进度条，`0:03 / 0:57`，挡住画面底部的字幕，
   而且关不掉（就是截图里那个「下载该视频 / ? / ✕」浮层旁边的东西）。

### 一、站酷：不是 UA 问题，是阿里云 WAF 的 JS 挑战

裸抓 www 主机，无论桌面 UA 还是 iPhone UA，拿到的都是一个 23KB 的壳：

```html
<textarea id="renderData" style="display:none">{"l1":"var arg1='5c2849…';", "l2":"GET"}</textarea>
<meta name="aliyun_waf_aa" content="1adb65ab…">
<script>(function xXRQBR(){ … })();</script>
```

页面逻辑是 `setCookie("acw_sc__v2", e); location.reload()`：脚本先把 `arg1`
喂进一段混淆过的函数算出 `acw_sc__v2`，写进 Cookie 再重载。函数被
`vA` 字符串表加 `D5()` 解码器包着，每次部署都换一套常量，`arg1` 的长度
也不是固定的 40（这次是 50，与常见的 acw_sc__v2 实现不兼容）。

**结论：这条路走不通，不要试图在 Go 里复现。** 试过带全套浏览器头、带
`acw_tc` 会话 Cookie、加 `Referer`、换成 XHR 头，全部还是挑战页。

真正可行的是**移动端镜像**：

```bash
# 仍然挑战
curl -A "<iPhone UA>" https://www.zcool.com.cn/work/ZNzQ0Mzk0ODg=.html
# 200，68KB，正文齐全
curl -A "<iPhone UA>" https://m.zcool.com.cn/work/ZNzQ0Mzk0ODg=.html
# 302（m 站不认桌面 UA）
curl -A "<Chrome 桌面 UA>" https://m.zcool.com.cn/work/ZNzQ0Mzk0ODg=.html
```

所以**主机和 UA 必须同时换**。新增 `internal/wafshield`：一个包在
transport 链最外层的 RoundTripper，`Peek` 响应体前 32KB，命中
`aliyun_waf` / `aliyunwaf_` / `id="renderData"` 就把请求重放到移动镜像上；
没命中就把读过的字节原样接回去，一个字节都不改。

**踩到的坑**：第一次实现只改了 UA 和 Referer，重放依旧被挑战。原因是
`httpclient.defaultHeaders` 里带着桌面 Chrome 的指纹：

```
Sec-CH-UA: "Google Chrome";v="144", "Chromium";v="144"
Sec-CH-UA-mobile: ?0
Sec-CH-UA-platform: "Windows"
Sec-Fetch-Site: none
Sec-Fetch-Mode: navigate
```

「iPhone 的 UA + Windows Chrome 的 client hints」是 WAF 眼里的标准机器人。
现在重放请求的 header 是**整体重建**的，不是打补丁：iOS Safari 根本不发
`Sec-CH-*` 和 `Sec-Fetch-*`，那就一个都不发。

镜像表在 `internal/wafshield/wafshield.go` 的 `mirrors`，要加站点只改这里。

### 二、站酷：内容在 Next.js 的 payload 里

m 站是 Next.js，`<body>` 只有一个空容器，作品全部在：

```html
<script id="__NEXT_DATA__" type="application/json">
  {"props":{"pageProps":{"workData":{"title":"…","productImages":[{"url":"…","width":3000,"height":2250}],"productTags":[…],"creatorObj":{"username":"…"},"copyrightStr":"…"}}}}
</script>
```

新增内容脚本 `pkg/extract/contentscripts/assets/scripts/zcool.js`
（`documentLoaded` 阶段跑，早于 readability）把 payload 读出来重建成：

```
作者：1To3_Design_Lab · 原创 · 平面 / 品牌 · 2026-11-24    ← 一行元信息
<figure><img src=… width=3000 height=2250></figure> × 21     ← 画廊，按原顺序
#品牌设计 #包装设计 #LOGO设计 …                              ← 话题行
作品版权由 1To3_Design_Lab 解释, 署名-非商业性使用-禁止演绎
```

`publishTime`/`createTime` 统一按 UTC+8 渲染，服务器在哪个时区都显示站酷的钟点。
配套 `site-config/zcool.com.cn.json` 只做两件事：`body_selectors` 指向
`article.zcool-work`，`title_selectors` 优先 og:title。

线上实测（`go run` 跑完整管线，不是 fixture）：

```
URL: https://m.zcool.com.cn/work/ZNzQ0Mzk0ODg=.html
TITLE: Panda Factory熊猫工厂品牌设计
AUTHOR: [1To3_Design_Lab]
HTML: <article>… 21 个 <figure><img> …</article>     6790 字节
```

### 三、知乎：Cookie 存了，但被 jar 顶掉

`sitecookies/transport.go` 原来是：

```go
if cookie != "" && r.Header.Get("Cookie") == "" {   // ← 就这一行
    r.Header.Set("Cookie", cookie)
}
```

客户端自带 `cookiejar`，同一次抽取里前面任何一个请求收到过 `Set-Cookie`
（知乎首页就会下发 `d_c0`），下一次请求的 `Cookie` 头就非空了，于是**保存的
登录 Cookie 被整条跳过**。现象和「Cookie 根本没配」完全一样，这也是为什么
上一轮加了提示，用户照着配了还是 403。

改成合并：`mergeCookieHeader` 保留 jar 里的、用保存值覆盖同名项（保存的是
用户明确粘贴的会话，优先级更高）。

同时把 403 的报错分成两种，因为下一步动作完全不同：

```
Invalid status code (403): the site refused the request,
  no login cookie is saved for www.zhihu.com yet, add one in Settings > Site cookies
Invalid status code (403): the site refused the request,
  the cookie saved for www.zhihu.com was refused, export a fresh one from a
  logged-in browser session (Settings > Site cookies)
```

判定靠 `extract.SiteCookieLookup` 这个钩子变量（`pkg/extract` 不能直接
import `internal/sitecookies`，会成环），在 `InitServer` 里赋值。

**知乎本身没有绕过的可能**：`www`、`m.zhihu.com`、`/appview/answer/<id>`、
`api.zhihu.com` 四个入口匿名一律 403，页面里带 `zh-zse-ck` 和
`/zse-ck/v4/*.js`。它要的是浏览器里那份 `__zse_ck`（JS 算出、写回 Cookie 的
签名），只能从已登录的浏览器导出。

### 四、插件侧：站酷整页重建 + 知乎视频的两种情况

`tools/web-extension/popup.js`（1.4.0）：

- **站酷**：DOM 里读 `script#__NEXT_DATA__`，按同样的结构重建 `<article>`，
  并把 `og:title` 和 `<title>` 改成作品名（原本是「作品名 - 站酷 (ZCOOL)」）。
  服务端因为 WAF 抓不到，插件这条路是唯一能拿到画廊的。
- **知乎 `<video>`**：
  - `src` 是真实 http(s) 流（`vdn.vzuu.com/xxx.mp4`）→ 原样留下，服务端当
    普通媒体资源归档；
  - `src` 是 `blob:`（MSE）→ 换成封面 `<img data-iframe-params="src=<回答页URL>">`，
    交给服务端 yt-dlp；
  - 封面缺失时退到页面的 `og:image`，否则只留一个链接、什么也下载不到。
- **知乎 `<iframe src="…/video/<id>">`**：同上，用 iframe 自己的 URL。

### 五、小红书：那条进度条是播放器自带的，关不掉

`<video controls>` 的控制条画在元素内部最底下，不是页面上的元素，CSS 挪不动、
藏不掉。竖屏视频底部通常正是烧进画面的字幕，于是被盖住。

做法是**把控制条延后到点了才给**：

- `modal_videos_controller.ts` 连接时扫一遍正文里的 `video`：去掉 `controls`，
  套一层 `.embedded-video`，点一下才 `setAttribute("controls","")` 并 `play()`。
  存下来的 HTML 仍然带 `controls`，所以导出/EPUB 里没有 JS 也能播。
- `video.scss` 加 `.embedded-video`：封面居中、`max-height:80dvh`、
  `.is-playing` 之前用 `::after`(黑圆) + `::before`(白三角) 画一个播放角标。

**第二个坑**：`.embedded-video` 一开始写在 `@layer components` 里，构建出来的
CSS 里根本没有这条规则。Tailwind 只会保留「类名在扫描文件里出现过」的
component 规则，而 `content` 只配了 `src/**/*.js` 和 `../**/*.templ` ——
这条类名是 `.ts` 里的控制器在运行时加上的，扫描不到就被裁掉。
现在这条规则移出了 `@layer`（普通 CSS 不参与裁剪），并在源码里写明了原因。

### 六、验证

```
go test ./internal/wafshield/            3 例（重放 / 原样透传 / 无镜像不动） PASS
go test ./internal/sitecookies/          合并 6 例（含"保存值覆盖 jar 值"）   PASS
go test ./pkg/extract/contentscripts/    TestZcoolWorkLayout                 PASS
go test ./pkg/extract/ -run TestExtractorRun                                PASS
go test ./pkg/archiver/ ./pkg/bleach/ ./pkg/extract/meta/                   PASS
jsdom zcool-test.js  站酷 7 项 / 知乎 5 项                                   ALL PASS
npx tsc --noEmit / npm run build                                            通过
线上：go run 直连站酷，21 张图 + 标题 + 作者全部到位
```

既有失败与本次无关（HEAD 上同样失败）：`TestExtractor_StripHeadingAnchors`、
`TestExtractor_ConvertMathBlocks`、`TestStrftime`、`internal/acls` 的两个
策略测试、`TestSqliteDSN`、`internal/docs` 与 `internal/profile` 的
`TestPermissions`。

### 七、升级后

1. 拉镜像 `docker compose pull && docker compose up -d`。
2. 扩展重载到 **1.4.0**。
3. **站酷**：直接粘贴链接即可，不需要 Cookie。存下来应该是「一行元信息 +
   21 张图 + 话题行 + 版权行」，图片按原顺序。
4. **知乎**：`设置 → 站点 Cookie`，域名 `zhihu.com`，值粘贴浏览器里完整的
   `Cookie` 请求头（要包含 `z_c0` 和 `__zse_ck`）。这次即使客户端 jar 里
   已经有 Cookie，保存的这份也会生效。
   - 报错说「no login cookie is saved」→ 确实没配；
   - 报错说「the cookie … was refused」→ 配了但过期/被风控，重新导出。
5. **小红书**：旧书签不会自愈（控制条是渲染时挂上去的，改造在客户端），
   刷新页面即可看到新的播放角标；点一下才出现控制条。
