# FNOS-API

将 [trim-cli](https://github.com/nicehash/trim-cli) 命令行工具封装为 RESTful API 服务的飞牛(fnOS)原生应用，支持文件管理、存储管理、Docker 管理、下载任务、日志中心、用户管理、系统监控等全部功能。

## 功能概览

| 模块 | 接口 | 说明 |
|------|------|------|
| 认证 | `/api/auth/login` `/api/auth/logout` | NAS 登录/登出 |
| 状态 | `/api/status` `/api/ls` `/api/search` | 快捷状态查询、文件浏览、搜索 |
| 系统 | `/api/system/info` `/api/monitor/cpu` `/api/monitor/memory` | 系统信息与资源监控 |
| 文件 | `/api/file/*` | 文件浏览、搜索、分享、上传、复制、移动、删除 |
| 应用中心 | `/api/app/*` | 应用列表、安装、更新、启停、卸载 |
| 下载 | `/api/download/*` | 下载任务管理（添加、暂停、恢复、删除、统计） |
| 日志 | `/api/logger/*` | 日志查询、模块列表、清理、导出、归档 |
| 用户 | `/api/user/*` | 用户/用户组 CRUD、权限管理、登录设备 |
| 存储 | `/api/storage/*` | 存储池、磁盘、S.M.A.R.T.、挂载/卸载、扩容 |
| Docker | `/api/docker/*` | 容器与镜像管理、Compose 列表 |

## 安装方式

### 方式一：下载 FPK 安装包

前往 [Releases](https://github.com/heijiangangqin/FNOS-API/releases) 下载最新 `.fpk` 文件，在飞牛 NAS 应用中心手动安装。

### 方式二：自行打包

```bash
git clone https://github.com/heijiangangqin/FNOS-API.git
cd FNOS-API/app/backend
npm install --production
cd ../..
tar -czf fnos-api_1.0.0_x86_64.fpk manifest ICON.PNG ICON_256.PNG app config cmd wizard
```

将生成的 `fnos-api_1.0.0_x86_64.fpk` 上传到飞牛 NAS 安装即可。

## 安装配置

安装向导会要求填写以下信息：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| NAS 地址 | 飞牛 NAS 的 IP 或域名 | `localhost` |
| WebSocket 端口 | NAS WebSocket 端口 | `5666` |
| NAS 用户名 | 登录用户名 | - |
| NAS 密码 | 登录密码 | - |
| API 访问令牌 | 调用 API 时的 Bearer Token | `fnosapi2024` |

## API 调用

所有接口（除登录外）需在请求头携带 Token：

```bash
curl -H "Authorization: Bearer fnosapi2024" http://<NAS-IP>:9932/api/system/info
```

### 示例

```bash
# 登录
curl -X POST http://<NAS-IP>:9932/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your-password"}'

# 获取系统信息
curl -H "Authorization: Bearer fnosapi2024" http://<NAS-IP>:9932/api/system/info

# 获取存储概览
curl -H "Authorization: Bearer fnosapi2024" http://<NAS-IP>:9932/api/storage/overview

# Docker 容器列表
curl -H "Authorization: Bearer fnosapi2024" http://<NAS-IP>:9932/api/docker/container/ls

# 添加下载任务
curl -X POST http://<NAS-IP>:9932/api/download/add-uri \
  -H "Authorization: Bearer fnosapi2024" \
  -H "Content-Type: application/json" \
  -d '{"uri":"https://example.com/file.zip","saveDir":"/vol1/downloads"}'
```

## 项目结构

```
FNOS-API/
├── manifest              # 应用清单（名称、版本、架构等）
├── ICON.PNG              # 应用图标
├── ICON_256.PNG          # 大图标
├── app/
│   ├── backend/          # Node.js 后端
│   │   ├── server.js     # Express 服务主文件
│   │   ├── trimcli.js    # trim-cli 封装模块
│   │   ├── trim-cli-bin/ # 多平台 trim-cli 二进制
│   │   └── package.json
│   ├── frontend/         # 前端欢迎页
│   └── ui/               # 飞牛桌面图标与配置
├── config/
│   ├── privilege         # 运行权限配置
│   └── resource          # 资源配置
├── cmd/                  # 生命周期脚本
│   ├── main              # 服务启停主脚本
│   ├── install_init      # 安装初始化
│   ├── install_callback  # 安装回调（生成配置文件）
│   ├── config_init       # 配置表单定义
│   ├── config_callback   # 配置保存回调
│   ├── upgrade_*         # 升级脚本
│   └── uninstall_*       # 卸载脚本
└── wizard/               # 安装/卸载向导
    ├── install           # 安装向导表单
    └── uninstall         # 卸载确认与数据保留选项
```

## 技术栈

- **后端**: Node.js + Express
- **CLI 封装**: trim-cli（多平台二进制）
- **前端**: 原生 HTML/CSS
- **运行环境**: 飞牛 fnOS 0.9.0+

## 注意事项

- 服务运行端口：`9932`
- trim-cli 二进制支持：linux-x64、linux-arm64、darwin-x64、darwin-arm64、windows-x64
- 安装后服务自动启动，可通过飞牛应用中心管理启停
- 修改配置后需重启服务生效

## License

MIT
