// SPDX-License-Identifier: AGPL-3.0-only
package org.ovohui.readeckshare;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Intent;
import android.graphics.Bitmap;
import android.graphics.Color;
import android.net.http.SslError;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.CookieManager;
import android.webkit.SslErrorHandler;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.TextView;
import android.widget.Toast;

import org.json.JSONObject;

import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Receives a shared link, renders the page in a WebView and hands the resulting
 * DOM to Readeck with the html field.
 *
 * <p>Rendering is the point. A share sheet only carries a URL, and for a lot of
 * mobile sites (xiaoheihe, douyin share pages, most single page apps) that URL
 * serves an empty shell whose body is fetched by signed API calls. The desktop
 * browser extension gets around it by scraping the live DOM; this does the same
 * thing on the phone.
 */
public class ShareActivity extends Activity {
    private static final String UA_DESKTOP =
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
                    + "(KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36";
    private static final String UA_MOBILE =
            "Mozilla/5.0 (Linux; Android 14; Pixel 7) AppleWebKit/537.36 "
                    + "(KHTML, like Gecko) Chrome/138.0.0.0 Mobile Safari/537.36";

    // Sharing text is not a bare URL: apps wrap it in their own pitch ("快来看看…
    // 复制打开"), and the link is often glued to Chinese punctuation or to the
    // next sentence. Stop at whitespace, brackets, quotes and CJK characters.
    private static final Pattern URL_RE =
            Pattern.compile("https?://[^\\s<>\"'（）【】《》「」\\u4e00-\\u9fa5，。、；：！？]+");
    private static final int MAX_SCROLL_TICKS = 60;

    private static final String JS_SCROLL =
            "(function(){var h=Math.max(document.body?document.body.scrollHeight:0,"
                    + "document.documentElement.scrollHeight);"
                    + "window.scrollBy(0,Math.max(600,Math.floor(window.innerHeight*0.9)));"
                    + "return (window.scrollY+window.innerHeight)>=(h-30);})()";

    private static final String JS_EXTRACT =
            "(function(){var d=document.documentElement.cloneNode(true);"
                    + "var ss=d.querySelectorAll('script');"
                    + "for(var i=0;i<ss.length;i++){ss[i].parentNode.removeChild(ss[i]);}"
                    + "var s='<!DOCTYPE html>'+d.outerHTML;"
                    + "var bytes=new TextEncoder().encode(s);"
                    + "var bin='';var CH=0x8000;"
                    + "for(var j=0;j<bytes.length;j+=CH){"
                    + "bin+=String.fromCharCode.apply(null,bytes.subarray(j,j+CH));}"
                    + "window.__rdB64=btoa(bin);window.__rdPos=0;"
                    + "return window.__rdB64.length;})()";

    private static final String JS_CHUNK =
            "(function(){var s=window.__rdB64.substr(window.__rdPos,200000);"
                    + "window.__rdPos+=s.length;return s;})()";

    private static final String JS_META =
            "JSON.stringify({title:document.title||'',url:location.href})";

    private final Handler ui = new Handler(Looper.getMainLooper());

    private WebView web;
    private TextView statusView;
    private ProgressBar spinner;

    private String targetUrl;
    private String finalUrl;
    private int scrollTicks;
    private boolean capturing;
    private boolean submitting;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        targetUrl = extractUrl(getIntent());
        if (targetUrl == null) {
            Toast.makeText(this, "没有找到链接", Toast.LENGTH_LONG).show();
            finish();
            return;
        }
        setContentView(buildUi());

        if (Prefs.token(this).isEmpty()) {
            AlertDialog.Builder b = new AlertDialog.Builder(this);
            b.setTitle("还没登录");
            b.setMessage("先在「转存 Readeck」里登录，再回来分享。");
            b.setPositiveButton("去设置", (d, w) -> {
                startActivity(new Intent(this, MainActivity.class));
                finish();
            });
            b.setNegativeButton("取消", (d, w) -> finish());
            b.show();
            return;
        }

