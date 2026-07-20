/**
 * FNOS-API Server
 * 将 trim-cli 封装为 RESTful API
 */
const fs = require('fs');
const path = require('path');
const express = require('express');
const cors = require('cors');
const TrimCLI = require('./trimcli');

const app = express();
const PORT = process.env.PORT || 9932;
const CONFIG_PATH = process.env.CONFIG_PATH || '';

// 加载配置
let config = {};
try {
  if (CONFIG_PATH && fs.existsSync(CONFIG_PATH)) {
    config = JSON.parse(fs.readFileSync(CONFIG_PATH, 'utf8'));
  }
} catch (e) {
  console.error('Failed to load config:', e.message);
}

const cli = new TrimCLI(config);
app.use(cors());
app.use(express.json({ limit: '10mb' }));
app.use(express.urlencoded({ extended: true }));

// 静态前端
app.use(express.static(path.join(__dirname, '..', 'frontend')));

// ==================== Auth Middleware ====================
function authMiddleware(req, res, next) {
  const token = config.api_token || 'fnosapi2024';
  const auth = req.headers['authorization'];
  if (!auth || auth !== 'Bearer ' + token) {
    return res.status(401).json({ error: 'Unauthorized', message: '请在 Authorization 头中携带 Bearer <token>' });
  }
  next();
}

// ==================== Helper ====================
function wrap(fn) {
  return async (req, res) => {
    try {
      const result = await fn(req, res);
      if (result !== undefined && !res.headersSent) {
        res.json({ success: true, data: result });
      }
    } catch (e) {
      const status = e.code === 'ETIMEDOUT' || e.killed ? 504 : 500;
      if (!res.headersSent) {
        res.status(status).json({ success: false, error: e.error || e.message, errmsg: e.errmsg, errno: e.errno, stderr: e.stderr });
      }
    }
  };
}

// ==================== Auth API ====================
app.post('/api/auth/login', wrap(async (req) => {
  const { username, password } = req.body;
  if (!username || !password) throw { message: 'username and password required' };
  return cli.login(username, password);
}));

app.post('/api/auth/logout', authMiddleware, wrap(async () => cli.logout()));

// ==================== Shortcut API ====================
app.get('/api/status', authMiddleware, wrap(async () => cli.getStatus()));

app.get('/api/ls', authMiddleware, wrap(async (req) => cli.ls(req.query.path)));

app.get('/api/search', authMiddleware, wrap(async (req) => {
  const paths = req.query.paths ? req.query.paths.split(',') : undefined;
  return cli.search(req.query.key, paths);
}));

// ==================== System & Monitor API ====================
app.get('/api/system/info', authMiddleware, wrap(async () => cli.systemInfo()));

app.get('/api/monitor/cpu', authMiddleware, wrap(async () => cli.monitorCpu()));

app.get('/api/monitor/memory', authMiddleware, wrap(async () => cli.monitorMemory()));

// ==================== File API ====================
app.get('/api/file/ls', authMiddleware, wrap(async (req) => cli.fileLs(req.query.path)));

app.get('/api/file/search', authMiddleware, wrap(async (req) => {
  const paths = req.query.paths ? req.query.paths.split(',') : undefined;
  return cli.fileSearch(req.query.key, paths);
}));

app.get('/api/file/search-others', authMiddleware, wrap(async (req) => cli.fileSearchOthers(req.query.key)));

app.get('/api/file/acl', authMiddleware, wrap(async (req) => cli.fileAclGet(req.query.path, req.query.prop === 'true')));

app.get('/api/file/share/info', authMiddleware, wrap(async (req) => cli.fileShareInfo(req.query.path)));

app.get('/api/file/share/list', authMiddleware, wrap(async (req) => cli.fileShareList(req.query.uid)));

app.get('/api/file/share/list-others', authMiddleware, wrap(async () => cli.fileShareListOthers()));

app.get('/api/file/share/admin-list', authMiddleware, wrap(async (req) => cli.fileShareAdminList(req.query.uid)));

