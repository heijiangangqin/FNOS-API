/**
 * trim-cli 封装模块
 * 将 trim-cli 命令行调用封装为异步函数
 */
const { execFile } = require('child_process');

class TrimCLI {
  constructor(config = {}) {
    this.binPath = process.env.TRIM_CLI_BIN || 'trim-cli';
    this.host = config.nas_host || 'localhost';
    this.port = config.nas_port || '';
    this.scheme = config.nas_scheme || 'auto';
    this.allowInsecureWs = config.allow_insecure_ws === 'true';
    this.tlsInsecure = config.tls_insecure === 'true';
    this.timeout = 60000;
    this.longTimeout = 300000;
  }

  async exec(args, options = {}) {
    const baseArgs = this._buildBaseArgs();
    const fullArgs = [...baseArgs, ...args];
    const timeout = options.timeout || this.timeout;
    return new Promise((resolve, reject) => {
      execFile(this.binPath, fullArgs, {
        timeout,
        maxBuffer: 10 * 1024 * 1024,
        env: { ...process.env }
      }, (error, stdout, stderr) => {
        if (error) {
          const errObj = { success: false, error: error.message, stderr: stderr.trim(), code: error.code, killed: error.killed || false };
          try { const parsed = JSON.parse(stderr); if (parsed.errmsg) { errObj.errmsg = parsed.errmsg; errObj.errno = parsed.errno; } } catch (e) {}
          reject(errObj);
          return;
        }
        if (options.rawOutput) { resolve(stdout); return; }
        const trimmed = stdout.trim();
        if (!trimmed) { resolve({ success: true, data: null }); return; }
        try { resolve(JSON.parse(trimmed)); } catch (e) { resolve({ success: true, message: trimmed }); }
      });
    });
  }

  _buildBaseArgs() {
    const args = ['--host', this.host];
    if (this.port) args.push('--port', String(this.port));
    if (this.scheme && this.scheme !== 'auto') args.push('--scheme', this.scheme);
    if (this.allowInsecureWs) args.push('--allow-insecure-ws');
    if (this.tlsInsecure) args.push('--tls-insecure');
    return args;
  }

  // ==================== Auth ====================
  async login(username, password) { return this.exec(['login', '-u', username, '-p', password]); }
  async logout() { return this.exec(['logout']); }

  // ==================== Shortcuts ====================
  async getStatus() { return this.exec(['+status']); }
  async ls(p) { const a = ['+ls']; if (p) a.push(p); return this.exec(a); }
  async search(key, paths) { const a = ['+search', key]; if (paths && paths.length) a.push(...paths); return this.exec(a); }

  // ==================== System & Monitor ====================
  async systemInfo() { return this.exec(['system', 'info']); }
  async monitorCpu() { return this.exec(['monitor', 'cpu']); }
  async monitorMemory() { return this.exec(['monitor', 'memory']); }

  // ==================== File ====================
  async fileLs(p) { const a = ['file','ls']; if (p) a.push(p); return this.exec(a); }
  async fileSearch(key, paths) { const a = ['file','search',key]; if (paths && paths.length) a.push(...paths); return this.exec(a); }
  async fileSearchOthers(key) { return this.exec(['file','search-others',key]); }
  async fileAclGet(p, withProp) { const a = ['file','acl','get',p]; if (withProp) a.push('--prop'); return this.exec(a); }
  async fileShareInfo(p) { return this.exec(['file','share','info',p]); }
  async fileShareList(uid) { const a = ['file','share','list']; if (uid) a.push(String(uid)); return this.exec(a); }
  async fileShareListOthers() { return this.exec(['file','share','list-others']); }
  async fileShareAdminList(uid) { const a = ['file','share','admin-list']; if (uid) a.push(String(uid)); return this.exec(a); }
  async fileShareAdminListOthers() { return this.exec(['file','share','admin-list-others']); }
  async fileShareAdd(p, name, permset, opts = {}) {
    const a = ['file','share','add',p,name,'--permset',permset];
    if (opts.sub) a.push('--sub'); if (opts.aclMode) a.push('--acl-mode',String(opts.aclMode));
    return this.exec(a, { timeout: this.longTimeout });
  }
  async fileShareDel(p, opts = {}) {
    const a = ['file','share','del',p];
    if (opts.sub) a.push('--sub'); if (opts.aclMode) a.push('--acl-mode',String(opts.aclMode));
    return this.exec(a, { timeout: this.longTimeout });
  }
  async fileMkdir(p) { return this.exec(['file','mkdir',p]); }
  async fileCheckUpload(p, size, overwrite) { const a = ['file','check-upload',p,String(size)]; if (overwrite) a.push('--overwrite',overwrite); return this.exec(a); }
  async fileUpload(remoteDir, localFile, overwrite) { const a = ['file','upload',remoteDir,localFile]; if (overwrite) a.push('--overwrite',overwrite); return this.exec(a, { timeout: this.longTimeout }); }
  async fileRm(p) { return this.exec(['file','rm',p]); }
  async fileCp(src, dest) { return this.exec(['file','cp',src,dest]); }
  async fileMv(src, dest) { return this.exec(['file','mv',src,dest]); }

