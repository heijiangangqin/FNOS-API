#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""fnos-api-gw 全量接口测试（v1.1.13）

分层：
  A. 只读 GET  —— 真实调用
  B. 写接口参数校验 —— 空 body，不产生副作用
  C. 高危接口 —— 仅无令牌探测（401 证明路由已注册且逻辑未执行）
  D. 安全/语义 —— 鉴权矩阵、JSON 404、策略拦截、错误码分级
  E. 写入链路 —— 真实建/复制/移动/改名/删除 + 共享生命周期（自建临时目录，用完即删）
"""
import json
import time
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:9932"
TOKEN = open("/tmp/gw_token.txt", encoding="utf-8").read().strip()

# 环境限制：这些接口在本机/本固件上失败属于预期，不算缺陷
ENV_LIMIT = {
    "/api/system/boot-on-power": "官方硬件专属 errno 100060031",
    "/api/system/trim-disk": "官方硬件专属",
    "/api/system/power-on-seconds": "官方硬件专属",
    "/api/system/reserved-partition": "官方机型专属 100060001",
    "/api/system/trim-feature": "上游代理失败（官方机型特性）",
    "/api/monitor/battery": "台式机无电池 100030002",
    "/api/monitor/proc-info": "固件未开放 100030001",
    "/api/baidu/status": "百度网盘插件未安装 502",
    "/api/baidu/quota": "百度网盘插件未安装 502",
    "/api/baidu/ls": "百度网盘插件未安装 502",
    "/api/baidu/task/list": "百度网盘插件未安装 502",
    "/api/download/info": "上游 task.getInfo 失败",
    "/api/download/files": "上游代理失败",
    "/api/docker/compose/ls": "本机未安装 Docker",
    "/api/photos/folders": "相册服务未初始化",
    "/api/photos/search": "相册服务未初始化",
    "/api/app/status": "查询的应用在本机不存在",
}

GET_EPS = [
    # 系统
    "/api/system/info", "/api/system/trim-version", "/api/system/kernel-version",
    "/api/system/host-name", "/api/system/machine-id", "/api/system/uptime",
    "/api/system/all-vols", "/api/system/vols", "/api/system/docker",
    "/api/system/unix-time", "/api/system/boot-on-power", "/api/system/hardware",
    "/api/system/reserved-partition", "/api/system/is-trim-machine",
    "/api/system/trim-feature", "/api/system/fan-mode", "/api/system/trim-disk",
    "/api/system/time-setting", "/api/system/timezone-list", "/api/system/power-plan",
    "/api/system/next-power-off-time", "/api/system/power-plan-status",
    "/api/system/power-on-seconds", "/api/system/inited-timestamp",
    # 监控
    "/api/monitor/cpu", "/api/monitor/memory", "/api/monitor/gen?item=cpuBusy,memPercent",
    "/api/monitor/net", "/api/monitor/disk", "/api/monitor/gpu", "/api/monitor/npu",
    "/api/monitor/proc-list", "/api/monitor/proc-srv", "/api/monitor/sys-warn",
    "/api/monitor/battery", "/api/monitor/sys-fan", "/api/monitor/beep-supported",
    "/api/monitor/proc-info?pids=1",
    # 存储
    "/api/storage/overview", "/api/storage/pools", "/api/storage/disks",
    "/api/storage/free-disks", "/api/storage/removable", "/api/storage/space",
    "/api/storage/health?disk=sda", "/api/storage/smart?disk=sda",
    "/api/storage/disk-info?disk=sda",
    # 文件
    "/api/file/ls?path=/vol1", "/api/file/ls-dir?path=/vol1", "/api/file/usage?vol=1",
    "/api/file/search?key=test&paths=/vol1/1000", "/api/file/search-others?key=test",
    "/api/file/prop?path=/vol1", "/api/file/size?path=/vol1",
    "/api/file/access?path=/vol1", "/api/file/is-mount-point?path=/vol1",
    "/api/file/acl/get?path=/vol1", "/api/file/share/list",
    "/api/file/share/list-others", "/api/file/share/admin-list",
    "/api/file/share/admin-list-others", "/api/file/trash/list", "/api/file/fav/list",
    "/api/file/recent/list", "/api/file/app-dir-list", "/api/file/sys-part-info",
    # 应用 / 下载 / Docker
    "/api/app/list", "/api/app/check-update", "/api/app/status?name=fnchat",
    "/api/download/ls", "/api/download/stat", "/api/download/info?id=1", "/api/download/files?id=1",
    "/api/docker/stats", "/api/docker/image/ls", "/api/docker/container/ls", "/api/docker/compose/ls",
    # 用户 / 日志 / 相册 / 百度
    "/api/user/list", "/api/user/info?user=weiwei", "/api/user/group-list",
    "/api/user/group-users", "/api/user/frozen-list", "/api/user/list-login-device",
    "/api/user/list-ug",
    "/api/logger/modules", "/api/logger/list?page=1&page_size=5", "/api/logger/archive/query",
    "/api/photos/folders", "/api/photos/search?key=a",
    "/api/baidu/status", "/api/baidu/quota", "/api/baidu/ls", "/api/baidu/task/list?transfer=download",
    # 其它
    "/api/auth/status", "/api/auth/diagnose", "/api/auth/manual",
    "/api/config", "/api/setup", "/api/status",
]

POST_VALIDATE = [
    ("/api/app/start", "name"), ("/api/app/stop", "name"),
    ("/api/app/restart", "name"), ("/api/app/uninstall", "name"),
    ("/api/storage/mount", "uuid"), ("/api/storage/umount", "uuid"),
    ("/api/storage/stop", "uuid"), ("/api/storage/extend", "uuid"),
    ("/api/storage/eject", "uuid"), ("/api/storage/format", "uuid"),
    ("/api/file/mkdir", "path"), ("/api/file/rm", "path"),
    ("/api/file/cp", "src"), ("/api/file/mv", "src"), ("/api/file/rename", "src"),
    ("/api/file/share/add", "path"), ("/api/file/share/del", "path"),
    ("/api/download/add-uri", "uri"), ("/api/download/pause", "ids"),
    ("/api/download/resume", "ids"), ("/api/download/retry", "ids"),
    ("/api/download/rm", "ids"),
    ("/api/docker/image/pull", "ref"),
    ("/api/docker/container/start", "id"), ("/api/docker/container/stop", "id"),
    ("/api/docker/container/restart", "id"), ("/api/docker/container/kill", "id"),
    ("/api/user/del", "user"), ("/api/user/set-admin", "user"),
    ("/api/logger/export", "level"),
]

POST_VALIDATE_140 = [
    ("/api/file/compress", "output"), ("/api/file/extract", "archive"),
    ("/api/file/chown", "path"), ("/api/file/acl/set", "path"),
    ("/api/file/trash/restore", "path"), ("/api/file/trash/clear", "confirm"),
    ("/api/file/fav/add", "path"), ("/api/file/fav/del", "path"),
    ("/api/file/team/trash-restore", "paths"), ("/api/file/team/trash-clear", "vol"),
    ("/api/file/cancel", "task_id"), ("/api/file/check-upload", "path"),
    ("/api/file/trimacl/conv", "vol"),
    ("/api/storage/disk-mount", "disk"), ("/api/storage/disk-umount", "disk"),
    ("/api/storage/create", "level"), ("/api/storage/add-disk", "uuid"),
    ("/api/storage/remove-disk", "uuid"), ("/api/storage/replace-disk", "uuid"),
    ("/api/storage/resize", "uuid"),
    ("/api/docker/image/rm", "ref"), ("/api/docker/container/create", "image"),
    ("/api/docker/container/rm", "id"),
    ("/api/logger/clear", "level"),
    ("/api/logger/debuglog/start", "output_dir"), ("/api/logger/debuglog/stop", "task_id"),
    ("/api/download/add-path", "path"),
]

GET_EPS_140 = [
    "/api/file/trash/search?key=__none__", "/api/file/team/list",
]

POST_EXISTS_ONLY = [
    "/api/power/reboot", "/api/power/poweroff", "/api/network/ssh",
    "/api/auth/logout", "/api/auth/start", "/api/auth/code",
]


def call(method, path, token=True, body=None, timeout=45):
    headers = {}
    if token is True:
        headers["Authorization"] = "Bearer " + TOKEN
    elif isinstance(token, str):
        headers["Authorization"] = "Bearer " + token
    data = None
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    t0 = time.time()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            code, raw = r.status, r.read()
    except urllib.error.HTTPError as e:
        code, raw = e.code, e.read()
    except Exception as e:
        return -1, {"error": str(e)}, int((time.time() - t0) * 1000)
    ms = int((time.time() - t0) * 1000)
    try:
        return code, json.loads(raw), ms
    except Exception:
        return code, {"raw": raw[:200].decode("utf-8", "replace")}, ms


def brief(d, n=70):
    if not isinstance(d, dict):
        return str(d)[:n]
    if d.get("success") is False:
        return (d.get("error") or d.get("errmsg") or "")[:n]
    data = d.get("data")
    if data is None:
        return "data=null"
    if isinstance(data, list):
        return "数组 %d 项" % len(data)
    if isinstance(data, dict):
        return "对象 %s" % ",".join(list(data.keys())[:4])
    return str(data)[:n]


R = {"get": [], "post_validate": [], "exists": [], "special": [], "write": []}

_DISK = None


def disk_name():
    """取本机真实的第一块磁盘名（VM 里是 vda/vdb，不是 sda）"""
    global _DISK
    if _DISK:
        return _DISK
    try:
        _, d, _ = call("GET", "/api/storage/disks")
        _DISK = d["data"]["disk"][0]["name"]
    except Exception:
        _DISK = "sda"
    return _DISK

# ---------- A. 只读 ----------
for ep in GET_EPS + GET_EPS_140:
    ep = ep.replace("disk=sda", "disk=" + disk_name())
    code, d, ms = call("GET", ep)
    key = ep.split("?")[0]
    env = ENV_LIMIT.get(key)
    ok = (code == 200 and d.get("success") is not False)
    R["get"].append({
        "ep": ep, "code": code, "ok": ok, "ms": ms, "brief": brief(d),
        "env_limit": bool(env) and not ok, "env_reason": env or "",
    })

# ---------- B. 写接口参数校验 ----------
for ep, field in POST_VALIDATE + POST_VALIDATE_140:
    code, d, ms = call("POST", ep, body={})
    txt = json.dumps(d, ensure_ascii=False)
    # 空 body 时必须 400；错误文案里应能看出缺什么（confirm 类会提示高风险）
    ok = code == 400 and d.get("success") is False
    R["post_validate"].append({
        "ep": ep, "code": code, "ok": ok, "ms": ms, "brief": brief(d), "field": field,
    })

# ---------- C. 高危接口存在性 ----------
for ep in POST_EXISTS_ONLY:
    code, d, ms = call("POST", ep, token=False, body={})
    R["exists"].append({"ep": ep, "code": code, "ok": code == 401, "ms": ms})


def sp(name, method, path, token=True, body=None, want=200, note=""):
    code, d, ms = call(method, path, token=token, body=body)
    R["special"].append({
        "name": name, "code": code, "want": want, "ok": code == want,
        "ms": ms, "brief": brief(d), "note": note,
    })


# ---------- D. 安全 / 语义 ----------
sp("健康检查免鉴权", "GET", "/api/health", token=False, want=200)
sp("无令牌访问 → 401", "GET", "/api/config", token=False, want=401)
sp("错误令牌 → 401", "GET", "/api/config", token="wrong-token-123", want=401)
sp("查询串令牌可用", "GET", "/api/config?token=" + TOKEN, token=False, want=200)
sp("未知路由 → JSON 404", "GET", "/api/not-exist-xyz", want=404)
sp("POST 到未知路由 → 404", "POST", "/api/not-exist-xyz", body={}, want=404)
sp("重复首次设置 → 409", "POST", "/api/setup", body={"token": "x" * 12}, want=409)
sp("cmd 拦截 media → 403", "POST", "/api/cmd", body={"args": ["media", "list"]}, want=403)
sp("cmd 拦截 login → 403", "POST", "/api/cmd", body={"args": ["login"]}, want=403)
sp("cmd 拦截改连接参数(前) → 403", "POST", "/api/cmd",
   body={"args": ["--host", "1.2.3.4", "system", "info"]}, want=403)
sp("cmd 拦截 --profile(子命令后) → 403", "POST", "/api/cmd",
   body={"args": ["system", "info", "--profile=x"]}, want=403)
sp("cmd 参数为字符串 → 400", "POST", "/api/cmd", body={"args": "system"}, want=400)
sp("cmd 参数含数字 → 400", "POST", "/api/cmd", body={"args": ["system", 1]}, want=400)
sp("cmd 空数组 → 400", "POST", "/api/cmd", body={"args": []}, want=400)
sp("cmd 只读命令 → 200", "POST", "/api/cmd", body={"args": ["monitor", "cpu"]}, want=200)
sp("file/ls 不存在 → 404", "GET", "/api/file/ls?path=/vol1/1000/nope-xyz", want=404)
sp("file/size 不存在 → 404", "GET", "/api/file/size?path=/vol1/1000/nope-xyz", want=404)
sp("file/prop 不存在 → 404", "GET", "/api/file/prop?path=/vol1/1000/nope-xyz", want=404)
sp("file/rm 不存在 → 404", "POST", "/api/file/rm", body={"path": "/vol1/1000/nope-xyz"}, want=404)
sp("share/del 非共享 → 404", "POST", "/api/file/share/del",
   body={"path": "/vol1/1000/nope-xyz"}, want=404)
sp("storage/health 假磁盘 → 404", "GET", "/api/storage/health?disk=zzz", want=404)
sp("storage/smart 假磁盘 → 404", "GET", "/api/storage/smart?disk=zzz", want=404)
sp("storage/disk-info 假磁盘 → 404", "GET", "/api/storage/disk-info?disk=zzz", want=404)
sp("user/info 传 uid → 400", "GET", "/api/user/info?user=1000", want=400)
sp("download/pause 不存在 id → 404", "POST", "/api/download/pause",
   body={"ids": [999999]}, want=404)
sp("download/pause 数字 ids 被接受", "POST", "/api/download/pause",
   body={"ids": ["999999"]}, want=404, note="数字/字符串等价，不存在的仍是 404")
sp("download/add-uri 非法 URI → 400", "POST", "/api/download/add-uri",
   body={"uri": "not-a-valid-uri", "saveDir": "/vol1/1000"}, want=400)
sp("storage/umount 密码错 → 403", "POST", "/api/storage/umount",
   body={"uuid": "00000000-0000-0000-0000-000000000000", "password": "wrong"}, want=403)

# ---------- E. 真实写入链路 ----------
TAG = time.strftime("%H%M%S")
ROOT = "/vol1/1000/_gw_t_" + TAG


def w(name, method, path, body=None, want=200):
    code, d, ms = call(method, path, body=body)
    R["write"].append({
        "name": name, "code": code, "want": want, "ok": code == want,
        "ms": ms, "brief": brief(d),
    })
    return code, d


w("mkdir 根目录", "POST", "/api/file/mkdir", {"path": ROOT})
w("mkdir 嵌套(自动建父目录)", "POST", "/api/file/mkdir", {"path": ROOT + "/a/b"})
w("mkdir 目标目录 dst", "POST", "/api/file/mkdir", {"path": ROOT + "/dst"})
w("cp 目录 → dst", "POST", "/api/file/cp", {"src": ROOT + "/a", "dest": ROOT + "/dst"})
w("mv 目录 → dst", "POST", "/api/file/mv", {"src": ROOT + "/a", "dest": ROOT + "/dst"})
w("rename 根目录", "POST", "/api/file/rename", {"src": ROOT, "name": "_gw_t_" + TAG + "_r"})
NEW = "/vol1/1000/_gw_t_" + TAG + "_r"
code, d, _ = call("GET", "/api/file/ls?path=" + NEW)
R["write"].append({"name": "回查改名后的目录", "code": code,
                   "want": 200, "ok": code == 200 and isinstance(d.get("data"), list),
                   "ms": 0, "brief": brief(d)})
w("rm 递归删除", "POST", "/api/file/rm", {"path": NEW})

# 共享生命周期（先确保目录存在，避免上游瞬时故障导致后续步骤全错）
SH = "/vol1/1000/_gw_share_" + TAG


def ensure_dir(p, tries=3):
    """建目录并确认成功，最多重试 3 次"""
    for i in range(tries):
        code, d, _ = call("POST", "/api/file/mkdir", body={"path": p})
        if code == 200:
            R["write"].append({"name": "mkdir " + p.split("/")[-1], "code": code, "want": 200,
                               "ok": True, "ms": 0, "brief": brief(d)})
            return True
        time.sleep(0.5)
    R["write"].append({"name": "mkdir " + p.split("/")[-1], "code": code, "want": 200,
                       "ok": False, "ms": 0, "brief": brief(d)})
    return False


if ensure_dir(SH):
    sp("share/info 已存在但未共享 → 404", "GET", "/api/file/share/info?path=" + SH, want=404)
    w("share/add 合法 permset", "POST", "/api/file/share/add",
      {"path": SH, "name": "gw-" + TAG, "permset": [{"owner": 32719, "inherit": 7}]})
    sp("share/info 创建后 → 200", "GET", "/api/file/share/info?path=" + SH, want=200)
    w("share/del", "POST", "/api/file/share/del", {"path": SH})
    sp("share/info 删除后 → 404", "GET", "/api/file/share/info?path=" + SH, want=404)
w("rm 临时共享目录", "POST", "/api/file/rm", {"path": SH})

# 残留检查
for p in (ROOT, NEW, SH):
    code, d, _ = call("GET", "/api/file/prop?path=" + p)
    R["write"].append({"name": "残留检查 " + p.split("/")[-1], "code": code,
                       "want": 404, "ok": code == 404, "ms": 0, "brief": brief(d)})

print(json.dumps(R, ensure_ascii=False))
