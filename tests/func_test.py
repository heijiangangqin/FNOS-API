import json, os, shutil, subprocess, time, urllib.request, urllib.error

BASE = "http://127.0.0.1:9932"
TOKEN = open("/tmp/gw_token.txt", encoding="utf-8").read().strip()
R = {"items": []}
SUDO = lambda *a: subprocess.run(["sudo", "-S"] + [str(x) for x in a], input=(os.environ.get("NAS_PASS","") + "\n").encode(),
                                 stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def call(method, path, body=None, timeout=60):
    url = BASE + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method,
                                 headers={"Authorization": "Bearer " + TOKEN,
                                          "Content-Type": "application/json"})
    t0 = time.time()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            raw = r.read().decode("utf-8", "replace"); code = r.status
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace"); code = e.code
    except Exception as e:
        raw = str(e); code = 0
    try: d = json.loads(raw)
    except Exception: d = {"raw": raw[:200]}
    return code, d, int((time.time() - t0) * 1000)

def rec(module, name, ok, detail, code=None, ms=None):
    R["items"].append({"module": module, "name": name, "ok": bool(ok),
                       "detail": detail, "code": code, "ms": ms})

def expect(module, name, method, path, body, want=200, check=None):
    code, d, ms = call(method, path, body)
    ok = code == want and (d.get("success") is (want == 200))
    detail = ""
    if ok and check:
        try:
            ok = bool(check(d))
            if not ok: detail = "断言未通过"
        except Exception as e:
            ok = False; detail = f"断言异常 {e}"
    if not ok:
        detail = detail or str(d.get("errmsg") or d.get("error") or d)[:90]
    rec(module, name, ok, detail, code, ms)
    return d

W = "/vol1/1000/_final"
SUDO("rm", "-rf", W); SUDO("mkdir", "-p", W + "/src", W + "/dst"); SUDO("chown", "-R", "weiwei:Users", "/vol1/1000/")
open("/tmp/f1.txt", "w").write("alpha"); open("/tmp/f2.txt", "w").write("beta")
shutil.copy("/tmp/f1.txt", W + "/src/"); shutil.copy("/tmp/f2.txt", W + "/src/")
# fnOS 该卷默认 ACL 会把新文件建成 000，这里补权限，否则复制/移动读不到源文件
SUDO("chmod", "-R", "a+rwX", W)

# ---------- 用户 ----------
expect("用户", "创建用户", "POST", "/api/user/add",
       {"user": "ftest", "password": "Ft#2026pass", "comment": "最终验证"})
expect("用户", "查询确认", "GET", "/api/user/info?user=ftest", None,
       check=lambda d: ((d.get("data") or {}).get("userInfo") or {}).get("uid") is not None)
expect("用户", "改名", "POST", "/api/user/mod", {"user": "ftest", "new_name": "ftest2", "comment": "已改名"})
expect("用户", "旧名已失效(404)", "GET", "/api/user/info?user=ftest", None, want=404)
expect("用户", "设为管理员", "POST", "/api/user/set-admin", {"user": "ftest2", "on": True})
expect("用户", "管理员开 SSH", "POST", "/api/user/mod", {"user": "ftest2", "allow_ssh": True})
expect("用户", "改密码", "POST", "/api/user/change-password",
       {"user": "ftest2", "old_password": "Ft#2026pass", "new_password": "Ft#2027pass"})
expect("用户", "创建用户组", "POST", "/api/user/group/add", {"group": "fgrp", "comment": "最终"})
expect("用户", "组改名", "POST", "/api/user/group/mod", {"group": "fgrp", "new_name": "fgrp2"})
expect("用户", "组加成员", "POST", "/api/user/group/add-users", {"group": "fgrp2", "users": ["ftest2"]})
expect("用户", "组内成员可查", "GET", "/api/user/group-users?group=fgrp2", None,
       check=lambda d: any(g.get("group") == "fgrp2" for g in (d.get("data", {}).get("groups") or [])))
expect("用户", "组移成员", "POST", "/api/user/group/del-users", {"group": "fgrp2", "users": ["ftest2"]})
expect("用户", "删除用户组", "POST", "/api/user/group/del", {"group": "fgrp2"})
expect("用户", "删除用户", "POST", "/api/user/del", {"user": "ftest2"})