app.get('/api/file/share/admin-list-others', authMiddleware, wrap(async () => cli.fileShareAdminListOthers()));

app.post('/api/file/share/add', authMiddleware, wrap(async (req) => {
  const { path: p, shareName, permset, sub, aclMode } = req.body;
  return cli.fileShareAdd(p, shareName, permset, { sub, aclMode });
}));

app.post('/api/file/share/del', authMiddleware, wrap(async (req) => cli.fileShareDel(req.body.path, { sub: req.body.sub, aclMode: req.body.aclMode })));

app.post('/api/file/mkdir', authMiddleware, wrap(async (req) => cli.fileMkdir(req.body.path)));

app.post('/api/file/rm', authMiddleware, wrap(async (req) => cli.fileRm(req.body.path)));

app.post('/api/file/cp', authMiddleware, wrap(async (req) => cli.fileCp(req.body.src, req.body.destDir)));

app.post('/api/file/mv', authMiddleware, wrap(async (req) => cli.fileMv(req.body.src, req.body.destDir)));

app.post('/api/file/check-upload', authMiddleware, wrap(async (req) => cli.fileCheckUpload(req.body.path, req.body.size, req.body.overwrite)));

// ==================== App Center API ====================
app.get('/api/app/list', authMiddleware, wrap(async () => cli.appList()));

app.get('/api/app/status', authMiddleware, wrap(async (req) => cli.appStatus(req.query.name)));

app.post('/api/app/install', authMiddleware, wrap(async (req) => cli.appInstall(req.body.name, req.body)));

app.post('/api/app/install-fpk', authMiddleware, wrap(async (req) => cli.appInstallFpk(req.body.localFile, req.body.volumeId, req.body.dryRun)));

app.post('/api/app/update', authMiddleware, wrap(async (req) => cli.appUpdate(req.body.name, req.body)));

app.post('/api/app/start', authMiddleware, wrap(async (req) => cli.appStart(req.body.name)));

app.post('/api/app/stop', authMiddleware, wrap(async (req) => cli.appStop(req.body.name)));

app.post('/api/app/uninstall', authMiddleware, wrap(async (req) => cli.appUninstall(req.body.name)));

// ==================== Download API ====================
app.get('/api/download/ls', authMiddleware, wrap(async (req) => cli.downloadLs(req.query.keyword)));

app.get('/api/download/info', authMiddleware, wrap(async (req) => cli.downloadInfo(req.query.id)));

app.get('/api/download/files', authMiddleware, wrap(async (req) => cli.downloadFiles(req.query.id)));

app.post('/api/download/add-uri', authMiddleware, wrap(async (req) => cli.downloadAddUri(req.body.uri, req.body.saveDir)));

app.post('/api/download/add-path', authMiddleware, wrap(async (req) => cli.downloadAddPath(req.body.path, req.body.saveDir)));

app.post('/api/download/pause', authMiddleware, wrap(async (req) => cli.downloadPause(...req.body.ids)));

app.post('/api/download/resume', authMiddleware, wrap(async (req) => cli.downloadResume(...req.body.ids)));

app.post('/api/download/retry', authMiddleware, wrap(async (req) => cli.downloadRetry(...req.body.ids)));

app.post('/api/download/rm', authMiddleware, wrap(async (req) => cli.downloadRm(...req.body.ids)));

app.get('/api/download/stat', authMiddleware, wrap(async () => cli.downloadStat()));

// ==================== Logger API ====================
app.get('/api/logger/list', authMiddleware, wrap(async (req) => cli.loggerList(req.query)));

app.get('/api/logger/modules', authMiddleware, wrap(async (req) => cli.loggerModules(req.query.locale)));

app.post('/api/logger/clear', authMiddleware, wrap(async (req) => cli.loggerClear(req.body.level, req.body.module)));

app.post('/api/logger/export', authMiddleware, wrap(async (req) => {
  const result = await cli.loggerExport(req.body.level, req.body.module, req.body.locale);
  return result;
}));

app.get('/api/logger/archive/query', authMiddleware, wrap(async () => cli.loggerArchiveQuery()));

