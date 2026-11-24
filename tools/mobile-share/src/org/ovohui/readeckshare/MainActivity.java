// SPDX-License-Identifier: AGPL-3.0-only
package org.ovohui.readeckshare;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.ClipboardManager;
import android.content.Intent;
import android.net.Uri;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.InputType;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;

import org.json.JSONObject;

/** Settings screen: server address, login, capture options. */
public class MainActivity extends Activity {
    private final Handler ui = new Handler(Looper.getMainLooper());

    private EditText linkField;
    private EditText serverField;
    private EditText tokenField;
    private EditText settleField;
    private CheckBox desktopBox;
    private TextView statusView;

    private volatile boolean polling;
    private String pendingClientId;
    private String pendingDeviceCode;
    private int pollIntervalSeconds = 5;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(buildUi());
        load();
    }

    @Override
    protected void onDestroy() {
        polling = false;
        super.onDestroy();
    }

    private int dp(int v) {
        return Math.round(getResources().getDisplayMetrics().density * v);
    }

    private View buildUi() {
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(dp(20), dp(20), dp(20), dp(24));

        TextView title = new TextView(this);
        title.setText("转存 Readeck");
        title.setTextSize(22);
        root.addView(title);

        TextView intro = new TextView(this);
        intro.setText("转存 Readeck：把网页完整渲染后交给服务端，正文和图片一起存下来。"
                + "可以在任何 App 里「分享」给本应用，也可以在下面粘贴一个链接。");
        intro.setTextSize(13);
        intro.setPadding(0, dp(8), 0, dp(16));
        root.addView(intro);

        root.addView(label("链接"));
        linkField = field(false);
        linkField.setHint("粘贴分享来的链接");
        root.addView(linkField);

        LinearLayout linkRow = new LinearLayout(this);
        linkRow.setOrientation(LinearLayout.HORIZONTAL);
        Button paste = new Button(this);
        paste.setText("粘贴");
        paste.setOnClickListener(v -> pasteFromClipboard());
        linkRow.addView(paste, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        Button grab = new Button(this);
        grab.setText("抓取并保存");
        grab.setOnClickListener(v -> captureFromField());
        linkRow.addView(grab, new LinearLayout.LayoutParams(0,
                ViewGroup.LayoutParams.WRAP_CONTENT, 1));
        root.addView(linkRow);

        root.addView(label("服务器地址"));
        serverField = field(false);
        serverField.setHint(Prefs.DEFAULT_SERVER);
        root.addView(serverField);

        root.addView(label("访问令牌"));
        tokenField = field(false);
        tokenField.setHint("登录后自动填写，也可粘一个 API token");
        root.addView(tokenField);

        Button device = new Button(this);
        device.setText("用设备码登录（推荐）");
        device.setOnClickListener(v -> startDeviceLogin());
        root.addView(device);

        Button test = new Button(this);
        test.setText("测试连接");
        test.setOnClickListener(v -> testConnection());
        root.addView(test);

        Button save = new Button(this);
        save.setText("保存");
        save.setOnClickListener(v -> {
            persist();
            toast("已保存");
        });
        root.addView(save);

        root.addView(label("抓取选项"));
        desktopBox = new CheckBox(this);
        desktopBox.setText("使用桌面版 UA（页面更完整，适合论坛与图文站）");
        root.addView(desktopBox);

        settleField = field(true);
        settleField.setHint("渲染后等待毫秒数，默认 2000");
        root.addView(settleField);

        statusView = new TextView(this);
        statusView.setTextSize(12);
        statusView.setPadding(0, dp(16), 0, 0);
        root.addView(statusView);

        TextView hint = new TextView(this);
        hint.setTextSize(12);
        hint.setPadding(0, dp(16), 0, 0);
        hint.setText("提示：第一次用先登录。服务端地址必须能在手机上打开，"
                + "自建实例记得带上端口，例如 https://example.com:15351。");
        root.addView(hint);

        ScrollView scroll = new ScrollView(this);
        scroll.addView(root, new ViewGroup.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        return scroll;
    }

    private TextView label(String text) {
        TextView v = new TextView(this);
        v.setText(text);
        v.setTextSize(13);
        v.setPadding(0, dp(14), 0, dp(4));
        return v;
    }

    private EditText field(boolean number) {
        EditText e = new EditText(this);
        e.setSingleLine(true);
        if (number) {
            e.setInputType(InputType.TYPE_CLASS_NUMBER);
        }
        return e;
    }

    /** Fills the link box from the clipboard, which is how a copied share link arrives. */
    private void pasteFromClipboard() {
        try {
            ClipboardManager cm = (ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
            CharSequence text = cm == null ? null : cm.getPrimaryClip() == null ? null
                    : cm.getPrimaryClip().getItemAt(0).coerceToText(this);
            if (text == null || text.length() == 0) {
                toast("剪贴板是空的");
                return;
            }
            linkField.setText(text.toString().trim());
        } catch (Exception e) {
            toast("读不到剪贴板：" + e.getMessage());
        }
    }

    /** Sends the typed link through the same pipeline a share would use. */
    private void captureFromField() {
        String url = linkField.getText().toString().trim();
        if (url.isEmpty()) {
            toast("先粘贴一个链接");
            return;
        }
        if (Prefs.token(this).isEmpty()) {
            toast("先登录");
            return;
        }
        Intent i = new Intent(this, ShareActivity.class);
        i.setAction(Intent.ACTION_SEND);
        i.setType("text/plain");
        i.putExtra(Intent.EXTRA_TEXT, url);
        startActivity(i);
    }

    private void load() {
        serverField.setText(Prefs.server(this));
        tokenField.setText(Prefs.token(this));
        desktopBox.setChecked(Prefs.desktopUA(this));
        settleField.setText(String.valueOf(Prefs.settleMs(this)));
        refreshStatus();
    }

    private void persist() {
        Prefs.setServer(this, serverField.getText().toString());
        Prefs.setToken(this, tokenField.getText().toString());
        Prefs.setDesktopUA(this, desktopBox.isChecked());
        try {
            int ms = Integer.parseInt(settleField.getText().toString().trim());
            Prefs.setSettleMs(this, Math.max(0, Math.min(15000, ms)));
        } catch (Exception ignored) {
            // Keep the previous value when the field is not a number.
        }
        load();
    }

    private void refreshStatus() {
        String token = Prefs.token(this);
        statusView.setText("服务器：" + Prefs.server(this) + "\n令牌："
                + (token.isEmpty() ? "未登录" : token.substring(0, Math.min(12, token.length())) + "…"));
    }

    private void toast(String msg) {
        Toast.makeText(this, msg, Toast.LENGTH_SHORT).show();
    }

    private void testConnection() {
        persist();
        final String base = Prefs.server(this);
        final String token = Prefs.token(this);
        if (token.isEmpty()) {
            toast("先去登录");
            return;
        }
        toast("正在连接…");
        new Thread(() -> {
            try {
                JSONObject p = Api.profile(base, token);
                final String name = p.optString("user", p.optString("username", "ok"));
                ui.post(() -> toast("连接成功：" + name));
            } catch (final Exception e) {
                ui.post(() -> new AlertDialog.Builder(this)
                        .setTitle("连接失败")
                        .setMessage(describe(e))
                        .setPositiveButton("知道了", null)
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

    // ---------------------------------------------------------------- login

    private void startDeviceLogin() {
        persist();
        final String base = Prefs.server(this);
        polling = true;
        statusView.setText("正在向 " + base + " 申请设备码…");

        new Thread(() -> {
            try {
                final String clientId = Api.registerClient(base);
                final JSONObject dev = Api.startDevice(base, clientId);
                pendingClientId = clientId;
                pendingDeviceCode = dev.getString("device_code");
                pollIntervalSeconds = Math.max(1, dev.optInt("interval", 5));
                final String userCode = dev.getString("user_code");
                final String verify = dev.optString("verification_uri",
                        base + "/device");
                final String verifyComplete = dev.optString("verification_uri_complete", verify);

                ui.post(() -> showDeviceDialog(userCode, verifyComplete));
                pollLoop(base, verify);
            } catch (final Exception e) {
                polling = false;
                ui.post(() -> {
                    statusView.setText("登录失败：" + describe(e));
                    new AlertDialog.Builder(this)
                            .setTitle("登录失败")
                            .setMessage(describe(e)
                                    + "\n\n如果是 400 invalid client URI，说明服务端版本较旧，"
                                    + "升级到包含 OAuth 修复的镜像即可。")
                            .setPositiveButton("知道了", null)
                            .show();
                });
            }
        }).start();
    }

    private void showDeviceDialog(final String userCode, final String verifyUri) {
        statusView.setText("等待授权：" + userCode);
        new AlertDialog.Builder(this)
                .setTitle("在浏览器里授权")
                .setMessage("已在浏览器打开授权页。输入下面的代码并点「授权」：\n\n"
                        + userCode + "\n\n授权完成后本页会自动继续。")
                .setPositiveButton("打开授权页", (d, w) -> {
                    try {
                        startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(verifyUri)));
                    } catch (Exception e) {
                        toast("打不开浏览器，请手动访问 " + verifyUri);
                    }
                })
                .setNegativeButton("取消", (d, w) -> polling = false)
                .show();
        try {
            startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(verifyUri)));
        } catch (Exception ignored) {
            // The dialog keeps the address available.
        }
    }

    private void pollLoop(final String base, final String verifyUri) {
        new Thread(() -> {
            long deadline = System.currentTimeMillis() + 5 * 60 * 1000L;
            while (polling && System.currentTimeMillis() < deadline) {
                try {
                    Thread.sleep(pollIntervalSeconds * 1000L);
                } catch (InterruptedException e) {
                    return;
                }
                try {
                    String token = Api.pollDevice(base, pendingClientId, pendingDeviceCode);
                    if (token != null && !token.isEmpty()) {
                        Prefs.setToken(this, token);
                        polling = false;
                        ui.post(() -> {
                            tokenField.setText(token);
                            refreshStatus();
                            statusView.setText("已登录");
                            toast("登录成功");
                        });
                        return;
                    }
                } catch (Api.ApiException e) {
                    if (e.body.contains("slow_down")) {
                        pollIntervalSeconds += 5;
                        continue;
                    }
                    polling = false;
                    final String msg = e.reason();
                    ui.post(() -> statusView.setText("登录中断：" + msg));
                    return;
                } catch (Exception e) {
                    polling = false;
                    final String msg = describe(e);
                    ui.post(() -> statusView.setText("登录中断：" + msg));
                    return;
                }
            }
            if (polling) {
                polling = false;
                ui.post(() -> statusView.setText("授权超时，请重新登录"));
            }
        }).start();
    }
}
