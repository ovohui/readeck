#!/usr/bin/env python3
"""Adds the "render the page, then save it" share target to the official APK.

The stock client only forwards the URL, which is useless for sites whose body
is fetched by their own scripts. This script bolts the capture activity from
tools/mobile-share onto the shipped APK, so one app both browses the library
and saves rendered pages.

What it changes:
  * removes requiredSplitTypes from the manifest (the result is one APK, not a
    bundle, so nothing may demand an ABI split at install time);
  * merges the ABI split's native libraries back into the APK;
  * disables the stock share activity's SEND filter and registers ours instead,
    with SEND + PROCESS_TEXT, plus a launcher entry that offers a paste box;
  * appends our classes as classes3.dex (minSdk 32, so multidex is native).

Everything runs in an ASCII temp directory: aapt2 cannot open a path that the
Windows ANSI code page cannot represent, and this repository is called
"Readeck 自定义改造".
"""
from __future__ import annotations

import argparse
import glob
import os
import re
import shutil
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
ATOOL = REPO / ".workbuddy" / "tmp" / "atool"

ANDROID = "http://schemas.android.com/apk/res/android"
ET.register_namespace("android", ANDROID)

KS_NAME = "readeck-share.jks"
KS_PASS = "readeck123"
KS_ALIAS = "readeckshare"

STOCK_SHARE_ACTIVITY = "com.mydeck.app.ui.share.ShareActivity"
OUR_SHARE = "org.ovohui.readeckshare.ShareActivity"
OUR_SETTINGS = "org.ovohui.readeckshare.MainActivity"


def log(msg: str) -> None:
    print(f"[patch] {msg}", flush=True)


def die(msg: str) -> "None":
    print(f"[patch] ERROR: {msg}", file=sys.stderr)
    raise SystemExit(1)


def first(pattern: str) -> Path | None:
    hits = sorted(glob.glob(pattern, recursive=True))
    return Path(hits[0]) if hits else None


def find_jdk() -> Path:
    for env in ("JDK_HOME", "JAVA_HOME"):
        v = os.environ.get(env)
        if v and (Path(v) / "bin" / "javac.exe").exists():
            return Path(v)
    p = first(str(ATOOL / "jdk-*" / "bin" / "javac.exe")) or first(str(ATOOL / "**" / "bin" / "javac.exe"))
    if p:
        return p.parent.parent
    die("cannot find the JDK")


def find_build_tools() -> Path:
    p = first(str(ATOOL / "bt" / "*" / "aapt2.exe")) or first(str(ATOOL / "**" / "aapt2.exe"))
    if not p:
        die("cannot find Android build-tools")
    return p.parent


def find_android_jar() -> Path:
    p = first(str(ATOOL / "plat" / "**" / "android.jar")) or first(str(ATOOL / "**" / "android.jar"))
    if not p:
        die("cannot find android.jar")
    return p


def find_apktool() -> Path:
    # Newer apktool knows the newer resource attributes; the app is built
    # against a very recent compileSdk, so prefer the latest jar available.
    cands = [REPO / ".workbuddy" / "tmp" / "apktool3.jar",
             REPO / ".workbuddy" / "tmp" / "apktool.jar",
             ATOOL / "apktool.jar"]
    cands += [Path(p) for p in sorted(glob.glob(str(REPO / ".workbuddy" / "tmp" / "apktool_*.jar")))]
    for c in cands:
        if c.exists():
            return c
    die("cannot find apktool.jar (put it in .workbuddy/tmp/)")


def run(cmd: list, **kw) -> None:
    log(" ".join(str(c) for c in cmd[:2]))
    res = subprocess.run([str(c) for c in cmd], **kw)
    if res.returncode != 0:
        die(f"command failed ({res.returncode}): {cmd[0]}")


# ----------------------------------------------------------------- manifest

def q(attr: str) -> str:
    return f"{{{ANDROID}}}{attr}"