app.post('/api/logger/archive/set', authMiddleware, wrap(async (req) => cli.loggerArchiveSet(req.body)));

// ==================== User API ====================
app.get('/api/user/list', authMiddleware, wrap(async (req) => cli.userList(req.query.uver)));

app.get('/api/user/frozen-list', authMiddleware, wrap(async () => cli.userFrozenList()));

app.get('/api/user/info', authMiddleware, wrap(async (req) => cli.userInfo(req.query.user)));

app.get('/api/user/list-ug', authMiddleware, wrap(async (req) => cli.userListUg(req.query)));

app.get('/api/user/group-list', authMiddleware, wrap(async () => cli.userGroupList()));

app.get('/api/user/group-info', authMiddleware, wrap(async (req) => cli.userGroupInfo(req.query.group)));

app.get('/api/user/group-users', authMiddleware, wrap(async (req) => cli.userGroupUsers(req.query.uver)));

app.get('/api/user/list-login-device', authMiddleware, wrap(async () => cli.userListLoginDevice()));

app.post('/api/user/add', authMiddleware, wrap(async (req) => cli.userAdd(req.body.user, req.body)));

app.post('/api/user/mod', authMiddleware, wrap(async (req) => cli.userMod(req.body.user, req.body)));

app.post('/api/user/del', authMiddleware, wrap(async (req) => cli.userDel(req.body.user)));

app.post('/api/user/set-admin', authMiddleware, wrap(async (req) => cli.userSetAdmin(req.body.user, req.body.on)));

app.post('/api/user/unfreeze', authMiddleware, wrap(async (req) => cli.userUnfreeze(req.body.user)));

app.post('/api/user/change-password', authMiddleware, wrap(async (req) => cli.userChangePassword(req.body.user, req.body)));

app.post('/api/user/group-add', authMiddleware, wrap(async (req) => cli.userGroupAdd(req.body.group, req.body.comment)));

app.post('/api/user/group-mod', authMiddleware, wrap(async (req) => cli.userGroupMod(req.body.group, req.body)));

app.post('/api/user/group-del', authMiddleware, wrap(async (req) => cli.userGroupDel(req.body.group)));

app.post('/api/user/group-set-users', authMiddleware, wrap(async (req) => cli.userGroupSetUsers(req.body.group, req.body.users)));

app.post('/api/user/group-add-users', authMiddleware, wrap(async (req) => cli.userGroupAddUsers(req.body.group, req.body.users)));

app.post('/api/user/group-del-users', authMiddleware, wrap(async (req) => cli.userGroupDelUsers(req.body.group, req.body.users)));

// ==================== Storage API ====================
app.get('/api/storage/overview', authMiddleware, wrap(async () => cli.storageOverview()));

app.get('/api/storage/pools', authMiddleware, wrap(async () => cli.storagePools()));

app.get('/api/storage/disks', authMiddleware, wrap(async () => cli.storageDisks()));

app.get('/api/storage/removable', authMiddleware, wrap(async () => cli.storageRemovable()));

app.get('/api/storage/space', authMiddleware, wrap(async () => cli.storageSpace()));

app.get('/api/storage/health', authMiddleware, wrap(async (req) => cli.storageHealth(req.query.disk)));

app.get('/api/storage/smart', authMiddleware, wrap(async (req) => cli.storageSmart(req.query.disk)));

app.post('/api/storage/mount', authMiddleware, wrap(async (req) => cli.storageMount(req.body.uuid)));

app.post('/api/storage/umount', authMiddleware, wrap(async (req) => cli.storageUmount(req.body.uuid, req.body.password)));

app.post('/api/storage/create', authMiddleware, wrap(async (req) => cli.storageCreate(req.body)));

app.post('/api/storage/stop', authMiddleware, wrap(async (req) => cli.storageStop(req.body.uuid, req.body.password)));

app.post('/api/storage/add-disk', authMiddleware, wrap(async (req) => cli.storageAddDisk(req.body.uuid, req.body.disks, req.body.password)));

