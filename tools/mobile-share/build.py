#!/usr/bin/env python3
"""Builds the share target APK without Gradle.

Needs three things on disk:
  * a JDK (javac, keytool, java),
  * Android build-tools (aapt2, d8.jar, zipalign, apksigner),
  * an android.jar from one platform.

Point them with JDK_HOME / BUILD_TOOLS / ANDROID_JAR, or drop the unzipped
archives under .workbuddy/tmp/atool/ and the script finds them.

Everything is compiled in a scratch directory with an ASCII path: aapt2 on
Windows cannot open a directory whose name is not representable in the ANSI
code page, and this repository lives under "Readeck 自定义改造".
"""
from __future__ import annotations

import glob
import os
import shutil
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
REPO = HERE.parents[1]
ATOOL = REPO / ".workbuddy" / "tmp" / "atool"

MIN_SDK = "24"
TARGET_SDK = "34"
KS_PASS = "readeck123"
KS_ALIAS = "readeckshare"
KS_NAME = "readeck-share.jks"


def log(msg: str) -> None:
    print(f"[build] {msg}", flush=True)


def die(msg: str) -> "None":
    print(f"[build] ERROR: {msg}", file=sys.stderr)
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
    die("cannot find the JDK (set JDK_HOME, or unzip one under .workbuddy/tmp/atool/)")


def find_build_tools() -> Path:
    v = os.environ.get("BUILD_TOOLS")
    if v and (Path(v) / "aapt2.exe").exists():
        return Path(v)
    p = first(str(ATOOL / "bt" / "*" / "aapt2.exe")) or first(str(ATOOL / "**" / "aapt2.exe"))
    if p:
        return p.parent
    die("cannot find Android build-tools (set BUILD_TOOLS)")


def find_android_jar() -> Path:
    v = os.environ.get("ANDROID_JAR")
    if v and Path(v).exists():
        return Path(v)
    p = first(str(ATOOL / "plat" / "**" / "android.jar")) or first(str(ATOOL / "**" / "android.jar"))
    if p:
        return p
    die("cannot find android.jar (set ANDROID_JAR)")


def run(cmd: list, **kw) -> None:
    shown = " ".join(str(c) for c in cmd[:2])
    log(shown + (" ..." if len(cmd) > 2 else ""))
    res = subprocess.run([str(c) for c in cmd], **kw)
    if res.returncode != 0:
        die(f"command failed ({res.returncode}): {cmd[0]}")


def main() -> None:
    jdk = find_jdk()
    bt = find_build_tools()
    android_jar = find_android_jar()

    java = jdk / "bin" / "java.exe"
    javac = jdk / "bin" / "javac.exe"
    keytool = jdk / "bin" / "keytool.exe"
    aapt2 = bt / "aapt2.exe"
    zipalign = bt / "zipalign.exe"
    apksigner_jar = bt / "lib" / "apksigner.jar"
    d8_jar = bt / "lib" / "d8.jar"

    for p in (java, javac, keytool, aapt2, zipalign, apksigner_jar, d8_jar):
        if not Path(p).exists():
            die(f"missing {p}")

    log(f"jdk          {jdk}")
    log(f"build-tools  {bt}")
    log(f"android.jar  {android_jar}")

    stage = Path(tempfile.mkdtemp(prefix="rdbuild-"))
    log(f"stage        {stage}")

    shutil.copytree(HERE / "res", stage / "res")
    shutil.copy2(HERE / "AndroidManifest.xml", stage / "AndroidManifest.xml")
    # aapt2 and the rest are ANSI-only on Windows: keep every path it touches ASCII.
    android_jar = shutil.copy2(android_jar, stage / "android.jar")
    (stage / "classes").mkdir()
    (stage / "dex").mkdir()

    sources = sorted(str(p) for p in (HERE / "src").rglob("*.java"))
    if not sources:
        die("no java sources")

    # 1. javac -> class files
    run([javac, "-encoding", "UTF-8", "-source", "11", "-target", "11", "-nowarn",
         "-classpath", android_jar, "-d", stage / "classes", *sources])

    # 2. d8 -> classes.dex
    classes = sorted(str(p) for p in (stage / "classes").rglob("*.class"))
    run([java, "-cp", d8_jar, "com.android.tools.r8.D8",
         "--min-api", MIN_SDK, "--lib", android_jar,
         "--output", stage / "dex", *classes])

    # 3. aapt2 compile + link
    run([aapt2, "compile", "--dir", stage / "res", "-o", stage / "res.zip"])
    raw_apk = stage / "base-unsigned.apk"
    run([aapt2, "link", "-o", raw_apk,
         "-I", android_jar,
         "--manifest", stage / "AndroidManifest.xml",
         "-R", stage / "res.zip",
         "--min-sdk-version", MIN_SDK,
         "--target-sdk-version", TARGET_SDK,
         "--auto-add-overlay"])

    # 4. put classes.dex in the archive
    with zipfile.ZipFile(raw_apk, "a", zipfile.ZIP_DEFLATED) as z:
        z.write(stage / "dex" / "classes.dex", "classes.dex")

    # 5. zipalign
    aligned = stage / "base-aligned.apk"
    run([zipalign, "-p", "-f", "4", raw_apk, aligned])

    # 6. keystore, kept next to the sources so updates keep the same signature
    repo_ks = HERE / KS_NAME
    stage_ks = stage / KS_NAME
    if repo_ks.exists():
        shutil.copy2(repo_ks, stage_ks)
    else:
        log("generating signing key")
        run([keytool, "-genkeypair", "-keystore", stage_ks, "-storetype", "PKCS12",
             "-alias", KS_ALIAS, "-keyalg", "RSA", "-keysize", "2048",
             "-validity", "10950", "-storepass", KS_PASS, "-keypass", KS_PASS,
             "-dname", "CN=Readeck Share, O=ovohui, C=CN"])

    # 7. sign (v1 + v2 + v3)
    signed = stage / "readeck-share.apk"
    run([java, "-jar", apksigner_jar, "sign", "--ks", stage_ks,
         "--ks-key-alias", KS_ALIAS,
         "--ks-pass", f"pass:{KS_PASS}", "--key-pass", f"pass:{KS_PASS}",
         "--out", signed, aligned])
    run([java, "-jar", apksigner_jar, "verify", "--print-certs", signed],
        stdout=subprocess.DEVNULL)

    out_dir = HERE / "out"
    out_dir.mkdir(exist_ok=True)
    final = out_dir / "readeck-share.apk"
    shutil.copy2(signed, final)
    if not repo_ks.exists():
        shutil.copy2(stage_ks, repo_ks)

    log(f"OK  {final}  {final.stat().st_size / 1024:.0f} KB")
    shutil.rmtree(stage, ignore_errors=True)


if __name__ == "__main__":
    main()