  // ==================== App Center ====================
  async appList() { return this.exec(['app','list']); }
  async appStatus(name) { return this.exec(['app','status',name]); }
  async appInstall(name, opts = {}) {
    const a = ['app','install',name];
    if (opts.version) a.push('--version',opts.version);
    if (opts.sourceId) a.push('--source-id',String(opts.sourceId));
    if (opts.volumeId) a.push('--volume-id',String(opts.volumeId));
    a.push('--yes');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async appInstallFpk(localFile, volumeId, dryRun) {
    const a = ['app','install-fpk',localFile,'--volume-id',String(volumeId)];
    if (dryRun) a.push('--dry-run');
    a.push('--yes');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async appUpdate(name, opts = {}) {
    const a = ['app','update',name];
    if (opts.version) a.push('--version',opts.version);
    if (opts.volumeId) a.push('--volume-id',String(opts.volumeId));
    a.push('--yes');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async appStart(name) { return this.exec(['app','start',name,'--yes']); }
  async appStop(name) { return this.exec(['app','stop',name,'--yes']); }
  async appUninstall(name) { return this.exec(['app','uninstall',name,'--yes']); }

  // ==================== Download ====================
  async downloadLs(keyword) { const a = ['download','ls']; if (keyword) a.push(keyword); return this.exec(a); }
  async downloadInfo(id) { return this.exec(['download','info',String(id)]); }
  async downloadFiles(id) { return this.exec(['download','files',String(id)]); }
  async downloadAddUri(uri, saveDir) { return this.exec(['download','add-uri',uri,saveDir], { timeout: this.longTimeout }); }
  async downloadAddPath(p, saveDir) { return this.exec(['download','add-path',p,saveDir], { timeout: this.longTimeout }); }
  async downloadPause(...ids) { return this.exec(['download','pause',...ids.map(String)]); }
  async downloadResume(...ids) { return this.exec(['download','resume',...ids.map(String)]); }
  async downloadRetry(...ids) { return this.exec(['download','retry',...ids.map(String)]); }
  async downloadRm(...ids) { return this.exec(['download','rm',...ids.map(String)]); }
  async downloadStat() { return this.exec(['download','stat']); }

  // ==================== Logger ====================
  async loggerList(opts = {}) {
    const a = ['logger','list'];
    if (opts.page) a.push('--page',String(opts.page));
    if (opts.pageSize) a.push('--page-size',String(opts.pageSize));
    if (opts.level != null) a.push('--level',String(opts.level));
    if (opts.module != null) a.push('--module',String(opts.module));
    if (opts.locale) a.push('--locale',opts.locale);
    return this.exec(a);
  }
  async loggerModules(locale) { const a = ['logger','modules']; if (locale) a.push('--locale',locale); return this.exec(a); }
  async loggerClear(level, module) { return this.exec(['logger','clear','--level',String(level),'--module',String(module)]); }
  async loggerExport(level, module, locale) {
    const a = ['logger','export','--level',String(level),'--module',String(module)];
    if (locale) a.push('--locale',locale);
    return this.exec(a, { rawOutput: true });
  }
  async loggerArchiveSet(opts = {}) {
    const a = ['logger','archive','set','--switch',String(opts.switchVal),'--file-path',opts.filePath || ''];
    if (opts.sizeGt != null) a.push('--size-gt',String(opts.sizeGt));
    if (opts.dateUnit != null) a.push('--date-unit',String(opts.dateUnit));
    if (opts.dateBefore != null) a.push('--date-before',String(opts.dateBefore));
    return this.exec(a);
  }
  async loggerArchiveQuery() { return this.exec(['logger','archive','query']); }

  // ==================== User ====================
  async userList(uver) { const a = ['user','list']; if (uver != null) a.push('--uver',String(uver)); return this.exec(a); }
  async userFrozenList() { return this.exec(['user','frozen-list']); }
  async userInfo(user) { const a = ['user','info']; if (user) a.push(user); return this.exec(a); }
  async userListUg(opts = {}) {
    const a = ['user','list-ug'];
    if (opts.users) a.push('--users'); if (opts.groups) a.push('--groups');
    if (opts.uver != null) a.push('--uver',String(opts.uver));
    return this.exec(a);
  }
  async userGroupList() { return this.exec(['user','group-list']); }
  async userGroupInfo(group) { return this.exec(['user','group-info',group]); }
  async userGroupUsers(uver) { const a = ['user','group-users']; if (uver != null) a.push('--uver',String(uver)); return this.exec(a); }
  async userListLoginDevice() { return this.exec(['user','list-login-device']); }
  async userAdd(user, opts = {}) {
    const a = ['user','add',user];
    if (opts.password) a.push('--password',opts.password);
    if (opts.groups) opts.groups.forEach(g => a.push('--groups',g));
    if (opts.comment) a.push('--comment',opts.comment);
    if (opts.email) a.push('--email',opts.email);
    if (opts.mobile) a.push('--mobile',opts.mobile);
    if (opts.disableChangePassword) a.push('--disable-change-password');
    if (opts.setAdmin) a.push('--set-admin');
    a.push('-y');
    return this.exec(a);
  }
  async userMod(user, opts = {}) {
    const a = ['user','mod',user];
    if (opts.newName) a.push('--new-name',opts.newName);
    if (opts.password) a.push('--password',opts.password);
    if (opts.groups) opts.groups.forEach(g => a.push('--groups',g));
    if (opts.comment) a.push('--comment',opts.comment);
    if (opts.email) a.push('--email',opts.email);
    if (opts.mobile) a.push('--mobile',opts.mobile);
    if (opts.disableChangePassword) a.push('--disable-change-password');
    if (opts.enableChangePassword) a.push('--enable-change-password');
    if (opts.disableUser != null) a.push('--disable-user',String(opts.disableUser));
    if (opts.allowSsh) a.push('--allow-ssh');
    if (opts.disallowSsh) a.push('--disallow-ssh');
    a.push('-y');
    return this.exec(a);
  }
  async userDel(user) { return this.exec(['user','del',user,'-y']); }
  async userSetAdmin(user, on) { return this.exec(['user','set-admin',user, on ? '--on' : '--off', '-y']); }
  async userUnfreeze(user) { return this.exec(['user','unfreeze',user,'-y']); }
  async userChangePassword(user, opts = {}) {
    const a = ['user','change-password'];
    if (user) a.push(user);
    if (opts.oldPassword) a.push('--old-password',opts.oldPassword);
    if (opts.newPassword) a.push('--new-password',opts.newPassword);
    if (opts.removeToken) a.push('--remove-token');
    a.push('-y');
    return this.exec(a);
  }
  async userGroupAdd(group, comment) { const a = ['user','group-add',group]; if (comment) a.push('--comment',comment); a.push('-y'); return this.exec(a); }
  async userGroupMod(group, opts = {}) { const a = ['user','group-mod',group]; if (opts.newName) a.push('--new-name',opts.newName); if (opts.comment) a.push('--comment',opts.comment); a.push('-y'); return this.exec(a); }
  async userGroupDel(group) { return this.exec(['user','group-del',group,'-y']); }
  async userGroupSetUsers(group, users) { const a = ['user','group-set-users',group]; users.forEach(u => a.push('--users',u)); a.push('-y'); return this.exec(a); }
  async userGroupAddUsers(group, users) { const a = ['user','group-add-users',group]; users.forEach(u => a.push('--users',u)); a.push('-y'); return this.exec(a); }
  async userGroupDelUsers(group, users) { const a = ['user','group-del-users',group]; users.forEach(u => a.push('--users',u)); a.push('-y'); return this.exec(a); }

  // ==================== Storage ====================
  async storageOverview() { return this.exec(['storage','overview']); }
  async storagePools() { return this.exec(['storage','pools']); }
  async storageDisks() { return this.exec(['storage','disks']); }
  async storageRemovable() { return this.exec(['storage','removable']); }
  async storageSpace() { return this.exec(['storage','space']); }
  async storageHealth(disk) { return this.exec(['storage','health',disk]); }
  async storageSmart(disk) { return this.exec(['storage','smart',disk]); }
  async storageMount(uuid) { return this.exec(['storage','mount',uuid,'-y']); }
  async storageUmount(uuid, password) { const a = ['storage','umount',uuid]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }
  async storageCreate(opts = {}) {
    const a = ['storage','create','--level',String(opts.level),'--disks',opts.disks.join(',')];
    if (opts.fstype) a.push('--fstype',opts.fstype);
    if (opts.comment) a.push('--comment',opts.comment);
    if (opts.checkDisk) a.push('--check-disk');
    if (opts.password) a.push('--password',opts.password);
    a.push('-y');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async storageStop(uuid, password) { const a = ['storage','stop',uuid]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }
  async storageAddDisk(uuid, disks, password) { const a = ['storage','add-disk',uuid,'--disks',disks.join(',')]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }
  async storageRemoveDisk(uuid, disks, password) { const a = ['storage','remove-disk',uuid,'--disks',disks.join(',')]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }
  async storageReplaceDisk(uuid, disks, newDisks, password) { const a = ['storage','replace-disk',uuid,'--disks',disks.join(','),'--new-disks',newDisks.join(',')]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }
  async storageExtend(uuid, password) { const a = ['storage','extend',uuid]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }
  async storageResize(uuid, opts = {}) {
    const a = ['storage','resize',uuid,'--disks',opts.disks.join(',')];
    if (opts.vdName) a.push('--vd-name',opts.vdName);
    if (opts.level != null) a.push('--level',String(opts.level));
    if (opts.password) a.push('--password',opts.password);
    a.push('-y');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async storageFormat(disk, fstype, partition, password) {
    const a = ['storage','format',disk,'--fstype',fstype];
    if (partition != null) a.push('--partition',String(partition));
    if (password) a.push('--password',password);
    a.push('-y');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async storageEject(disk, password) { const a = ['storage','eject',disk]; if (password) a.push('--password',password); a.push('-y'); return this.exec(a, { timeout: this.longTimeout }); }

  // ==================== Docker ====================
  async dockerStats() { return this.exec(['docker','stats']); }
  async dockerImageLs() { return this.exec(['docker','image','ls']); }
  async dockerImagePull(imageRef) { return this.exec(['docker','image','pull',imageRef], { timeout: this.longTimeout }); }
  async dockerImageInspect(imageRef) { return this.exec(['docker','image','inspect',imageRef]); }
  async dockerImageRm(imageRef, force) { const a = ['docker','image','rm',imageRef]; if (force) a.push('--force'); a.push('--yes'); return this.exec(a); }
  async dockerContainerLs() { return this.exec(['docker','container','ls']); }
  async dockerContainerInspect(id) { return this.exec(['docker','container','inspect',id]); }
  async dockerContainerTop(id) { return this.exec(['docker','container','top',id]); }
  async dockerContainerStats(id) { return this.exec(['docker','container','stats',id]); }
  async dockerContainerStart(id) { return this.exec(['docker','container','start',id], { timeout: this.longTimeout }); }
  async dockerContainerStop(id) { return this.exec(['docker','container','stop',id], { timeout: this.longTimeout }); }
  async dockerContainerRestart(id) { return this.exec(['docker','container','restart',id], { timeout: this.longTimeout }); }
  async dockerContainerKill(id) { return this.exec(['docker','container','kill',id]); }
  async dockerContainerRm(id, force) { const a = ['docker','container','rm',id]; if (force) a.push('--force'); a.push('--yes'); return this.exec(a); }
  async dockerContainerCreate(opts = {}) {
    const a = ['docker','container','create','--image',opts.image];
    if (opts.name) a.push('--name',opts.name);
    if (opts.start) a.push('--start');
    if (opts.restart) a.push('--restart');
    if (opts.memory) a.push('--memory',String(opts.memory));
    if (opts.cpu) a.push('--cpu',String(opts.cpu));
    if (opts.env) opts.env.forEach(e => a.push('--env',e));
    if (opts.cmd) opts.cmd.forEach(c => a.push('--cmd',c));
    if (opts.port) opts.port.forEach(p => a.push('--port',p));
    if (opts.mount) opts.mount.forEach(m => a.push('--mount',m));
    a.push('--yes');
    return this.exec(a, { timeout: this.longTimeout });
  }
  async dockerComposeLs() { return this.exec(['docker','compose','ls']); }
}

module.exports = TrimCLI;