        startLoad();
        // Safety net: a page that never fires onPageFinished must not hang here.
        ui.postDelayed(() -> {
            if (!capturing && !submitting) {
                setStatus("页面加载超时，尝试直接提取…");
                beginCapture();
            }
        }, 45000);
    }

    @Override
    protected void onDestroy() {
        if (web != null) {
            web.stopLoading();
            web.destroy();
            web = null;
        }
        super.onDestroy();
    }

    private int dp(int v) {
        return Math.round(getResources().getDisplayMetrics().density * v);
    }

    private View buildUi() {
        FrameLayout root = new FrameLayout(this);

        web = new WebView(this);
        FrameLayout.LayoutParams wp = new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT);
        root.addView(web, wp);

        LinearLayout bar = new LinearLayout(this);
        bar.setOrientation(LinearLayout.VERTICAL);
        bar.setBackgroundColor(Color.argb(230, 24, 24, 27));
        bar.setPadding(dp(16), dp(10), dp(16), dp(12));

        statusView = new TextView(this);
        statusView.setTextColor(Color.WHITE);
        statusView.setTextSize(13);
        statusView.setText("正在加载 " + targetUrl);
        bar.addView(statusView);

        spinner = new ProgressBar(this);
        spinner.setIndeterminate(true);
        LinearLayout.LayoutParams sp = new LinearLayout.LayoutParams(dp(22), dp(22));
        sp.topMargin = dp(8);
        bar.addView(spinner, sp);

        FrameLayout.LayoutParams bp = new FrameLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        bp.gravity = Gravity.BOTTOM;
        root.addView(bar, bp);

        return root;
    }

    private void setStatus(final String text) {
        ui.post(() -> {
            if (statusView != null) {
                statusView.setText(text);
            }
        });
    }

    private void configureWebView() {
        WebSettings s = web.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        s.setDatabaseEnabled(true);
        s.setLoadsImagesAutomatically(true);
        s.setBlockNetworkImage(false);
        s.setUseWideViewPort(true);
        s.setLoadWithOverviewMode(true);
        s.setSupportZoom(false);
        s.setUserAgentString(Prefs.desktopUA(this) ? UA_DESKTOP : UA_MOBILE);
        s.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        s.setMediaPlaybackRequiresUserGesture(true);

        CookieManager cm = CookieManager.getInstance();
        cm.setAcceptCookie(true);
        cm.setAcceptThirdPartyCookies(web, true);

        web.setWebChromeClient(new WebChromeClient());
        web.setWebViewClient(new WebViewClient() {
            @Override
            public void onPageStarted(WebView view, String url, Bitmap favicon) {
                finalUrl = url;
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                finalUrl = url;
                if (!capturing && !submitting) {
                    beginCapture();
                }
            }

            @Override
            public void onReceivedSslError(WebView view, final SslErrorHandler handler,
                                           SslError error) {
                // Only reachable on a site whose certificate is broken. Ask instead
                // of silently trusting it.
                ui.post(() -> new AlertDialog.Builder(ShareActivity.this)
                        .setTitle("证书有问题")
                        .setMessage("这个页面的 HTTPS 证书校验失败：\n"
                                + error.getUrl() + "\n\n继续打开有一定风险，仅在你确认站点可信时继续。")
                        .setPositiveButton("继续", (d, w) -> handler.proceed())
                        .setNegativeButton("放弃", (d, w) -> {
                            handler.cancel();
                            finish();
                        })
                        .show());
            }
        });
    }

    private void startLoad() {
        configureWebView();
        web.loadUrl(targetUrl);
    }

    // ------------------------------------------------------------- capture

    private void beginCapture() {
        if (capturing) {
            return;
        }
        capturing = true;
        scrollTicks = 0;
        setStatus("正在滚动加载图片…");
        ui.postDelayed(this::scrollTick, 500);
    }

    private void scrollTick() {
        if (web == null) {
            return;
        }
        scrollTicks++;
        if (scrollTicks > MAX_SCROLL_TICKS) {
            settle();
            return;
        }
        web.evaluateJavascript(JS_SCROLL, value -> {
            if ("true".equals(value)) {
                settle();
            } else {
                ui.postDelayed(this::scrollTick, 260);
            }
        });
    }

    private void settle() {
        if (web == null) {
            return;
        }
        web.evaluateJavascript("window.scrollTo(0,0)", null);
        int wait = Prefs.settleMs(this);
        setStatus("等待页面渲染完成…");
        ui.postDelayed(this::extract, Math.max(300, wait));
    }

    /**
     * Pulls the page out in chunks.
     *
     * <p>evaluateJavascript hands its result back over Binder, which means anything
     * bigger than a few hundred kilobytes risks a failed transaction — and a real
     * article page is several megabytes of HTML. So the page is base64 encoded
     * inside the WebView and fetched 200 KB at a time, which keeps every result
     * well under the limit and survives CJK content.
     */
    private void extract() {
        if (web == null) {
            return;
        }
        setStatus("正在提取页面内容…");
        web.evaluateJavascript(JS_EXTRACT, value -> {
            int total;
            try {
                total = (int) Double.parseDouble(String.valueOf(value).replace("\"", ""));
            } catch (Exception e) {
                failed("提取页面失败：" + value);
                return;
            }
            if (total <= 0) {
                failed("页面内容为空，可能还没渲染完");
                return;
            }
            pullChunk(new StringBuilder(total), total);
        });
    }

    private void pullChunk(final StringBuilder b64, final int expected) {
        if (web == null) {
            return;
        }
        web.evaluateJavascript(JS_CHUNK, value -> {
            if (value == null || "null".equals(value)) {
                failed("读取页面内容中断");
                return;
            }
            String piece;
            try {
                piece = (String) new org.json.JSONTokener(value).nextValue();
            } catch (Exception e) {
                failed("解析页面片段失败：" + e);
                return;
            }
            b64.append(piece);
            if (b64.length() < expected) {
                setStatus("正在提取页面内容… " + (b64.length() * 100 / expected) + "%");
                pullChunk(b64, expected);
                return;
            }
            finishExtract(b64.toString());
        });
    }

    private void finishExtract(String b64) {
        String html;
        try {
            html = new String(android.util.Base64.decode(b64, android.util.Base64.DEFAULT), "UTF-8");
        } catch (Exception e) {
            failed("解码页面失败：" + e);
            return;
        }
        if (html.length() < 200) {
            failed("取到的内容太短（" + html.length() + " 字节），页面可能没有渲染出来");
            return;
        }
        web.evaluateJavascript(JS_META, value -> {
            String title = "";
            String url = finalUrl == null ? targetUrl : finalUrl;
            try {
                Object o = new org.json.JSONTokener(String.valueOf(value)).nextValue();
                JSONObject meta = new JSONObject((String) o);
                title = meta.optString("title", "");
                url = meta.optString("url", url);
            } catch (Exception ignored) {
                // A missing title is not worth failing the capture over.
            }
            submit(url, title, html);
        });
    }

    private void submit(final String url, final String title, final String html) {
        submitting = true;
        setStatus("正在提交到 Readeck（" + (html.length() / 1024) + " KB）…");
        final String base = Prefs.server(this);
        final String token = Prefs.token(this);
        new Thread(() -> {
            try {
                Api.createBookmark(base, token, url, title, html);
                ui.post(() -> {
                    setStatus("已保存：" + title);
                    Toast.makeText(this, "已存到 Readeck", Toast.LENGTH_SHORT).show();
                    ui.postDelayed(this::finish, 700);
                });
            } catch (final Exception e) {
                ui.post(() -> failed(describe(e)));
            }
        }).start();
    }

    private void failed(String message) {
        submitting = false;
        capturing = true;
        setStatus("失败：" + message);
        if (isFinishing()) {
            return;
        }
        new AlertDialog.Builder(this)
                .setTitle("没能存下完整页面")
                .setMessage(message)
                .setPositiveButton("重试", (d, w) -> {
                    capturing = false;
                    submitting = false;
                    startLoad();
                })
                .setNeutralButton("只存链接", (d, w) -> submitLinkOnly())
                .setNegativeButton("取消", (d, w) -> finish())
                .show();
    }

    private void submitLinkOnly() {
        final String base = Prefs.server(this);
        final String token = Prefs.token(this);
        final String url = targetUrl;
        setStatus("正在提交链接…");
        new Thread(() -> {
            try {
                Api.createBookmark(base, token, url, null, null);
                ui.post(() -> {
                    Toast.makeText(this, "已存为链接，正文由服务端抓取", Toast.LENGTH_SHORT).show();
                    finish();
                });
            } catch (final Exception e) {
                ui.post(() -> new AlertDialog.Builder(this)
                        .setTitle("提交失败")
                        .setMessage(describe(e))
                        .setPositiveButton("知道了", (d, w) -> finish())
                        .show());
            }
        }).start();
    }

    private static String describe(Exception e) {
        if (e instanceof Api.ApiException) {
            Api.ApiException ae = (Api.ApiException) e;
            return "HTTP " + ae.status + "\n" + ae.reason();
        }
        return String.valueOf(e.getMessage());
    }

    /** Pulls the first http(s) URL out of whatever the sharing app put in the intent. */
    static String extractUrl(Intent intent) {
        if (intent == null) {
            return null;
        }
        CharSequence text = intent.getCharSequenceExtra(Intent.EXTRA_TEXT);
        if (text == null) {
            // "转存" in a text selection menu arrives as PROCESS_TEXT instead.
            text = intent.getCharSequenceExtra(Intent.EXTRA_PROCESS_TEXT);
        }
        String body = text == null ? "" : text.toString();
        if (body.isEmpty() && intent.getDataString() != null) {
            body = intent.getDataString();
        }
        Matcher m = URL_RE.matcher(body);
        if (!m.find()) {
            return null;
        }
        String url = m.group();
        // Sharing apps love to append punctuation to the link.
        while (url.length() > 8 && "，。、；：！？,.;:!?)]}>\"'".indexOf(url.charAt(url.length() - 1)) >= 0) {
            url = url.substring(0, url.length() - 1);
        }
        return url;
    }
}
