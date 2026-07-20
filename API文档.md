# fnOS API 接口文档

> **Base URL**: `http://{NAS_IP}:9932`  
> **认证方式**: 除登录接口外，所有请求需在 Header 中携带 `Authorization: Bearer <token>`  
> **默认 Token**: `fnosapi2024`（安装时在向导中设置）  
> **响应格式**: JSON，统一包裹为 `{ "success": true/false, "data": ... }`  
> **Swagger 文档**: `http://{NAS_IP}:9932/api-docs`

---

## 目录

- [1. 认证](#1-认证)
- [2. 快捷方式](#2-快捷方式)
- [3. 系统与监控](#3-系统与监控)
- [4. 文件管理](#4-文件管理)
- [5. 应用中心](#5-应用中心)
- [6. 下载任务](#6-下载任务)
- [7. 日志中心](#7-日志中心)
- [8. 用户管理](#8-用户管理)
- [9. 存储管理](#9-存储管理)
- [10. Docker 管理](#10-docker-管理)
- [11. 配置与健康检查](#11-配置与健康检查)

---

## 通用说明

### 请求头

| Header | 必填 | 说明 |
|--------|------|------|
| `Authorization` | 是（登录除外） | `Bearer <api_token>` |
| `Content-Type` | POST 请求时 | `application/json` |

### 统一响应格式

**成功响应：**
```json
{
  "success": true,
  "data": { ... }
}
```

**失败响应：**
```json
{
  "success": false,
  "error": "Error message",
  "errmsg": "NAS 端错误描述",
  "errno": 65534
}
```

**HTTP 状态码：**

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 401 | 未认证（Token 缺失或错误） |
| 500 | CLI 执行错误 |
| 504 | CLI 执行超时 |

### 路径约定

- NAS 文件路径使用 `/vol{v}/...` 格式，如 `/vol1/downloads`、`/vol3/1106/media`
- 存储池标识使用 uuid 或 `trim_*` 形式
- 磁盘标识使用纯设备名，如 `sda`、`nvme0n1`

---

## 1. 认证

### POST /api/auth/login

登录 NAS 并保存 session。

**无需 Authorization Header。**

**请求体：**
```json
{
  "username": "xiaoqian",
  "password": "your_password"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | 是 | NAS 用户名 |
| password | string | 是 | NAS 密码 |

**响应示例：**
```json
{
  "success": true,
  "data": {
    "admin": true,
    "uid": 1000,
    "username": "xiaoqian",
    "token_persisted": true,
    "long_token_persisted": true,
    "secret_persisted": true,
    "back_id_persisted": true,
    "endpoint": "user.login"
  }
}
```

---

### POST /api/auth/logout

登出 NAS 并清除本地 session。

**响应示例：**
```json
{
  "success": true,
  "data": { "message": "Logged out" }
}
```

---

## 2. 快捷方式

### GET /api/status

聚合系统信息、CPU、内存和存储总览，输出单个 JSON 对象。

**请求示例：**
```
GET /api/status
Authorization: Bearer fnosapi2024
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "operation": "Status",
    "plan": {
      "cpu": { "core": 4, "name": "Intel(R) Core(TM) i9-14900KF", "thread": 4 },
      "memory": { "mem": { "total": 4294967296, "used": 2620702720, "free": 216629248 } },
      "storage": { "result": "succ" },
      "system": { "model": "10 152 128" }
    }
  }
}
```

---

### GET /api/ls

列出目录，快捷入口（等同 `file ls`）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 否 | 目录路径，如 `/vol1/downloads`；不传则列出当前用户目录 |

**请求示例：**
```
GET /api/ls?path=/vol1/downloads
```

---

### GET /api/search

搜索文件，快捷入口（等同 `file search`）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 搜索关键字 |
| paths | string | 否 | 逗号分隔的搜索路径，如 `/vol1/1000,/vol2/1000` |

**请求示例：**
```
GET /api/search?key=report&paths=/vol1/1000,/vol2/1000
```

---

## 3. 系统与监控

### GET /api/system/info

查看 NAS 机器类型和系统信息。

**响应示例：**
```json
{
  "success": true,
  "data": {
    "model": "10 152 128",
    "type": "",
    "version": "0.8.x"
  }
}
```

---

### GET /api/monitor/cpu

查看 CPU 使用率。

---

### GET /api/monitor/memory

查看内存使用率。

---

## 4. 文件管理

### GET /api/file/ls

列出文件目录。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 否 | 目录路径，如 `/vol1/downloads` |

**请求示例：**
```
GET /api/file/ls?path=/vol1/1000
```

---

### GET /api/file/search

搜索文件。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 搜索关键字 |
| paths | string | 否 | 逗号分隔的搜索路径 |

---

### GET /api/file/search-others

搜索"其他用户共享给当前用户"的文件。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | 搜索关键字 |

**请求示例：**
```
GET /api/file/search-others?key=.txt
```

---

### GET /api/file/acl

查看指定路径的 ACL。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 文件路径 |
| prop | string | 否 | 设为 `true` 额外带出文件属性 |

**请求示例：**
```
GET /api/file/acl?path=/vol1/1000/docs&prop=true
```

---

### GET /api/file/share/info

查看指定路径的共享目录信息。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 共享目录路径 |

---

### GET /api/file/share/list

列出当前用户可见的共享目录。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uid | integer | 否 | 按源用户 uid 过滤 |

---

### GET /api/file/share/list-others

列出有哪些用户向当前用户共享了目录。

---

### GET /api/file/share/admin-list

以管理员视角列出共享目录。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uid | integer | 否 | 按 owner uid 过滤 |

---

### GET /api/file/share/admin-list-others

以管理员视角列出共享目录的 owner。

---

### POST /api/file/share/add

将路径创建为共享目录。

**请求体：**
```json
{
  "path": "/vol1/1000/share",
  "shareName": "team-share",
  "permset": "[{\"id\":1000,\"perm\":7}]",
  "sub": false,
  "aclMode": 2
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 共享目录路径 |
| shareName | string | 是 | 共享名称 |
| permset | string | 是 | 权限设置 JSON 字符串，建议复用 `file/acl` 返回值 |
| sub | boolean | 否 | 是否应用于子目录 |
| aclMode | integer | 否 | ACL 模式 |

---

### POST /api/file/share/del

移除指定路径的共享目录状态。

**请求体：**
```json
{
  "path": "/vol1/1000/share",
  "sub": false,
  "aclMode": 2
}
```

---

### POST /api/file/mkdir

创建目录。

**请求体：**
```json
{
  "path": "/vol2/1106/new-dir"
}
```

---

### POST /api/file/rm

删除文件或目录。

**请求体：**
```json
{
  "path": "/vol2/1106/old-dir"
}
```

---

### POST /api/file/cp

复制文件到目标目录。

**请求体：**
```json
{
  "src": "/vol1/a.txt",
  "destDir": "/vol2/backup"
}
```

---

### POST /api/file/mv

移动文件到目标目录。

**请求体：**
```json
{
  "src": "/vol1/a.txt",
  "destDir": "/vol2/archive"
}
```

---

### POST /api/file/check-upload

上传前检查目标路径是否可用。

**请求体：**
```json
{
  "path": "/vol2/1106/new.jpg",
  "size": 12345,
  "overwrite": "rename"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 是 | 完整目标文件路径 |
| size | integer | 是 | 文件大小（字节） |
| overwrite | string | 否 | `rename` 或 `overwrite` |

---

## 5. 应用中心

### GET /api/app/list

列出已安装应用。

**响应示例：**
```json
{
  "success": true,
  "data": [
    { "name": "trim.alist", "version": "3.0.13", "status": "running" }
  ]
}
```

---

### GET /api/app/status

查看单个已安装应用状态。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 应用名，如 `trim.alist` |

**请求示例：**
```
GET /api/app/status?name=trim.alist
```

---

### POST /api/app/install

下载并安装应用中心指定版本。

**请求体：**
```json
{
  "name": "trim.alist",
  "version": "3.0.13",
  "sourceId": 265,
  "volumeId": 2
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 应用名 |
| version | string | 否 | 版本号 |
| sourceId | integer | 否 | 应用中心 sourceID |
| volumeId | integer | 是 | 安装卷号 |

---

### POST /api/app/install-fpk

上传并安装本地 `.fpk` 文件。

**请求体：**
```json
{
  "localFile": "/vol1/@appdata/demo.fpk",
  "volumeId": 2,
  "dryRun": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| localFile | string | 是 | fpk 文件路径（NAS 上的路径） |
| volumeId | integer | 是 | 安装卷号 |
| dryRun | boolean | 否 | `true` 只检查不安装 |

---

### POST /api/app/update

更新应用到指定版本。

**请求体：**
```json
{
  "name": "trim.alist",
  "version": "3.0.14",
  "volumeId": 2
}
```

---

### POST /api/app/start

启动应用。

**请求体：**
```json
{ "name": "trim.alist" }
```

---

### POST /api/app/stop

停止应用。

**请求体：**
```json
{ "name": "trim.alist" }
```

---

### POST /api/app/uninstall

卸载应用。

**请求体：**
```json
{ "name": "trim.alist" }
```

---

## 6. 下载任务

### GET /api/download/ls

列出下载任务。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| keyword | string | 否 | 按关键字搜索 |

---

### GET /api/download/info

查看单个下载任务详情。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务 ID |

**请求示例：**
```
GET /api/download/info?id=42
```

---

### GET /api/download/files

查看下载任务的文件列表（BT 类任务）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | integer | 是 | 任务 ID |

---

### POST /api/download/add-uri

从 URI 创建下载任务（HTTP/HTTPS/FTP/SFTP/磁力链接）。

**请求体：**
```json
{
  "uri": "https://example.com/file.zip",
  "saveDir": "/vol1/downloads"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uri | string | 是 | 下载地址或磁力链接 |
| saveDir | string | 是 | 保存目录，必须为 `/vol{v}/...` 格式 |

---

### POST /api/download/add-path

从 NAS 上已有的 `.torrent` 文件创建下载任务。

**请求体：**
```json
{
  "path": "/vol1/torrents/demo.torrent",
  "saveDir": "/vol1/downloads"
}
```

---

### POST /api/download/pause

暂停下载任务。

**请求体：**
```json
{ "ids": [42, 43] }
```

---

### POST /api/download/resume

恢复暂停的下载任务。

**请求体：**
```json
{ "ids": [42, 43] }
```

---

### POST /api/download/retry

重试失败的下载任务。

**请求体：**
```json
{ "ids": [42] }
```

---

### POST /api/download/rm

删除下载任务（不删除已下载文件）。

**请求体：**
```json
{ "ids": [42, 43] }
```

---

### GET /api/download/stat

查看下载中心汇总统计。

---

## 7. 日志中心

### GET /api/logger/list

按页列出日志记录。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | integer | 否 | 页码 |
| pageSize | integer | 否 | 每页条数 |
| level | integer | 否 | 日志级别 |
| module | integer | 否 | 日志模块 ID |
| locale | string | 否 | 语言 |

**请求示例：**
```
GET /api/logger/list?page=1&pageSize=20&level=3
```

---

### GET /api/logger/modules

列出可用日志模块。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| locale | string | 否 | 语言 |

---

### POST /api/logger/clear

清除日志。

**请求体：**
```json
{
  "level": 3,
  "module": 1
}
```

---

### POST /api/logger/export

导出日志。

**请求体：**
```json
{
  "level": 3,
  "module": 1,
  "locale": "zh-CN"
}
```

---

### GET /api/logger/archive/query

查看当前日志归档配置。

---

### POST /api/logger/archive/set

设置日志归档策略。

**请求体：**
```json
{
  "switchVal": 1,
  "filePath": "/vol1/@appdata/logs",
  "sizeGt": 100,
  "dateUnit": 2,
  "dateBefore": 3
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| switchVal | integer | 是 | `0` 关闭归档，`1` 开启 |
| filePath | string | 是 | 归档路径，关闭时可传空字符串 |
| sizeGt | integer | 否 | 大小阈值 |
| dateUnit | integer | 否 | `0` 天 `1` 周 `2` 月 `3` 年 |
| dateBefore | integer | 否 | 时间阈值 |

---

## 8. 用户管理

### GET /api/user/list

列出用户。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uver | integer | 否 | 缓存版本号 |

**请求示例：**
```
GET /api/user/list
Authorization: Bearer fnosapi2024
```

**响应示例：**
```json
{
  "success": true,
  "data": {
    "uver": 116940298453079,
    "users": [
      { "uid": 1000, "user": "xiaoqian", "admin": true, "allowSSH": true },
      { "uid": 1049, "user": "xlt225", "comment": "广东_xlt 使用" }
    ]
  }
}
```

---

### GET /api/user/frozen-list

列出被冻结的用户。

---

### GET /api/user/info

查看用户信息。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user | string | 否 | 用户名，不传则查当前用户 |

**请求示例：**
```
GET /api/user/info?user=alice
```

---

### GET /api/user/list-ug

列出用户/用户组名称到 ID 的映射。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| users | boolean | 否 | 只返回用户映射 |
| groups | boolean | 否 | 只返回用户组映射 |
| uver | integer | 否 | 缓存版本号 |

---

### GET /api/user/group-list

列出用户组。

---

### GET /api/user/group-info

查看用户组详情。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| group | string | 是 | 用户组名 |

**请求示例：**
```
GET /api/user/group-info?group=Administrators
```

---

### GET /api/user/group-users

列出各组及其成员用户 ID。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| uver | integer | 否 | 缓存版本号 |

---

### GET /api/user/list-login-device

列出当前用户的登录设备。

---

### POST /api/user/add

新增用户。

**请求体：**
```json
{
  "user": "newuser",
  "password": "P@ssw0rd",
  "groups": ["Users", "Developers"],
  "comment": "开发_新用户",
  "email": "new@example.com",
  "mobile": "13800138000",
  "disableChangePassword": false,
  "setAdmin": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user | string | 是 | 用户名 |
| password | string | 否 | 密码 |
| groups | string[] | 否 | 所属用户组 |
| comment | string | 否 | 备注 |
| email | string | 否 | 邮箱 |
| mobile | string | 否 | 手机号 |
| disableChangePassword | boolean | 否 | 禁止修改密码 |
| setAdmin | boolean | 否 | 设为管理员 |

---

### POST /api/user/mod

修改用户。

**请求体：**
```json
{
  "user": "alice",
  "newName": "alice2",
  "password": "NewP@ss",
  "groups": ["Users"],
  "comment": "修改备注",
  "email": "alice2@example.com",
  "mobile": "",
  "enableChangePassword": true,
  "disableUser": 0,
  "allowSsh": true
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user | string | 是 | 要修改的用户名 |
| newName | string | 否 | 新用户名 |
| password | string | 否 | 新密码 |
| groups | string[] | 否 | 新的用户组列表 |
| comment | string | 否 | 备注 |
| email | string | 否 | 邮箱 |
| mobile | string | 否 | 手机号 |
| disableChangePassword | boolean | 否 | 禁止修改密码 |
| enableChangePassword | boolean | 否 | 允许修改密码 |
| disableUser | integer | 否 | 禁用用户 |
| allowSsh | boolean | 否 | 允许 SSH（仅管理员） |
| disallowSsh | boolean | 否 | 禁止 SSH |

---

### POST /api/user/del

删除用户。

**请求体：**
```json
{ "user": "alice" }
```

---

### POST /api/user/set-admin

授予或撤销管理员权限。

**请求体：**
```json
{
  "user": "alice",
  "on": true
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user | string | 是 | 用户名 |
| on | boolean | 是 | `true` 授予，`false` 撤销 |

---

### POST /api/user/unfreeze

解冻用户。

**请求体：**
```json
{ "user": "alice" }
```

---

### POST /api/user/change-password

修改密码。

**请求体：**
```json
{
  "user": "alice",
  "oldPassword": "OldPass",
  "newPassword": "NewPass",
  "removeToken": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user | string | 否 | 用户名，不传则修改当前用户 |
| oldPassword | string | 否 | 旧密码 |
| newPassword | string | 否 | 新密码 |
| removeToken | boolean | 否 | 是否移除 token |

---

### POST /api/user/group-add

新增用户组。

**请求体：**
```json
{
  "group": "Developers",
  "comment": "开发组"
}
```

---

### POST /api/user/group-mod

修改用户组。

**请求体：**
```json
{
  "group": "Developers",
  "newName": "Devs",
  "comment": "开发团队"
}
```

---

### POST /api/user/group-del

删除用户组。

**请求体：**
```json
{ "group": "Developers" }
```

---

### POST /api/user/group-set-users

整体替换用户组成员。

**请求体：**
```json
{
  "group": "Developers",
  "users": ["alice", "bob", "charlie"]
}
```

---

### POST /api/user/group-add-users

向用户组增加成员。

**请求体：**
```json
{
  "group": "Developers",
  "users": ["alice", "bob"]
}
```

---

### POST /api/user/group-del-users

从用户组移除成员。

**请求体：**
```json
{
  "group": "Developers",
  "users": ["charlie"]
}
```

---

## 9. 存储管理

> ⚠️ **高风险操作提醒**：存储写操作（挂载/卸载/创建/停止/格式化等）可能导致数据丢失，操作前建议先执行只读命令确认目标状态。

### GET /api/storage/overview

存储总览信息。

---

### GET /api/storage/pools

列出存储池。

---

### GET /api/storage/disks

列出磁盘。

---

### GET /api/storage/removable

列出可移动存储设备。

---

### GET /api/storage/space

查看聚合存储空间信息。

---

### GET /api/storage/health

查看指定磁盘健康信息。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| disk | string | 是 | 纯设备名，如 `sda` |

**请求示例：**
```
GET /api/storage/health?disk=sda
```

---

### GET /api/storage/smart

查看指定磁盘 SMART 信息。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| disk | string | 是 | 纯设备名，如 `sda` |

---

### POST /api/storage/mount

挂载存储池。

**请求体：**
```json
{ "uuid": "trim_pool" }
```

---

### POST /api/storage/umount

卸载存储池（受保护操作需密码）。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "password": "your_password"
}
```

---

### POST /api/storage/create

创建存储池。

**请求体：**
```json
{
  "level": 5,
  "disks": ["sda", "sdb"],
  "fstype": "btrfs",
  "comment": "数据盘",
  "checkDisk": true,
  "password": "your_password"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| level | integer | 是 | RAID 级别：`0` `1` `4` `5` `6` `10` |
| disks | string[] | 是 | 磁盘列表 |
| fstype | string | 否 | 文件系统类型 |
| comment | string | 否 | 备注 |
| checkDisk | boolean | 否 | 检查磁盘 |
| password | string | 否 | 操作密码 |

---

### POST /api/storage/stop

停止并删除存储池。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "password": "your_password"
}
```

---

### POST /api/storage/add-disk

向存储池添加磁盘。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "disks": ["sdc", "sdd"],
  "password": "your_password"
}
```

---

### POST /api/storage/remove-disk

从存储池移除磁盘。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "disks": ["sdc"],
  "password": "your_password"
}
```

---

### POST /api/storage/replace-disk

替换存储池中的磁盘。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "disks": ["sdc"],
  "newDisks": ["sdd"],
  "password": "your_password"
}
```

---

### POST /api/storage/extend

扩展存储池。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "password": "your_password"
}
```

---

### POST /api/storage/resize

调整存储池。

**请求体：**
```json
{
  "uuid": "trim_pool",
  "disks": ["sdc"],
  "vdName": "vd1",
  "level": 6,
  "password": "your_password"
}
```

---

### POST /api/storage/format

格式化磁盘或分区。

**请求体：**
```json
{
  "disk": "sdz",
  "fstype": "ext4",
  "partition": 1,
  "password": "your_password"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| disk | string | 是 | 磁盘设备名 |
| fstype | string | 是 | 文件系统类型 |
| partition | integer | 否 | 分区号，默认 `0` |
| password | string | 否 | 操作密码 |

---

### POST /api/storage/eject

弹出可移动磁盘。

**请求体：**
```json
{
  "disk": "sdz",
  "password": "your_password"
}
```

---

## 10. Docker 管理

### GET /api/docker/stats

Docker 聚合统计信息。

---

### GET /api/docker/image/ls

列出 Docker 镜像。

---

### POST /api/docker/image/pull

拉取 Docker 镜像（长耗时操作）。

**请求体：**
```json
{ "imageRef": "nginx:latest" }
```

---

### GET /api/docker/image/inspect

查看镜像详情。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| imageRef | string | 是 | 镜像引用，如 `nginx:latest` |

---

### POST /api/docker/image/rm

删除 Docker 镜像。

**请求体：**
```json
{
  "imageRef": "nginx:latest",
  "force": false
}
```

---

### GET /api/docker/container/ls

列出 Docker 容器。

---

### GET /api/docker/container/inspect

查看容器详情。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 容器 ID 或名称 |

---

### GET /api/docker/container/top

查看容器进程列表。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 容器 ID |

---

### GET /api/docker/container/stats

查看容器资源使用统计。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 容器 ID |

---

### POST /api/docker/container/start

启动容器。

**请求体：**
```json
{ "id": "container_name_or_id" }
```

---

### POST /api/docker/container/stop

停止容器（长耗时操作）。

**请求体：**
```json
{ "id": "container_name_or_id" }
```

---

### POST /api/docker/container/restart

重启容器（长耗时操作）。

**请求体：**
```json
{ "id": "container_name_or_id" }
```

---

### POST /api/docker/container/kill

强杀容器。

**请求体：**
```json
{ "id": "container_name_or_id" }
```

---

### POST /api/docker/container/rm

删除容器。

**请求体：**
```json
{
  "id": "container_name_or_id",
  "force": false
}
```

---

### POST /api/docker/container/create

创建容器。

**请求体：**
```json
{
  "image": "nginx:latest",
  "name": "my-nginx",
  "start": true,
  "restart": true,
  "memory": 128,
  "cpu": 512,
  "env": ["NODE_ENV=production", "PORT=3000"],
  "cmd": ["nginx", "-g", "daemon off;"],
  "port": ["8080:80/tcp", "443:443"],
  "mount": ["mydata:/data:rw"]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| image | string | 是 | 镜像引用 |
| name | string | 否 | 容器名称 |
| start | boolean | 否 | 创建后立即启动 |
| restart | boolean | 否 | 自动重启 |
| memory | integer | 否 | 内存限制（MB） |
| cpu | integer | 否 | CPU shares |
| env | string[] | 否 | 环境变量 `KEY=VALUE` |
| cmd | string[] | 否 | 启动命令参数 |
| port | string[] | 否 | 端口映射 `host:container[/proto]` |
| mount | string[] | 否 | 挂载 `source:target[:ro\|rw]` |

---

### GET /api/docker/compose/ls

列出 Docker Compose 项目。

---

## 11. 配置与健康检查

### GET /api/health

健康检查（无需认证）。

**响应示例：**
```json
{
  "status": "ok",
  "version": "1.0.0",
  "cliBin": "/vol1/@appcenter/fnos-api/backend/trim-cli-bin/trim-cli-linux-x64"
}
```

---

### GET /api/config

获取当前配置（脱敏，不返回密码和 token）。

**响应示例：**
```json
{
  "success": true,
  "data": {
    "nas_host": "localhost",
    "nas_port": "5666",
    "nas_scheme": "auto",
    "nas_username": "xiaoqian",
    "allow_insecure_ws": "false",
    "tls_insecure": "false"
  }
}
```

---

## Python 快速入门

```python
import requests

BASE = "http://192.168.2.165:9932"
TOKEN = "fnosapi2024"
HEADERS = {"Authorization": f"Bearer {TOKEN}"}

# 1. 登录
r = requests.post(f"{BASE}/api/auth/login", json={
    "username": "932soft",
    "password": "your_password"
})
print("登录:", r.json()["success"])

# 2. 获取用户列表
r = requests.get(f"{BASE}/api/user/list", headers=HEADERS)
for u in r.json()["data"]["users"]:
    print(f"  {u['uid']:>5} {u['user']}")

# 3. 系统状态
r = requests.get(f"{BASE}/api/status", headers=HEADERS)

# 4. 列出文件
r = requests.get(f"{BASE}/api/file/ls?path=/vol1/downloads", headers=HEADERS)

# 5. Docker 容器列表
r = requests.get(f"{BASE}/api/docker/container/ls", headers=HEADERS)

# 6. 创建下载任务
r = requests.post(f"{BASE}/api/download/add-uri", headers=HEADERS, json={
    "uri": "https://example.com/file.zip",
    "saveDir": "/vol1/downloads"
})
```
