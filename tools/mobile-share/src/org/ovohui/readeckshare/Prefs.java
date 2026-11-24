// SPDX-License-Identifier: AGPL-3.0-only
package org.ovohui.readeckshare;

import android.content.Context;
import android.content.SharedPreferences;

/** Settings shared by the two activities. */
final class Prefs {
    static final String DEFAULT_SERVER = "https://myreadeck.zh.ovohui.com:15351";

    private Prefs() {
    }

    private static SharedPreferences sp(Context ctx) {
        return ctx.getSharedPreferences("readeck-share", Context.MODE_PRIVATE);
    }

    static String server(Context ctx) {
        String v = sp(ctx).getString("server", DEFAULT_SERVER);
        return Api.normalize(v == null || v.isEmpty() ? DEFAULT_SERVER : v);
    }

    static void setServer(Context ctx, String v) {
        sp(ctx).edit().putString("server", Api.normalize(v)).apply();
    }

    static String token(Context ctx) {
        String v = sp(ctx).getString("token", "");
        return v == null ? "" : v;
    }

    static void setToken(Context ctx, String v) {
        sp(ctx).edit().putString("token", v == null ? "" : v.trim()).apply();
    }

    static boolean desktopUA(Context ctx) {
        return sp(ctx).getBoolean("desktopUA", true);
    }

    static void setDesktopUA(Context ctx, boolean v) {
        sp(ctx).edit().putBoolean("desktopUA", v).apply();
    }

    static int settleMs(Context ctx) {
        return sp(ctx).getInt("settleMs", 2000);
    }

    static void setSettleMs(Context ctx, int v) {
        sp(ctx).edit().putInt("settleMs", v).apply();
    }
}
