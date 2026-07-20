#!/usr/bin/env python3
"""FNOS-API 接口测试脚本 - 读取 config.ini 统一配置，逐个测试所有 API"""

import configparser
import json
import sys
import requests

def load_config(path="config.ini"):
    cfg = configparser.ConfigParser()
    cfg.read(path, encoding="utf-8")
    return cfg["fnos-api"]

def headers(token):
    return {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

def get(base, token, path, params=None):
    r = requests.get(f"{base}{path}", headers=headers(token), params=params, timeout=30)
    return r.status_code, r.json()

def post(base, token, path, data=None):
    r = requests.post(f"{base}{path}", headers=headers(token), json=data, timeout=60)
    return r.status_code, r.json()

def test(name, result):
    status, body = result
    ok = "OK" if status == 200 and body.get("success") else "FAIL"
    print(f"  [{ok}] {name} (HTTP {status})")
    if ok == "FAIL":
        print(f"       {json.dumps(body, ensure_ascii=False)[:200]}")

def main():
    cfg = load_config()
    base = cfg["base_url"].rstrip("/")
    token = cfg["api_token"]
    username = cfg["nas_username"]
    password = cfg["nas_password"]

    print("=" * 60)
    print("FNOS-API 接口测试")
    print(f"  目标: {base}")
    print("=" * 60)

    # ==================== Auth ====================
    print("\n--- Auth 认证 ---")
    test("登录", post(base, token, "/api/auth/login", {"username": username, "password": password}))
    test("登出", post(base, token, "/api/auth/logout"))

    # ==================== Shortcut ====================
    print("\n--- Shortcut 快捷 ---")
    test("状态查询", get(base, token, "/api/status"))
    test("文件列表", get(base, token, "/api/ls", {"path": "/"}))
    test("搜索", get(base, token, "/api/search", {"key": "test"}))

    # ==================== System & Monitor ====================
    print("\n--- System & Monitor 系统与监控 ---")
    test("系统信息", get(base, token, "/api/system/info"))
    test("CPU 监控", get(base, token, "/api/monitor/cpu"))
    test("内存监控", get(base, token, "/api/monitor/memory"))

    # ==================== File ====================
    print("\n--- File 文件管理 ---")
    test("文件列表", get(base, token, "/api/file/ls", {"path": "/"}))
    test("文件搜索", get(base, token, "/api/file/search", {"key": "test"}))
    test("搜索其他共享", get(base, token, "/api/file/search-others", {"key": "test"}))
    test("文件ACL", get(base, token, "/api/file/acl", {"path": "/", "prop": "true"}))
    test("共享信息", get(base, token, "/api/file/share/info", {"path": "/"}))
    test("共享列表", get(base, token, "/api/file/share/list"))
    test("其他共享列表", get(base, token, "/api/file/share/list-others"))
    test("管理共享列表", get(base, token, "/api/file/share/admin-list"))
    test("其他管理共享", get(base, token, "/api/file/share/admin-list-others"))
    test("添加共享", post(base, token, "/api/file/share/add", {"path": "/vol1/data", "shareName": "test-share", "permset": "rw", "sub": False}))
    test("删除共享", post(base, token, "/api/file/share/del", {"path": "/vol1/data", "sub": False}))
    test("创建目录", post(base, token, "/api/file/mkdir", {"path": "/vol1/test-dir"}))
    test("复制文件", post(base, token, "/api/file/cp", {"src": "/vol1/test-dir/file.txt", "destDir": "/vol1/backup"}))
    test("移动文件", post(base, token, "/api/file/mv", {"src": "/vol1/test-dir/file.txt", "destDir": "/vol1/archive"}))
    test("删除文件", post(base, token, "/api/file/rm", {"path": "/vol1/test-dir"}))
    test("检查上传", post(base, token, "/api/file/check-upload", {"path": "/vol1/test.txt", "size": 1024, "overwrite": False}))

    # ==================== App Center ====================
    print("\n--- App Center 应用中心 ---")
    test("应用列表", get(base, token, "/api/app/list"))
    test("应用状态", get(base, token, "/api/app/status", {"name": "docker"}))
    test("安装应用", post(base, token, "/api/app/install", {"name": "jellyfin", "volumeId": 1}))
    test("安装FPK", post(base, token, "/api/app/install-fpk", {"localFile": "/vol1/app.fpk", "volumeId": 1, "dryRun": False}))
    test("更新应用", post(base, token, "/api/app/update", {"name": "jellyfin"}))
    test("启动应用", post(base, token, "/api/app/start", {"name": "docker"}))
    test("停止应用", post(base, token, "/api/app/stop", {"name": "docker"}))
    test("卸载应用", post(base, token, "/api/app/uninstall", {"name": "jellyfin"}))

    # ==================== Download ====================
    print("\n--- Download 下载管理 ---")
    test("下载列表", get(base, token, "/api/download/ls"))
    test("下载信息", get(base, token, "/api/download/info", {"id": "1"}))
    test("下载文件列表", get(base, token, "/api/download/files", {"id": "1"}))
    test("添加URL下载", post(base, token, "/api/download/add-uri", {"uri": "https://example.com/file.zip", "saveDir": "/vol1/downloads"}))
    test("添加路径下载", post(base, token, "/api/download/add-path", {"path": "/vol1/torrents/test.torrent", "saveDir": "/vol1/downloads"}))
    test("暂停下载", post(base, token, "/api/download/pause", {"ids": [1]}))
    test("恢复下载", post(base, token, "/api/download/resume", {"ids": [1]}))
    test("重试下载", post(base, token, "/api/download/retry", {"ids": [1]}))
    test("删除下载", post(base, token, "/api/download/rm", {"ids": [1]}))
    test("下载统计", get(base, token, "/api/download/stat"))

    # ==================== Logger ====================
    print("\n--- Logger 日志中心 ---")
    test("日志列表", get(base, token, "/api/logger/list", {"page": 1, "pageSize": 10}))
    test("日志模块", get(base, token, "/api/logger/modules"))
    test("清理日志", post(base, token, "/api/logger/clear", {"level": 3, "module": 0}))
    test("导出日志", post(base, token, "/api/logger/export", {"level": 3, "module": 0, "locale": "zh-CN"}))
    test("归档查询", get(base, token, "/api/logger/archive/query"))
    test("归档设置", post(base, token, "/api/logger/archive/set", {"switchVal": 1, "filePath": "/vol1/logs", "sizeGt": 100, "dateUnit": 1, "dateBefore": "2024-01-01"}))

    # ==================== User ====================
    print("\n--- User 用户管理 ---")
    test("用户列表", get(base, token, "/api/user/list"))
    test("冻结用户列表", get(base, token, "/api/user/frozen-list"))
    test("用户信息", get(base, token, "/api/user/info", {"user": "admin"}))
    test("用户/组列表", get(base, token, "/api/user/list-ug", {"users": "true"}))
    test("用户组列表", get(base, token, "/api/user/group-list"))
    test("用户组信息", get(base, token, "/api/user/group-info", {"group": "admin"}))
    test("用户组成员", get(base, token, "/api/user/group-users"))
    test("登录设备列表", get(base, token, "/api/user/list-login-device"))
    test("添加用户", post(base, token, "/api/user/add", {"user": "testuser", "password": "Test@123", "comment": "测试用户", "setAdmin": False}))
    test("修改用户", post(base, token, "/api/user/mod", {"user": "testuser", "comment": "修改后的备注"}))
    test("删除用户", post(base, token, "/api/user/del", {"user": "testuser"}))
    test("设置管理员", post(base, token, "/api/user/set-admin", {"user": "testuser", "on": True}))
    test("解冻用户", post(base, token, "/api/user/unfreeze", {"user": "testuser"}))
    test("修改密码", post(base, token, "/api/user/change-password", {"user": "testuser", "oldPassword": "Test@123", "newPassword": "NewPass@456"}))
    test("添加用户组", post(base, token, "/api/user/group-add", {"group": "testgroup", "comment": "测试组"}))
    test("修改用户组", post(base, token, "/api/user/group-mod", {"group": "testgroup", "comment": "修改后的组备注"}))
    test("删除用户组", post(base, token, "/api/user/group-del", {"group": "testgroup"}))
    test("设置组成员", post(base, token, "/api/user/group-set-users", {"group": "admin", "users": ["admin", "testuser"]}))
    test("添加组成员", post(base, token, "/api/user/group-add-users", {"group": "admin", "users": ["testuser"]}))
    test("删除组成员", post(base, token, "/api/user/group-del-users", {"group": "admin", "users": ["testuser"]}))

    # ==================== Storage ====================
    print("\n--- Storage 存储管理 ---")
    test("存储概览", get(base, token, "/api/storage/overview"))
    test("存储池列表", get(base, token, "/api/storage/pools"))
    test("磁盘列表", get(base, token, "/api/storage/disks"))
    test("可移动磁盘", get(base, token, "/api/storage/removable"))
    test("存储空间", get(base, token, "/api/storage/space"))
    test("磁盘健康", get(base, token, "/api/storage/health", {"disk": "sda"}))
    test("S.M.A.R.T.", get(base, token, "/api/storage/smart", {"disk": "sda"}))
    test("挂载存储", post(base, token, "/api/storage/mount", {"uuid": "test-uuid"}))
    test("卸载存储", post(base, token, "/api/storage/umount", {"uuid": "test-uuid", "password": ""}))
    test("创建存储", post(base, token, "/api/storage/create", {"level": 1, "disks": ["sda", "sdb"], "fstype": "ext4"}))
    test("停止存储", post(base, token, "/api/storage/stop", {"uuid": "test-uuid", "password": ""}))
    test("添加磁盘", post(base, token, "/api/storage/add-disk", {"uuid": "test-uuid", "disks": ["sdc"], "password": ""}))
    test("移除磁盘", post(base, token, "/api/storage/remove-disk", {"uuid": "test-uuid", "disks": ["sdc"], "password": ""}))
    test("替换磁盘", post(base, token, "/api/storage/replace-disk", {"uuid": "test-uuid", "disks": ["sda"], "newDisks": ["sdc"], "password": ""}))
    test("扩容存储", post(base, token, "/api/storage/extend", {"uuid": "test-uuid", "password": ""}))
    test("调整存储", post(base, token, "/api/storage/resize", {"uuid": "test-uuid", "disks": ["sda", "sdb"]}))
    test("格式化磁盘", post(base, token, "/api/storage/format", {"disk": "sdb", "fstype": "ext4", "partition": 1, "password": ""}))
    test("弹出磁盘", post(base, token, "/api/storage/eject", {"disk": "sdb", "password": ""}))

    # ==================== Docker ====================
    print("\n--- Docker 容器管理 ---")
    test("Docker 统计", get(base, token, "/api/docker/stats"))
    test("镜像列表", get(base, token, "/api/docker/image/ls"))
    test("拉取镜像", post(base, token, "/api/docker/image/pull", {"imageRef": "nginx:latest"}))
    test("镜像详情", get(base, token, "/api/docker/image/inspect", {"imageRef": "nginx:latest"}))
    test("删除镜像", post(base, token, "/api/docker/image/rm", {"imageRef": "nginx:latest", "force": False}))
    test("容器列表", get(base, token, "/api/docker/container/ls"))
    test("容器详情", get(base, token, "/api/docker/container/inspect", {"id": "container-id"}))
    test("容器进程", get(base, token, "/api/docker/container/top", {"id": "container-id"}))
    test("容器资源", get(base, token, "/api/docker/container/stats", {"id": "container-id"}))
    test("启动容器", post(base, token, "/api/docker/container/start", {"id": "container-id"}))
    test("停止容器", post(base, token, "/api/docker/container/stop", {"id": "container-id"}))
    test("重启容器", post(base, token, "/api/docker/container/restart", {"id": "container-id"}))
    test("杀死容器", post(base, token, "/api/docker/container/kill", {"id": "container-id"}))
    test("删除容器", post(base, token, "/api/docker/container/rm", {"id": "container-id", "force": False}))
    test("创建容器", post(base, token, "/api/docker/container/create", {
        "image": "nginx:latest",
        "name": "test-nginx",
        "start": True,
        "restart": True,
        "memory": 512,
        "cpu": 1,
        "env": ["TZ=Asia/Shanghai"],
        "port": ["8080:80"],
        "mount": ["/vol1/html:/usr/share/nginx/html"]
    }))
    test("Compose列表", get(base, token, "/api/docker/compose/ls"))

    # ==================== Config ====================
    print("\n--- Config 配置 ---")
    test("获取配置", get(base, token, "/api/config"))

    print("\n" + "=" * 60)
    print("测试完成")

if __name__ == "__main__":
    main()