def build_manifest(dec: Path) -> None:
    path = dec / "AndroidManifest.xml"
    tree = ET.parse(path)
    root = tree.getroot()

    # One APK instead of a bundle: no component may require a split.
    for attr in ("requiredSplitTypes", "splitTypes"):
        if q(attr) in root.attrib:
            del root.attrib[q(attr)]

    app = root.find("application")
    if app is None:
        die("manifest has no <application>")

    # The stock share activity is replaced by ours; keep the activity itself,
    # drop only its SEND filter so the chooser shows a single entry.
    removed = 0
    for act in app.findall("activity"):
        if act.get(q("name")) == STOCK_SHARE_ACTIVITY:
            for filt in act.findall("intent-filter"):
                if any(a.get(q("name")) == "android.intent.action.SEND" for a in filt.findall("action")):
                    act.remove(filt)
                    removed += 1
    if removed == 0:
        log("warning: stock share filter not found, nothing removed")
    else:
        log(f"removed {removed} stock SEND filter(s)")

    def activity(name: str, label: str, theme: str, main: bool) -> ET.Element:
        act = ET.Element("activity")
        act.set(q("name"), name)
        act.set(q("label"), label)
        act.set(q("exported"), "true")
        act.set(q("theme"), theme)
        return act

    share = activity(OUR_SHARE, "转存 Readeck", "@android:style/Theme.Material.Light.NoActionBar", False)
    share.set(q("launchMode"), "singleTop")
    share.set(q("configChanges"), "orientation|screenSize|keyboardHidden|screenLayout")

    send = ET.SubElement(share, "intent-filter")
    ET.SubElement(send, "action").set(q("name"), "android.intent.action.SEND")
    ET.SubElement(send, "category").set(q("name"), "android.intent.category.DEFAULT")
    ET.SubElement(send, "data").set(q("mimeType"), "text/plain")

    # Selecting text in any app offers "转存 Readeck".
    proc = ET.SubElement(share, "intent-filter")
    ET.SubElement(proc, "action").set(q("name"), "android.intent.action.PROCESS_TEXT")
    ET.SubElement(proc, "category").set(q("name"), "android.intent.category.DEFAULT")
    ET.SubElement(proc, "data").set(q("mimeType"), "text/plain")

    settings = activity(OUR_SETTINGS, "转存 Readeck", "@android:style/Theme.Material.Light.DarkActionBar", True)
    launcher = ET.SubElement(settings, "intent-filter")
    ET.SubElement(launcher, "action").set(q("name"), "android.intent.action.MAIN")
    ET.SubElement(launcher, "category").set(q("name"), "android.intent.category.LAUNCHER")

    app.append(share)
    app.append(settings)

    # Android 13 lets a user override the app language. Without this list the
    # entry does not appear in Settings > Apps > Readeck > Language, which is
    # the only way to get a Chinese UI on a phone whose system language is not.
    xml_dir = dec / "res" / "xml"
    if xml_dir.exists() and not (xml_dir / "locales_config.xml").exists():
        (xml_dir / "locales_config.xml").write_text(
            '<?xml version="1.0" encoding="utf-8"?>\n'
            '<locale-config xmlns:android="http://schemas.android.com/apk/res/android">\n'
            '    <locale android:name="zh-CN"/>\n'
            '    <locale android:name="zh-TW"/>\n'
            '    <locale android:name="zh-HK"/>\n'
            '    <locale android:name="en"/>\n'
            '</locale-config>\n',
            encoding="utf-8")
        app.set(q("localeConfig"), "@xml/locales_config")
        log("locale config added")

    # Marks the build. The About screen reads versionName through PackageInfo,
    # there is no BuildConfig constant in the dex, so this value is what the
    # app displays. versionCode goes up as well: the stock release is 1001000.
    root.set(q("versionCode"), "1001001")
    root.set(q("versionName"), "1.1.0-cn")

    # The stock manifest hardcodes the label instead of pointing at app_name,
    # so the string resource alone would not rename the icon.
    app.set(q("label"), APP_LABEL)

    tree.write(path, encoding="utf-8", xml_declaration=True)
    log("manifest updated")


def merge_locales(java: Path, apktool: Path, work: Path, dec: Path, splits: list) -> None:
    """Pulls the language resources out of the config splits.

    The upstream bundle ships its translations as separate APKs, so a merged
    single APK ends up with English only. Each split decodes into
    res/values-<locale>/, and those directories can simply be dropped next to
    the base resources: the string names already exist, so aapt2 keeps the
    existing ids and just adds another configuration for them.
    """
    for split in splits:
        split = Path(split).resolve()
        if not split.exists():
            die(f"no such locale split: {split}")
        out = work / ("locale-" + split.stem)
        run([java, "-jar", apktool, "d", "-s", "-f", "-o", out, split])
        res = out / "res"
        if not res.exists():
            die(f"{split.name} has no resources")
        copied = 0
        for values in sorted(res.iterdir()):
            if not values.is_dir() or not values.name.startswith("values-"):
                continue
            target = dec / "res" / values.name
            if target.exists():
                shutil.rmtree(target)
            shutil.copytree(values, target)
            copied += 1
        log(f"{split.name}: merged {copied} locale resource dir(s)")


def apply_locale_overlay(dec: Path) -> None:
    """Adds our own translations on top of whatever the splits provided.

    The upstream language pack covers 339 of 671 strings, so half of the UI
    would still be English. locale/values-<qualifier>/*.xml only holds the
    missing names, which keeps it a pure addition: same qualifier, different
    resource names, no duplicates.
    """
    src = HERE / "locale"
    if not src.exists():
        return
    for values in sorted(src.iterdir()):
        if not values.is_dir():
            continue
        target = dec / "res" / values.name
        target.mkdir(parents=True, exist_ok=True)
        for f in values.iterdir():
            shutil.copy2(f, target / f.name)
        log(f"overlay {values.name}: {len(list(values.iterdir()))} file(s)")