# ---------- 文件 ----------
expect("文件", "建目录", "POST", "/api/file/mkdir", {"path": W + "/work"})
expect("文件", "上传文件", "POST", "/api/file/upload", {"remote_dir": W + "/dst", "local_file": W + "/src/f1.txt"},
       check=lambda d: os.path.exists(W + "/dst/f1.txt"))
expect("文件", "上传预检", "POST", "/api/file/check-upload",
       {"path": W + "/dst/new.txt", "size": 2048, "overwrite": True})
expect("文件", "复制到目录", "POST", "/api/file/cp", {"src": W + "/src/f1.txt", "dest_dir": W + "/work"},
       check=lambda d: os.path.exists(W + "/work/f1.txt"))
expect("文件", "移动到目录(别名 dst)", "POST", "/api/file/mv", {"src": W + "/work/f1.txt", "dst": W + "/dst"},
       check=lambda d: os.path.exists(W + "/dst/f1.txt"))
expect("文件", "改名(别名 path/new_name)", "POST", "/api/file/rename", {"path": W + "/dst/f1.txt", "new_name": "renamed.txt"},
       check=lambda d: os.path.exists(W + "/dst/renamed.txt"))
expect("文件", "改属主", "POST", "/api/file/chown", {"path": W + "/dst/renamed.txt", "uid": 1001})
expect("文件", "压缩", "POST", "/api/file/compress",
       {"output": W + "/all.zip", "sources": [W + "/src"]},
       check=lambda d: os.path.exists(W + "/all.zip"))
expect("文件", "解压", "POST", "/api/file/extract",
       {"archive": W + "/all.zip", "output_dir": W + "/out"},
       check=lambda d: os.path.exists(W + "/out"))
expect("文件", "收藏添加", "POST", "/api/file/fav/add", {"path": W + "/src"})
expect("文件", "收藏删除", "POST", "/api/file/fav/del", {"path": W + "/src"})
expect("文件", "下载链接", "GET", "/api/file/download-url?path=" + W + "/src/f1.txt", None)
expect("文件", "删除文件", "POST", "/api/file/rm", {"path": W + "/dst/renamed.txt"})

expect("文件", "复制到不存在目录被拒", "POST", "/api/file/cp",
       {"src": W + "/src/f1.txt", "dest_dir": W + "/__nodir__"}, want=400)

# ---------- 下载 ----------
D = W + "/dl"; SUDO("mkdir", "-p", D); SUDO("chown", "-R", "weiwei:Users", W)
subprocess.run(["head", "-c", "200000", "/dev/urandom"], stdout=open(D + "/s.bin", "wb"))
SUDO("chmod", "-R", "a+rwX", D)
srv = subprocess.Popen(["python3", "-m", "http.server", "8899"], cwd=D,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
time.sleep(1.5)
expect("下载", "添加任务", "POST", "/api/download/add-uri",
       {"uri": "http://127.0.0.1:8899/s.bin", "saveDir": D},
       check=lambda d: (d.get("data") or {}).get("ids", [-1])[0] not in (-1, None))
time.sleep(3)
code, d, _ = call("GET", "/api/download/ls")
tasks = (d.get("data") or {}).get("tasks") or []
tid = tasks[0]["id"] if tasks else None
rec("下载", "任务列表可见", bool(tid), "" if tid else "无任务", code)
if tid:
    expect("下载", "删除任务", "POST", "/api/download/rm", {"ids": [str(tid)]})
srv.terminate()

# ---------- 日志 ----------
expect("日志", "清空缺 confirm 被拒", "POST", "/api/logger/clear", {"level": 0, "module": 1}, want=400)
expect("日志", "清空带 confirm", "POST", "/api/logger/clear", {"level": 0, "module": 1, "confirm": True})

# ---------- 安全语义 ----------
code, d, _ = call("GET", "/api/logs")
rec("安全", "请求日志可读", code == 200 and d.get("success"), "", code)
req = urllib.request.Request(BASE + "/api/user/list", headers={"Authorization": "Bearer wrong"})
try:
    urllib.request.urlopen(req, timeout=10); bad = 200
except urllib.error.HTTPError as e: bad = e.code
rec("安全", "错误令牌被拒 401", bad == 401, "" if bad == 401 else f"实际 {bad}", bad)

# 清理
SUDO("rm", "-rf", W)
print(json.dumps(R, ensure_ascii=False))
