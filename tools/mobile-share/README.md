# 转存 Readeck（Android 分享目标）

手机里任何 App 的「分享」菜单里多一项 **转存到 Readeck**。选中它，本应用会用内置浏览器把页面
完整渲染一遍（执行站点自己的 JS，包括那些带签名的 API 请求），然后把**渲染后的 DOM** 连同
地址一起 `POST /api/bookmarks` 给你的 Readeck 服务端。

这正是浏览器扩展在桌面上做的事，只是搬到了手机上。所以小黑盒、抖音分享页、各种 SPA 图文站
这类「HTML 是空壳、正文靠脚本拉」的页面，也能存下正文和图片，而不只是一个干链接。

---

## 安装

```
tools/mobile-share/out/readeck-share.apk      ← 28 KB，已签名（v2 + v3）
```

1. 传到手机（微信文件传输、数据线、`adb install` 都行）；
2. 首次安装需要在系统里允许「安装未知来源应用」；
3. 装完桌面出现「转存 Readeck」。

包名 `org.ovohui.readeckshare`，与官方客户端 `com.mydeck.app` 互不影响，可以共存。

## 首次使用：登录

打开应用，两件事：

**服务器地址** —— 填你实例的完整地址，带端口，例如
`https://myreadeck.zh.ovohui.com:15351`。

**登录**，两种方式：

- **设备码登录（推荐）**：点「用设备码登录」→ 应用自动打开浏览器授权页并显示 8 位码 →
  在已登录的浏览器里输码、点「授权」→ 回到应用，它自己轮询拿到令牌并填入。和官方客户端的
  登录方式完全一致。
- **粘贴 API 令牌**：浏览器打开 `https://<你的实例>/profile/tokens`，新建一个令牌，
  勾上读取书签、写入书签的权限，复制后粘进「访问令牌」框，点「保存」。
  服务端还没升级到带 OAuth 修复的镜像时，用这条路。

填完点「测试连接」，看到「连接成功」就成了。

## 抓取选项

| 选项 | 说明 |
| --- | --- |
| 使用桌面版 UA | 默认开。桌面版的页面结构完整、懒加载更少，论坛和图文站效果更好；某些只给手机端完整内容的站点可以关掉试试 |
| 渲染后等待毫秒数 | 默认 2000。页面滚到底之后再多等一会，让图片、评论、异步区块补齐；网络慢就调大 |

## 日常使用

在任意 App 里「分享」→ 选「转存到 Readeck」：

1. 应用打开目标页，底部黑条显示进度（加载 → 滚动 → 等待 → 提取 → 提交）；
2. 成功会提示「已存到 Readeck」并自动关闭；
3. 失败会弹窗，给你三个选择：**重试**、**只存链接**（让服务端自己抓）、**取消**。

滚动是自动的：每 260 ms 往下滚一屏，直到页面底部或 60 次（约 15 秒），目的是把所有懒加载
图片都触发一遍；然后滚回顶部，等设定的毫秒数，再抓 `document.documentElement`。

## 边界

- **要登录才能看的页面**：内置浏览器是干净的会话，没有你平时浏览器的 Cookie。小红书、微博
  这类需要登录态的站点，公开笔记没问题，私密内容不行。
- **强制跳 App 的页面**：有些分享页只给一句「打开 App 查看」。那种页面里本来就没有正文，
  WebView 也变不出来。
- **纯视频页**：本应用抓的是 HTML，不处理视频流；视频离线仍然靠服务端的 yt-dlp。
- **服务端要有可用的 DNS**：图片是服务端下载的，容器 DNS 坏了图片就存不下来（见
  `CUSTOMIZATIONS.md` 第 20 节）。

## 源码与构建

```
tools/mobile-share/
  AndroidManifest.xml
  res/drawable/icon.png        make_icon.py 生成，纯 Python 画图，无外部依赖
  src/org/ovohui/readeckshare/
    MainActivity.java          设置、设备码登录、连接测试
    ShareActivity.java         WebView 渲染、滚动、抓取、提交
    Api.java                   OAuth 设备码 + POST /api/bookmarks
    Prefs.java                 设置存储
  build.py                     不用 Gradle 的构建脚本
  readeck-share.jks            签名证书（口令 readeck123，别名 readeckshare）
  out/readeck-share.apk        产物
```

构建不需要 Android Studio，也不需要 Gradle，只要有 JDK 17 + build-tools + 一个 android.jar：

```bash
python tools/mobile-share/make_icon.py     # 只在改图标时需要
JDK_HOME=/path/to/jdk17 \
BUILD_TOOLS=/path/to/build-tools \
ANDROID_JAR=/path/to/platforms/android-34/android.jar \
python tools/mobile-share/build.py
```

不给环境变量也行，把解压后的 JDK、build-tools、platform 放进
`.workbuddy/tmp/atool/{jdk17,bt,plat}/`，脚本会自己找。

构建过程是 `javac → d8 → aapt2 compile/link → 塞 classes.dex → zipalign → apksigner`，
所有中间文件都在系统临时目录里（ASCII 路径），因为 aapt2 在 Windows 上打不开中文目录。

**不要删 `readeck-share.jks`**：升级安装要求签名一致，换了就得先卸载再装（书签数据在服务端，
卸载不丢东西，但要重新登录一次）。

## 版本

| 版本 | 变更 |
| --- | --- |
| 1.0 | 首个版本：分享目标、WebView 渲染抓取、设备码登录、只存链接回退 |
