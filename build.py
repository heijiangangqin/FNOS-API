#!/usr/bin/env python3
"""构建 FPK：编译 Go 静态二进制 -> 组装 app.tgz -> 写入 manifest 校验和 -> 打包 .fpk"""
import hashlib
import io
import os
import shutil
import subprocess
import sys
import tarfile

ROOT = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(ROOT, "src")
PKG = os.path.join(ROOT, "pkg")
BUILD = os.path.join(ROOT, "_build")
STAGE = os.path.join(BUILD, "stage")
DIST = os.path.join(ROOT, "dist")
APPNAME = "fnos-api-gw"
VERSION = "1.5.3"
CLI_BIN = os.environ.get(
    "TRIM_CLI",
    os.path.expanduser("~/.workbuddy/skills/fnos/bin/trim-cli-linux-x64"),
)  # 可用环境变量 TRIM_CLI 指定 trim-cli 路径
FIXED_MTIME = 1750000000
ICONS = os.path.join(ROOT, "icons")


def sh(cmd, cwd=None):
    print("+", " ".join(cmd))
    subprocess.run(cmd, cwd=cwd, check=True)


def add(tar, path, arcname, mode, text=False):
    """text=True 时强制把 CRLF 转成 LF。

    Windows 上编辑过的脚本会带 \r\n，装到 Linux 上会报
    "$'\r': command not found"，安装回调直接失败。这里统一兜底，
    避免以后在 Windows 上改一次脚本就踩一次。
    """
    ti = tar.gettarinfo(path, arcname)
    ti.mode = mode
    ti.uid = ti.gid = 0
    ti.uname = ti.gname = "root"
    ti.mtime = FIXED_MTIME
    with open(path, "rb") as f:
        data = f.read()
    if text and b"\r\n" in data:
        data = data.replace(b"\r\n", b"\n")
        ti.size = len(data)
    tar.addfile(ti, io.BytesIO(data))


def main():
    for d in (BUILD, DIST):
        if os.path.isdir(d):
            shutil.rmtree(d)
        os.makedirs(d)

    # 1) 编译 Linux 静态二进制
    print("== 编译 ==")
    env = dict(os.environ, CGO_ENABLED="0", GOOS="linux", GOARCH="amd64")
    subprocess.run(
        ["go", "build", "-trimpath", "-ldflags", "-s -w", "-o",
         os.path.join(BUILD, "fnos-api-gw"), "."],
        cwd=SRC, env=env, check=True,
    )
    size = os.path.getsize(os.path.join(BUILD, "fnos-api-gw"))
    print(f"   主程序 {size/1024/1024:.2f} MB")

    if not os.path.isfile(CLI_BIN):
        sys.exit(f"找不到 trim-cli: {CLI_BIN}")

    # 2) app.tgz
    print("== 组装 app.tgz ==")
    appdir = os.path.join(STAGE, "app")
    os.makedirs(os.path.join(appdir, "backend"))
    os.makedirs(os.path.join(appdir, "ui", "images"))

    shutil.copy2(os.path.join(BUILD, "fnos-api-gw"), os.path.join(appdir, "backend", "fnos-api-gw"))
    shutil.copy2(CLI_BIN, os.path.join(appdir, "backend", "trim-cli"))
    shutil.copy2(os.path.join(ICONS, "icon_64.png"),
                 os.path.join(appdir, "ui", "images", "icon_64.png"))
    shutil.copy2(os.path.join(ICONS, "icon_256.png"),
                 os.path.join(appdir, "ui", "images", "icon_256.png"))

    uiconfig = os.path.join(appdir, "ui", "config")
    with open(uiconfig, "w", encoding="utf-8") as f:
        f.write('''{
    ".url": {
        "%s.main": {
            "title": "FNOS API 网关",
            "icon": "images/icon_{0}.png",
            "type": "iframe",
            "protocol": "http",
            "port": "9932",
            "url": "/",
            "allUsers": false
        }
    }
}
''' % APPNAME)

    app_tgz = os.path.join(STAGE, "app.tgz")
    with tarfile.open(app_tgz, "w:gz") as tar:
        add(tar, os.path.join(appdir, "backend", "fnos-api-gw"), "backend/fnos-api-gw", 0o755)
        add(tar, os.path.join(appdir, "backend", "trim-cli"), "backend/trim-cli", 0o755)
        add(tar, uiconfig, "ui/config", 0o644, text=True)
        add(tar, os.path.join(appdir, "ui", "images", "icon_64.png"), "ui/images/icon_64.png", 0o644)
        add(tar, os.path.join(appdir, "ui", "images", "icon_256.png"), "ui/images/icon_256.png", 0o644)

    with open(app_tgz, "rb") as f:
        checksum = hashlib.md5(f.read()).hexdigest()
    print(f"   app.tgz {os.path.getsize(app_tgz)/1024/1024:.2f} MB, md5={checksum}")

    # 3) manifest
    print("== 写入 manifest ==")
    manifest = f"""appname               = {APPNAME}
version               = {VERSION}
display_name          = FNOS API 网关
desc                  = 用 HTTP 接口操作飞牛：查看系统状态、管理文件与用户、控制应用和 Docker，不必登录 SSH。安装后打开即用，访问令牌自动生成、可随时修改。
arch                  = x86_64
platform              = all
source                = thirdparty
maintainer            = jianji
maintainer_url        = 
distributor           = jianji
distributor_url       = 
os_min_version        = 1.2.0800
desktop_uidir         = ui
desktop_applaunchname = {APPNAME}.main
service_port          = 9932
checkport             = true
ctl_stop              = true
install_type          = 
install_dep_apps      = 
changelog             = {VERSION}：修复经飞牛桌面打开时接口路径缺少前缀导致仍提示输入令牌；安装一律重置令牌进入首次设置；描述改为纯文本
checksum              = {checksum}
"""
    with open(os.path.join(STAGE, "manifest"), "w", encoding="utf-8", newline="\n") as f:
        f.write(manifest)

    # 4) 打包 .fpk
    print("== 打包 .fpk ==")
    out = os.path.join(DIST, f"{APPNAME}-{VERSION}.fpk")
    with tarfile.open(out, "w:gz") as tar:
        add(tar, os.path.join(STAGE, "manifest"), "manifest", 0o644, text=True)
        add(tar, os.path.join(ICONS, "icon_512.png"), "ICON.PNG", 0o644)
        add(tar, os.path.join(ICONS, "icon_256.png"), "ICON_256.PNG", 0o644)
        for name in sorted(os.listdir(os.path.join(PKG, "config"))):
            add(tar, os.path.join(PKG, "config", name), f"config/{name}", 0o644, text=True)
        for name in sorted(os.listdir(os.path.join(PKG, "cmd"))):
            add(tar, os.path.join(PKG, "cmd", name), f"cmd/{name}", 0o755, text=True)
        wdir = os.path.join(PKG, "wizard")
        if os.path.isdir(wdir):  # 允许没有向导（安装时不问任何东西）
            for name in sorted(os.listdir(wdir)):
                add(tar, os.path.join(wdir, name), f"wizard/{name}", 0o644, text=True)
        add(tar, app_tgz, "app.tgz", 0o644)

    print(f"\n产出: {out}  ({os.path.getsize(out)/1024/1024:.2f} MB)")


if __name__ == "__main__":
    main()