RX_STRING = re.compile(r'(<string\s+name="([^"]+)"([^>]*)>)(.*?)(</string>)', re.S)
RX_PLURALS = re.compile(r'(<plurals\s+name="([^"]+)"[^>]*>)(.*?)(</plurals>)', re.S)
RX_APP_NAME = re.compile(r'(<string\s+name="app_name"[^>]*>)(.*?)(</string>)', re.S)

APP_LABEL = "Readeck 中文版"


def tag_app_label(dec: Path, label: str = APP_LABEL) -> None:
    """Renames the launcher label so the installed build is recognisable.

    Version numbers are a poor way to tell two APKs apart: they sit two taps
    deep in About, and the previous build already claimed 1.1.0. The name
    under the icon is visible from the home screen, and it survives whatever
    the About screen chooses to display.
    """
    res = dec / "res"
    changed = 0
    for path in sorted(res.glob("values*/strings.xml")):
        text = path.read_text(encoding="utf-8")
        new = RX_APP_NAME.sub(lambda m: f"{m.group(1)}{label}{m.group(3)}", text)
        if new != text:
            path.write_text(new, encoding="utf-8")
            changed += 1
    if changed == 0:
        die("app_name not found, cannot tag the build")
    log(f"launcher label set to {label} in {changed} file(s)")


def _read_string_map(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}
    text = path.read_text(encoding="utf-8")
    return {m.group(2): m.group(4) for m in RX_STRING.finditer(text)}


def _read_plurals_map(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}
    text = path.read_text(encoding="utf-8")
    return {m.group(2): m.group(3) for m in RX_PLURALS.finditer(text)}


def force_default_chinese(dec: Path) -> None:
    """Promotes the Chinese resources to the default configuration.

    A phone whose locale is zh-Hans, zh-SG or anything else that is not
    exactly zh-CN / zh-TW / zh-HK never matches a zh-rCN configuration and
    silently falls back to values/, which is English - and on Android 12 the
    per-app language entry does not exist at all. This build has exactly one
    audience, so the default values are replaced with the Chinese ones: the
    interface is Chinese whatever the system locale is. The values-zh-*
    directories are left alone, so the Android 13 language picker still lists
    them.
    """
    res = dec / "res"
    strings = _read_string_map(res / "values-zh-rCN" / "strings.xml")
    strings.update(_read_string_map(HERE / "locale" / "values-zh-rCN" / "strings_extra.xml"))
    plurals = _read_plurals_map(res / "values-zh-rCN" / "plurals.xml")
    plurals.update(_read_plurals_map(HERE / "locale" / "values-zh-rCN" / "plurals_extra.xml"))

    if not strings:
        die("no Chinese strings found, nothing to promote")

    replaced = [0]

    def sub_string(m: re.Match) -> str:
        head, name, attrs, inner, tail = m.group(1), m.group(2), m.group(3), m.group(4), m.group(5)
        if 'translatable="false"' in attrs:
            return m.group(0)
        value = strings.get(name)
        if value is None or value.strip() == inner.strip():
            return m.group(0)
        replaced[0] += 1
        return f"{head}{value}{tail}"

    path = res / "values" / "strings.xml"
    text = RX_STRING.sub(sub_string, path.read_text(encoding="utf-8"))
    path.write_text(text, encoding="utf-8")
    log(f"default strings translated: {replaced[0]}")

    replaced[0] = 0

    def sub_plurals(m: re.Match) -> str:
        head, name, inner, tail = m.group(1), m.group(2), m.group(3), m.group(4)
        value = plurals.get(name)
        if value is None or value.strip() == inner.strip():
            return m.group(0)
        replaced[0] += 1
        return f"{head}{value}{tail}"

    path = res / "values" / "plurals.xml"
    if path.exists():
        text = RX_PLURALS.sub(sub_plurals, path.read_text(encoding="utf-8"))
        path.write_text(text, encoding="utf-8")
        log(f"default plurals translated: {replaced[0]}")


# -------------------------------------------------------------------- build