app.post('/api/storage/remove-disk', authMiddleware, wrap(async (req) => cli.storageRemoveDisk(req.body.uuid, req.body.disks, req.body.password)));

app.post('/api/storage/replace-disk', authMiddleware, wrap(async (req) => cli.storageReplaceDisk(req.body.uuid, req.body.disks, req.body.newDisks, req.body.password)));

app.post('/api/storage/extend', authMiddleware, wrap(async (req) => cli.storageExtend(req.body.uuid, req.body.password)));

app.post('/api/storage/resize', authMiddleware, wrap(async (req) => cli.storageResize(req.body.uuid, req.body)));

app.post('/api/storage/format', authMiddleware, wrap(async (req) => cli.storageFormat(req.body.disk, req.body.fstype, req.body.partition, req.body.password)));

app.post('/api/storage/eject', authMiddleware, wrap(async (req) => cli.storageEject(req.body.disk, req.body.password)));

// ==================== Docker API ====================
app.get('/api/docker/stats', authMiddleware, wrap(async () => cli.dockerStats()));

app.get('/api/docker/image/ls', authMiddleware, wrap(async () => cli.dockerImageLs()));

app.post('/api/docker/image/pull', authMiddleware, wrap(async (req) => cli.dockerImagePull(req.body.imageRef)));

app.get('/api/docker/image/inspect', authMiddleware, wrap(async (req) => cli.dockerImageInspect(req.query.imageRef)));

app.post('/api/docker/image/rm', authMiddleware, wrap(async (req) => cli.dockerImageRm(req.body.imageRef, req.body.force)));

app.get('/api/docker/container/ls', authMiddleware, wrap(async () => cli.dockerContainerLs()));

app.get('/api/docker/container/inspect', authMiddleware, wrap(async (req) => cli.dockerContainerInspect(req.query.id)));

app.get('/api/docker/container/top', authMiddleware, wrap(async (req) => cli.dockerContainerTop(req.query.id)));

app.get('/api/docker/container/stats', authMiddleware, wrap(async (req) => cli.dockerContainerStats(req.query.id)));

app.post('/api/docker/container/start', authMiddleware, wrap(async (req) => cli.dockerContainerStart(req.body.id)));

app.post('/api/docker/container/stop', authMiddleware, wrap(async (req) => cli.dockerContainerStop(req.body.id)));

app.post('/api/docker/container/restart', authMiddleware, wrap(async (req) => cli.dockerContainerRestart(req.body.id)));

app.post('/api/docker/container/kill', authMiddleware, wrap(async (req) => cli.dockerContainerKill(req.body.id)));

app.post('/api/docker/container/rm', authMiddleware, wrap(async (req) => cli.dockerContainerRm(req.body.id, req.body.force)));

app.post('/api/docker/container/create', authMiddleware, wrap(async (req) => cli.dockerContainerCreate(req.body)));

app.get('/api/docker/compose/ls', authMiddleware, wrap(async () => cli.dockerComposeLs()));

// ==================== Config API ====================
app.get('/api/config', authMiddleware, wrap(async () => {
  const safe = { ...config };
  delete safe.nas_password;
  delete safe.api_token;
  return safe;
}));

// ==================== Auto Login ====================
async function autoLogin() {
  const username = config.nas_username;
  const password = config.nas_password;
  if (username && password) {
    try {
      await cli.login(username, password);
      console.log('[FNOS-API] Auto login success: ' + username);
    } catch (e) {
      console.error('[FNOS-API] Auto login failed: ' + (e.error || e.message));
    }
  } else {
    console.log('[FNOS-API] No credentials configured, skip auto login');
  }
}

// ==================== Start ====================
app.listen(PORT, '0.0.0.0', () => {
  console.log('[FNOS-API] Server running on port ' + PORT);
  console.log('[FNOS-API] trim-cli binary: ' + cli.binPath);
  console.log('[FNOS-API] NAS target: ' + cli.host + ':' + (cli.port || 'auto'));
  autoLogin();
});
