// SPDX-License-Identifier: AGPL-3.0-only
package org.ovohui.readeckshare;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

/**
 * The handful of Readeck API calls the share target needs.
 *
 * <p>Everything here blocks; callers run it off the main thread.
 */
final class Api {
    /** Scopes asked from the server when logging in with a device code. */
    static final String SCOPE = "bookmarks:read bookmarks:write profile:read";

    /** HTTP status plus the body the server sent back, kept for the error dialog. */
    static final class ApiException extends Exception {
        final int status;
        final String body;

        ApiException(int status, String body) {
            super("HTTP " + status + (body == null || body.isEmpty() ? "" : " " + body));
            this.status = status;
            this.body = body == null ? "" : body;
        }

        /** Best effort human readable reason out of an API error document. */
        String reason() {
            try {
                JSONObject o = new JSONObject(body);
                if (o.has("error_description")) {
                    return o.getString("error_description");
                }
                if (o.has("error")) {
                    return o.getString("error");
                }
                if (o.has("message")) {
                    return o.getString("message");
                }
                if (o.has("errors")) {
                    return o.get("errors").toString();
                }
            } catch (Exception ignored) {
                // Not JSON, fall through.
            }
            return body.isEmpty() ? "HTTP " + status : body;
        }
    }

    private Api() {
    }

    static String normalize(String base) {
        String b = base == null ? "" : base.trim();
        while (b.endsWith("/")) {
            b = b.substring(0, b.length() - 1);
        }
        return b;
    }

    private static JSONObject call(String method, String url, JSONObject payload, String token,
                                   int readTimeoutMs) throws IOException, ApiException {
        HttpURLConnection conn = (HttpURLConnection) new URL(url).openConnection();
        try {
            conn.setRequestMethod(method);
            conn.setConnectTimeout(15000);
            conn.setReadTimeout(readTimeoutMs);
            conn.setRequestProperty("Accept", "application/json");
            conn.setRequestProperty("User-Agent", "ReadeckShare/1.0 (Android)");
            if (token != null && !token.isEmpty()) {
                conn.setRequestProperty("Authorization", "Bearer " + token);
            }
            if (payload != null) {
                byte[] raw = payload.toString().getBytes(StandardCharsets.UTF_8);
                conn.setDoOutput(true);
                conn.setRequestProperty("Content-Type", "application/json");
                conn.setFixedLengthStreamingMode(raw.length);
                try (OutputStream os = conn.getOutputStream()) {
                    os.write(raw);
                }
            }

            int code = conn.getResponseCode();
            InputStream in = code >= 400 ? conn.getErrorStream() : conn.getInputStream();
            String body = readAll(in);
            if (code >= 400) {
                throw new ApiException(code, body);
            }
            if (body.isEmpty()) {
                return new JSONObject();
            }
            try {
                return new JSONObject(body);
            } catch (Exception e) {
                // A 2xx with something that is not JSON (an HTML error page in
                // front of the API, a captive portal) must not crash the caller.
                throw new IOException("服务器返回了非 JSON 内容：" + body.substring(0, Math.min(200, body.length())));
            }
        } finally {
            conn.disconnect();
        }
    }

    private static String readAll(InputStream in) throws IOException {
        if (in == null) {
            return "";
        }
        try (InputStream src = in) {
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            byte[] buf = new byte[8192];
            int n;
            while ((n = src.read(buf)) > 0) {
                out.write(buf, 0, n);
            }
            return new String(out.toByteArray(), StandardCharsets.UTF_8);
        }
    }

    /** Registers this install as an OAuth client. Returns the client id. */
    static String registerClient(String base) throws IOException, ApiException {
        JSONObject body = new JSONObject();
        try {
            body.put("client_name", "Readeck 分享");
            body.put("client_uri", "https://github.com/ovohui/readeck");
            body.put("software_id", "org.ovohui.readeckshare");
            body.put("software_version", "1.0");
            body.put("redirect_uris", new org.json.JSONArray().put("readeckshare://oauth-callback"));
            body.put("grant_types",
                    new org.json.JSONArray().put("urn:ietf:params:oauth:grant-type:device_code"));
            body.put("token_endpoint_auth_method", "none");
            body.put("response_types", new org.json.JSONArray().put("code"));
        } catch (Exception e) {
            throw new IOException(e);
        }
        JSONObject rsp = call("POST", base + "/api/oauth/client", body, null, 20000);
        try {
            return rsp.getString("client_id");
        } catch (Exception e) {
            throw new IOException("服务器没有返回 client_id：" + rsp);
        }
    }

    /** Asks for a device code. The answer carries user_code and verification_uri. */
    static JSONObject startDevice(String base, String clientId) throws IOException, ApiException {
        JSONObject body = new JSONObject();
        try {
            body.put("client_id", clientId);
            body.put("scope", SCOPE);
        } catch (Exception e) {
            throw new IOException(e);
        }
        return call("POST", base + "/api/oauth/device", body, null, 20000);
    }

    /**
     * One poll of the token endpoint. Returns the access token, or null while the
     * user has not approved the device yet.
     */
    static String pollDevice(String base, String clientId, String deviceCode)
            throws IOException, ApiException {
        JSONObject body = new JSONObject();
        try {
            body.put("grant_type", "urn:ietf:params:oauth:grant-type:device_code");
            body.put("device_code", deviceCode);
            body.put("client_id", clientId);
        } catch (Exception e) {
            throw new IOException(e);
        }
        try {
            JSONObject rsp = call("POST", base + "/api/oauth/token", body, null, 20000);
            return rsp.optString("access_token", "");
        } catch (ApiException e) {
            if (e.status == 400 && e.body.contains("authorization_pending")) {
                return null;
            }
            throw e;
        }
    }

    /** Reads the profile, which doubles as a "is this token still good" check. */
    static JSONObject profile(String base, String token) throws IOException, ApiException {
        return call("GET", base + "/api/profile", null, token, 20000);
    }

    /**
     * Stores a bookmark from an already rendered page.
     *
     * <p>Sending the html is the whole point: the server does not have to fetch the
     * page again, so sites whose body only exists after their own JavaScript has run
     * (single page apps, share landing pages) still archive with their content.
     */
    static void createBookmark(String base, String token, String url, String title, String html)
            throws IOException, ApiException {
        JSONObject body = new JSONObject();
        try {
            body.put("url", url);
            if (title != null && !title.isEmpty()) {
                body.put("title", title);
            }
            if (html != null && !html.isEmpty()) {
                body.put("html", html);
            }
        } catch (Exception e) {
            throw new IOException(e);
        }
        // A large page takes a while to transfer and the server then extracts it.
        call("POST", base + "/api/bookmarks", body, token, 120000);
    }
}