def compile_dex(work: Path, jdk: Path, android_jar: Path, d8_jar: Path, min_api: str) -> Path:
    src = work / "src"
    shutil.copytree(HERE / "src", src)
    classes = work / "classes"
    dex_dir = work / "dex"
    classes.mkdir()
    dex_dir.mkdir()

    sources = sorted(str(p) for p in src.rglob("*.java"))
    javac = jdk / "bin" / "javac.exe"
    java = jdk / "bin" / "java.exe"
    run([javac, "-encoding", "UTF-8", "-source", "11", "-target", "11", "-nowarn",
         "-classpath", android_jar, "-d", classes, *sources])
    run([java, "-cp", d8_jar, "com.android.tools.r8.D8",
         "--min-api", min_api, "--lib", android_jar,
         "--output", dex_dir, *sorted(str(p) for p in classes.rglob("*.class"))])
    return dex_dir / "classes.dex"


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", required=True, help="the shipped base.apk")
    ap.add_argument("--abi-split", help="split_config.arm64_v8a.apk to merge in")
    ap.add_argument("--out", default=str(HERE / "out" / "readeck-patched.apk"))
    ap.add_argument("--locale-split", action="append", default=[],
                    help="a split_config.xx.apk holding translations (repeatable)")
    ap.add_argument("--min-api", default="32")
    args = ap.parse_args()

    base = Path(args.base).resolve()
    if not base.exists():
        die(f"no such file: {base}")
    abi = Path(args.abi_split).resolve() if args.abi_split else None

    jdk = find_jdk()
    bt = find_build_tools()
    android_jar = find_android_jar()
    apktool = find_apktool()
    aapt2, zipalign = bt / "aapt2.exe", bt / "zipalign.exe"
    apksigner_jar, d8_jar = bt / "lib" / "apksigner.jar", bt / "lib" / "d8.jar"
    java = jdk / "bin" / "java.exe"

    work = Path(tempfile.mkdtemp(prefix="rdpatch-"))
    log(f"work {work}")

    stage_android = work / "android.jar"
    shutil.copy2(android_jar, stage_android)
    shutil.copy2(base, work / "base.apk")
    shutil.copy2(apktool, work / "apktool.jar")

    dex = compile_dex(work, jdk, stage_android, d8_jar, args.min_api)
    log(f"compiled {dex.stat().st_size} bytes of dex")

    run([java, "-jar", work / "apktool.jar", "d", "-s", "-f",
         "-o", work / "dec", work / "base.apk"])
    build_manifest(work / "dec")
    if args.locale_split:
        merge_locales(java, work / "apktool.jar", work, work / "dec", args.locale_split)
        apply_locale_overlay(work / "dec")
        force_default_chinese(work / "dec")
        tag_app_label(work / "dec")
    run([java, "-jar", work / "apktool.jar", "b", work / "dec",
         "-o", work / "patched-unsigned.apk"])

    merged = work / "merged.apk"
    with zipfile.ZipFile(work / "patched-unsigned.apk") as src_zip, \
            zipfile.ZipFile(merged, "w", zipfile.ZIP_DEFLATED) as dst:
        for item in src_zip.infolist():
            dst.writestr(item, src_zip.read(item.filename))
        dst.write(dex, "classes3.dex")
        log("added classes3.dex")

    if abi and abi.exists():
        added = 0
        with zipfile.ZipFile(abi) as abi_zip:
            with zipfile.ZipFile(merged, "a") as dst:
                for name in abi_zip.namelist():
                    if not name.startswith("lib/"):
                        continue
                    # extractNativeLibs="false": the loader maps these straight
                    # out of the APK, so they must be stored, not deflated.
                    info = zipfile.ZipInfo(name)
                    info.compress_type = zipfile.ZIP_STORED
                    info.external_attr = 0o644 << 16
                    dst.writestr(info, abi_zip.read(name))
                    added += 1
        log(f"merged {added} native librar{'y' if added == 1 else 'ies'}")

    aligned = work / "aligned.apk"
    run([zipalign, "-p", "-f", "4", merged, aligned])

    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    if out.exists():
        out.unlink()

    ks = HERE / KS_NAME
    stage_ks = work / KS_NAME
    shutil.copy2(ks, stage_ks)
    run([java, "-jar", apksigner_jar, "sign", "--ks", stage_ks,
         "--ks-key-alias", KS_ALIAS,
         "--ks-pass", f"pass:{KS_PASS}", "--key-pass", f"pass:{KS_PASS}",
         "--out", out, aligned])
    verify = subprocess.run(
        [str(java), "-jar", str(apksigner_jar), "verify", "--print-certs", str(out)],
        capture_output=True, text=True)
    if verify.returncode != 0:
        die("signature verification failed:\n" + verify.stdout + verify.stderr)
    log(verify.stdout.strip().splitlines()[0])

    badging = subprocess.run([str(aapt2), "dump", "badging", str(out)],
                             capture_output=True, text=True).stdout
    for line in badging.splitlines()[:3]:
        log(line)

    log(f"OK  {out}  {out.stat().st_size / 1024 / 1024:.1f} MB")
    shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    main()
