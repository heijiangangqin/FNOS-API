package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// ============ 只读接口 ============

func (a *App) registerReadRoutes(mux *http.ServeMux) {
	// 快捷聚合
	mux.HandleFunc("GET /api/status", a.auth(a.run([]string{"+status"}, false)))

	// 系统
	for _, c := range []string{
		"info", "trim-version", "kernel-version", "host-name", "machine-id", "uptime",
		"all-vols", "vols", "docker", "unix-time", "boot-on-power", "hardware",
		"reserved-partition", "is-trim-machine", "trim-feature", "fan-mode",
		"trim-disk", "time-setting", "timezone-list", "power-plan",
		"next-power-off-time", "power-plan-status", "power-on-seconds", "inited-timestamp",
	} {
		mux.HandleFunc("GET /api/system/"+c, a.auth(a.run([]string{"system", c}, false)))
	}

	// 监控
	for _, c := range []string{
		"cpu", "memory", "net", "disk", "gpu", "npu",
		"proc-list", "proc-srv", "sys-warn", "battery", "sys-fan", "beep-supported",
	} {
		mux.HandleFunc("GET /api/monitor/"+c, a.auth(a.run([]string{"monitor", c}, false)))
	}
	// gen 必须带 --item（cpu/memory/net/disk/gpu/npu 等）
	mux.HandleFunc("GET /api/monitor/gen", a.auth(func(w http.ResponseWriter, r *http.Request) {
		item := q(r, "item", "")
		if item == "" {
			a.fail(w, paramErr("缺少 item 参数"))
			return
		}
		args := []string{"monitor", "gen"}
		for _, it := range splitCSV(item) {
			args = append(args, "--item", it)
		}
		a.execInto(w, args, false)
	}))
	// proc-info 必须带 --pids
	mux.HandleFunc("GET /api/monitor/proc-info", a.auth(func(w http.ResponseWriter, r *http.Request) {
		pids := q(r, "pids", q(r, "pid", ""))
		if pids == "" {
			a.fail(w, paramErr("缺少 pids 参数"))
			return
		}
		a.execInto(w, []string{"monitor", "proc-info", "--pids", pids}, false)
	}))

	// 存储（只读）
	for _, c := range []string{"overview", "pools", "disks", "free-disks", "removable", "space"} {
		mux.HandleFunc("GET /api/storage/"+c, a.auth(a.run([]string{"storage", c}, false)))
	}
	for _, c := range []string{"health", "smart", "disk-info"} {
		cmd := c
		mux.HandleFunc("GET /api/storage/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			disk := q(r, "disk", "")
			if disk == "" {
				a.fail(w, paramErr("缺少 disk 参数"))
				return
			}
			// 上游对不存在的磁盘有的返回空数据、有的报错，这里统一先核对一遍
			if exists, known := a.diskExists(disk); known && !exists {
				a.fail(w, &CLIError{Message: "磁盘不存在：" + disk, Status: httpStatusNotFound})
				return
			}
			a.execInto(w, []string{"storage", cmd, disk}, false)
		}))
	}

	// 文件（只读）
	mux.HandleFunc("GET /api/file/ls", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"file", "ls"}
		if p := q(r, "path", ""); p != "" {
			args = append(args, p)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/ls-dir", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"file", "ls-dir"}
		if p := q(r, "path", ""); p != "" {
			args = append(args, p)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/usage", a.auth(func(w http.ResponseWriter, r *http.Request) {
		vol := q(r, "vol", "1")
		a.execInto(w, []string{"file", "usage", vol}, false)
	}))
	mux.HandleFunc("GET /api/file/search", a.auth(func(w http.ResponseWriter, r *http.Request) {
		key := q(r, "key", "")
		if key == "" {
			a.fail(w, paramErr("缺少 key 参数"))
			return
		}
		args := []string{"file", "search", key}
		if paths := q(r, "paths", ""); paths != "" {
			args = append(args, splitCSV(paths)...)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/search-others", a.auth(func(w http.ResponseWriter, r *http.Request) {
		a.execInto(w, []string{"file", "search-others", q(r, "key", "")}, false)
	}))
	for _, c := range []string{"size", "access", "is-mount-point"} {
		cmd := c
		mux.HandleFunc("GET /api/file/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			p := q(r, "path", "")
			if p == "" {
				a.fail(w, paramErr("缺少 path 参数"))
				return
			}
			if cmd == "access" && a.pathMissing(p) {
				a.fail(w, &CLIError{Message: "路径不存在：" + p, Status: httpStatusNotFound})
				return
			}
			a.execInto(w, []string{"file", cmd, p}, false)
		}))
	}
	mux.HandleFunc("GET /api/file/acl/get", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := q(r, "path", "")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		args := []string{"file", "acl", "get", p}
		if q(r, "prop", "") == "true" {
			args = append(args, "--prop")
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/share/info", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := q(r, "path", "")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		data, err := a.cli.Run([]string{"file", "share", "info", p}, false)
		if err != nil {
			a.fail(w, err)
			return
		}
		if !looksLikeShare(data) {
			a.fail(w, &CLIError{Message: "该路径不是共享目录：" + p, Status: httpStatusNotFound})
			return
		}
		a.ok(w, data)
	}))
	mux.HandleFunc("GET /api/file/share/list", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"file", "share", "list"}
		if uid := q(r, "uid", ""); uid != "" {
			args = append(args, uid)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/share/list-others", a.auth(a.run([]string{"file", "share", "list-others"}, false)))
	mux.HandleFunc("GET /api/file/share/admin-list", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"file", "share", "admin-list"}
		if uid := q(r, "uid", ""); uid != "" {
			args = append(args, uid)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/share/admin-list-others", a.auth(a.run([]string{"file", "share", "admin-list-others"}, false)))
	mux.HandleFunc("GET /api/file/trash/list", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"file", "trash", "list"}
		if p := q(r, "path", ""); p != "" {
			args = append(args, p)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/file/fav/list", a.auth(a.run([]string{"file", "fav", "list"}, false)))
	mux.HandleFunc("GET /api/file/recent/list", a.auth(a.run([]string{"file", "recent", "list"}, false)))
	// prop：上游会把「路径不存在」塞在 data.prop[].errno 里，这里提升为真正的失败
	mux.HandleFunc("GET /api/file/prop", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := q(r, "path", "")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		data, err := a.cli.Run([]string{"file", "prop", p}, false)
		if err != nil {
			a.fail(w, err)
			return
		}
		if code, allBad := propErrno(data); allBad {
			a.fail(w, &CLIError{
				Message: fmt.Sprintf("路径不可访问：%s（errno %d）", p, code),
				ErrNo:   code, Status: httpStatusNotFound,
			})
			return
		}
		a.ok(w, data)
	}))
	mux.HandleFunc("GET /api/file/app-dir-list", a.auth(a.run([]string{"file", "app-dir-list"}, false)))
	mux.HandleFunc("GET /api/file/sys-part-info", a.auth(a.run([]string{"file", "sys-part-info"}, false)))

	// 应用中心（只读）
	mux.HandleFunc("GET /api/app/list", a.auth(a.run([]string{"app", "list"}, false)))
	mux.HandleFunc("GET /api/app/check-update", a.auth(a.run([]string{"app", "check-update"}, false)))
	mux.HandleFunc("GET /api/app/status", a.auth(func(w http.ResponseWriter, r *http.Request) {
		name := q(r, "name", "")
		if name == "" {
			a.fail(w, paramErr("缺少 name 参数"))
			return
		}
		a.execInto(w, []string{"app", "status", name}, false)
	}))

	// 下载（只读）
	mux.HandleFunc("GET /api/download/ls", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"download", "ls"}
		if k := q(r, "keyword", ""); k != "" {
			args = append(args, k)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/download/stat", a.auth(a.run([]string{"download", "stat"}, false)))
	for _, c := range []string{"info", "files"} {
		cmd := c
		mux.HandleFunc("GET /api/download/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			id := q(r, "id", "")
			if id == "" {
				a.fail(w, paramErr("缺少 id 参数"))
				return
			}
			a.execInto(w, []string{"download", cmd, id}, false)
		}))
	}

	// Docker（只读）
	mux.HandleFunc("GET /api/docker/stats", a.auth(a.run([]string{"docker", "stats"}, false)))
	mux.HandleFunc("GET /api/docker/image/ls", a.auth(a.run([]string{"docker", "image", "ls"}, false)))
	mux.HandleFunc("GET /api/docker/image/inspect", a.auth(func(w http.ResponseWriter, r *http.Request) {
		ref := q(r, "ref", "")
		if ref == "" {
			a.fail(w, paramErr("缺少 ref 参数"))
			return
		}
		a.execInto(w, []string{"docker", "image", "inspect", ref}, false)
	}))
	mux.HandleFunc("GET /api/docker/container/ls", a.auth(a.run([]string{"docker", "container", "ls"}, false)))
	mux.HandleFunc("GET /api/docker/compose/ls", a.auth(a.run([]string{"docker", "compose", "ls"}, false)))
	for _, c := range []string{"inspect", "top", "stats"} {
		cmd := c
		mux.HandleFunc("GET /api/docker/container/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			id := q(r, "id", "")
			if id == "" {
				a.fail(w, paramErr("缺少 id 参数"))
				return
			}
			a.execInto(w, []string{"docker", "container", cmd, id}, false)
		}))
	}

	// 用户（只读）
	mux.HandleFunc("GET /api/user/list", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"user", "list"}
		if v := q(r, "uver", ""); v != "" {
			args = append(args, "--uver", v)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/user/info", a.auth(func(w http.ResponseWriter, r *http.Request) {
		u := q(r, "user", "")
		if u == "" {
			a.fail(w, paramErr("缺少 user 参数（用户名，不是 uid）"))
			return
		}
		// CLI 只认用户名，传 uid 会得到上游 errno 131073
		if isAllDigits(u) {
			a.fail(w, paramErr("user/info 只接受用户名，不接受 uid（用 /api/user/list 查对应关系）"))
			return
		}
		a.execInto(w, []string{"user", "info", u}, false)
	}))
	mux.HandleFunc("GET /api/user/group-list", a.auth(a.run([]string{"user", "group-list"}, false)))
	mux.HandleFunc("GET /api/user/group-users", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"user", "group-users"}
		if v := q(r, "uver", ""); v != "" {
			args = append(args, "--uver", v)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/user/frozen-list", a.auth(a.run([]string{"user", "frozen-list"}, false)))
	mux.HandleFunc("GET /api/user/list-login-device", a.auth(a.run([]string{"user", "list-login-device"}, false)))
	mux.HandleFunc("GET /api/user/list-ug", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"user", "list-ug"}
		if q(r, "users", "") != "" {
			args = append(args, "--users")
		}
		if q(r, "groups", "") != "" {
			args = append(args, "--groups")
		}
		a.execInto(w, args, false)
	}))

	// 日志（只读）
	mux.HandleFunc("GET /api/logger/modules", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"logger", "modules"}
		if l := q(r, "locale", ""); l != "" {
			args = append(args, "--locale", l)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/logger/list", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"logger", "list"}
		if v := q(r, "page", ""); v != "" {
			args = append(args, "--page", v)
		}
		if v := q(r, "page_size", ""); v != "" {
			args = append(args, "--page-size", v)
		}
		if v := q(r, "level", ""); v != "" {
			args = append(args, "--level", v)
		}
		if v := q(r, "module", ""); v != "" {
			args = append(args, "--module", v)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("GET /api/logger/archive/query", a.auth(a.run([]string{"logger", "archive", "query"}, false)))

	// 相册
	mux.HandleFunc("GET /api/photos/folders", a.auth(a.run([]string{"photos", "folders"}, false)))
	mux.HandleFunc("GET /api/photos/search", a.auth(func(w http.ResponseWriter, r *http.Request) {
		key := q(r, "key", "")
		if key == "" {
			a.fail(w, paramErr("缺少 key 参数"))
			return
		}
		args := []string{"photos", "search", key}
		if v := q(r, "limit", ""); v != "" {
			args = append(args, "--limit", v)
		}
		a.execInto(w, args, false)
	}))

	// 百度网盘（只读）
	mux.HandleFunc("GET /api/baidu/status", a.auth(a.run([]string{"baidu-netdisk", "status"}, false)))
	mux.HandleFunc("GET /api/baidu/quota", a.auth(a.run([]string{"baidu-netdisk", "quota"}, false)))
	mux.HandleFunc("GET /api/baidu/ls", a.auth(func(w http.ResponseWriter, r *http.Request) {
		args := []string{"baidu-netdisk", "ls"}
		if p := q(r, "path", ""); p != "" {
			args = append(args, p)
		}
		a.execInto(w, args, false)
	}))
	// 百度网盘任务列表必须带 --transfer（upload/download）
	mux.HandleFunc("GET /api/baidu/task/list", a.auth(func(w http.ResponseWriter, r *http.Request) {
		transfer := q(r, "transfer", "")
		if transfer == "" {
			a.fail(w, paramErr("缺少 transfer 参数（upload 或 download）"))
			return
		}
		args := []string{"baidu-netdisk", "task", "list", "--transfer", transfer}
		for _, k := range []struct{ param, flag string }{
			{"state", "--state"}, {"page", "--page"}, {"page_size", "--page-size"}, {"sort_by", "--sort-by"},
		} {
			if v := q(r, k.param, ""); v != "" {
				args = append(args, k.flag, v)
			}
		}
		if q(r, "ascending", "") == "true" {
			args = append(args, "--ascending")
		}
		a.execInto(w, args, false)
	}))
}

// ============ 写接口 ============

func (a *App) registerWriteRoutes(mux *http.ServeMux) {

	// 文件写
	mux.HandleFunc("POST /api/file/mkdir", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := str(body(r), "path")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		a.execInto(w, []string{"file", "mkdir", p, "--yes"}, true)
	}))
	mux.HandleFunc("POST /api/file/rm", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		p := str(m, "path")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		_ = m // trim-cli file rm 没有递归开关，删目录本身就是递归的
		if a.pathMissing(p) {
			a.fail(w, &CLIError{Message: "路径不存在：" + p, Status: httpStatusNotFound})
			return
		}
		a.execInto(w, []string{"file", "rm", p, "--yes"}, true)
	}))
	for _, c := range []string{"cp", "mv"} {
		cmd := c
		mux.HandleFunc("POST /api/file/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			m := body(r)
			src := str(m, "src")
			// 命令行第二个参数是「目标目录」，不是完整文件路径。
			// 这里接受 dest_dir（推荐）/ dest / dst 三种写法。
			dir := str(m, "dest_dir")
			if dir == "" {
				dir = str(m, "dest")
			}
			if dir == "" {
				dir = str(m, "dst")
			}
			if src == "" || dir == "" {
				a.fail(w, paramErr("缺少 src 或 dest_dir 参数"))
				return
			}
			if a.pathMissing(dir) {
				a.fail(w, &CLIError{
					Message: "目标目录不存在：" + dir + "（" + cmd + " 的第二个参数是目录，不是文件路径）",
					Status:  httpStatusBadRequest,
				})
				return
			}
			a.execInto(w, []string{"file", cmd, src, dir, "--yes"}, true)
		}))
	}
	mux.HandleFunc("POST /api/file/rename", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		src, name := str(m, "src"), str(m, "name")
		if src == "" {
			src = str(m, "path") // 别名
		}
		if name == "" {
			name = str(m, "new_name") // 别名
		}
		if src == "" || name == "" {
			a.fail(w, paramErr("缺少 src / name 参数（也接受 path / new_name）"))
			return
		}
		a.execInto(w, []string{"file", "rename", src, name, "--yes"}, true)
	}))
	mux.HandleFunc("POST /api/file/share/add", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		p, name := str(m, "path"), str(m, "name")
		if p == "" || name == "" {
			a.fail(w, paramErr("缺少 path / name 参数"))
			return
		}
		// permset 是 ACL 对象数组，CLI 要一个 JSON 字符串：数组直接序列化，字符串原样透传
		permset := ""
		switch v := m["permset"].(type) {
		case string:
			permset = v
		case []any, map[string]any:
			if b, err := json.Marshal(v); err == nil {
				permset = string(b)
			}
		}
		if permset == "" {
			a.fail(w, paramErr("缺少 permset 参数，需为 ACL 对象数组，如 [{\"owner\":32719,\"inherit\":7}]"))
			return
		}
		data, err := a.cli.Run([]string{"file", "share", "add", p, name, "--permset", permset, "--yes"}, true)
		if err != nil {
			// 实测上游在 permset 非法时会「先建共享再报错」，留下查得到、删不掉的半成品。
			// 这里主动回滚一次，再返回原始错误。
			_, _ = a.cli.Run([]string{"file", "share", "del", p, "--yes"}, true)
			a.fail(w, err)
			return
		}
		a.ok(w, data)
	}))
	mux.HandleFunc("POST /api/file/share/del", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := str(body(r), "path")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		// 上游对未共享的目录也会回“删除成功”，这里先核对一次。
		// 注意 share/list 看不到「半成品」共享（add 失败但记录已建），所以再用 info 兜底。
		if exists, known := a.shareState(p); known && !exists {
			a.fail(w, &CLIError{Message: "该路径不是共享目录：" + p, Status: httpStatusNotFound})
			return
		}
		a.execInto(w, []string{"file", "share", "del", p, "--yes"}, true)
	}))

	// 存储写（高危，均需显式 --yes）
	mux.HandleFunc("POST /api/storage/mount", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		uuid := str(m, "uuid")
		if uuid == "" {
			a.fail(w, paramErr("缺少 uuid 参数"))
			return
		}
		a.execInto(w, []string{"storage", "mount", uuid, "--yes"}, true)
	}))
	for _, c := range []string{"umount", "stop", "extend", "eject", "format"} {
		cmd := c
		mux.HandleFunc("POST /api/storage/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			m := body(r)
			target := str(m, "uuid")
			if target == "" {
				target = str(m, "disk")
			}
			if target == "" {
				a.fail(w, paramErr("缺少 uuid / disk 参数"))
				return
			}
			args := []string{"storage", cmd, target}
			if pw := str(m, "password"); pw != "" {
				args = append(args, "--password", pw)
			}
			if cmd == "format" {
				if fs := str(m, "fstype"); fs != "" {
					args = append(args, "--fstype", fs)
				}
			}
			args = append(args, "--yes")
			a.execInto(w, args, true)
		}))
	}

	// 应用中心写
	mux.HandleFunc("POST /api/app/start", a.auth(a.appAction("start")))
	mux.HandleFunc("POST /api/app/stop", a.auth(a.appAction("stop")))
	mux.HandleFunc("POST /api/app/restart", a.auth(a.appAction("restart")))
	mux.HandleFunc("POST /api/app/uninstall", a.auth(a.appAction("uninstall")))

	// 下载写
	mux.HandleFunc("POST /api/download/add-uri", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		uri, dir := str(m, "uri"), str(m, "saveDir")
		if uri == "" || dir == "" {
			a.fail(w, paramErr("缺少 uri / saveDir 参数"))
			return
		}
		data, err := a.cli.Run([]string{"download", "add-uri", uri, dir, "--yes"}, true)
		if err != nil {
			a.fail(w, err)
			return
		}
		if msg, bad := downloadRejected(data); bad {
			a.fail(w, &CLIError{Message: "下载未被接受：" + msg, Status: httpStatusBadRequest})
			return
		}
		a.ok(w, data)
	}))
	for _, c := range []string{"pause", "resume", "retry", "rm"} {
		cmd := c
		mux.HandleFunc("POST /api/download/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			ids := strList(body(r), "ids")
			if len(ids) == 0 {
				a.fail(w, paramErr("缺少 ids 参数（数组，数字或字符串均可）"))
				return
			}
			// 上游对不存在的 id 是静默 no-op，这里先核对，避免假成功
			if known, missing := a.downloadMissing(ids); known && len(missing) > 0 {
				a.fail(w, &CLIError{
					Message: "下载任务不存在：" + strings.Join(missing, ","),
					Status:  httpStatusNotFound,
				})
				return
			}
			a.execInto(w, append(append([]string{"download", cmd}, ids...), "--yes"), true)
		}))
	}

	// Docker 写
	mux.HandleFunc("POST /api/docker/image/pull", a.auth(func(w http.ResponseWriter, r *http.Request) {
		ref := str(body(r), "ref")
		if ref == "" {
			a.fail(w, paramErr("缺少 ref 参数"))
			return
		}
		a.execInto(w, []string{"docker", "image", "pull", ref, "--yes"}, true)
	}))
	for _, c := range []string{"start", "stop", "restart", "kill"} {
		cmd := c
		mux.HandleFunc("POST /api/docker/container/"+cmd, a.auth(func(w http.ResponseWriter, r *http.Request) {
			id := str(body(r), "id")
			if id == "" {
				a.fail(w, paramErr("缺少 id 参数"))
				return
			}
			a.execInto(w, []string{"docker", "container", cmd, id, "--yes"}, true)
		}))
	}

	// 用户写
	mux.HandleFunc("POST /api/user/del", a.auth(func(w http.ResponseWriter, r *http.Request) {
		u := str(body(r), "user")
		if u == "" {
			a.fail(w, paramErr("缺少 user 参数"))
			return
		}
		a.execInto(w, []string{"user", "del", u, "--yes"}, false)
	}))
	mux.HandleFunc("POST /api/user/set-admin", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		u := str(m, "user")
		if u == "" {
			a.fail(w, paramErr("缺少 user 参数"))
			return
		}
		flag := "--off"
		if boolean(m, "on") {
			flag = "--on"
		}
		a.execInto(w, []string{"user", "set-admin", u, flag, "--yes"}, false)
	}))

	// 新建用户
	mux.HandleFunc("POST /api/user/add", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		u := str(m, "user")
		if u == "" {
			a.fail(w, paramErr("缺少 user 参数（新用户名）"))
			return
		}
		args := []string{"user", "add", u}
		for _, opt := range []struct{ key, flag string }{
			{"password", "--password"}, {"groups", "--groups"},
			{"comment", "--comment"}, {"email", "--email"}, {"mobile", "--mobile"},
		} {
			if v := str(m, opt.key); v != "" {
				args = append(args, opt.flag, v)
			}
		}
		if boolean(m, "admin") {
			args = append(args, "--set-admin")
		}
		if boolean(m, "disable_change_password") {
			args = append(args, "--disable-change-password")
		}
		a.execInto(w, append(args, "--yes"), true)
	}))

	// 修改用户：改名、改备注、改邮箱、改组、改 SSH、禁用/启用
	mux.HandleFunc("POST /api/user/mod", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		u := str(m, "user")
		if u == "" {
			a.fail(w, paramErr("缺少 user 参数（要修改的用户名）"))
			return
		}
		args := []string{"user", "mod", u}
		for _, opt := range []struct{ key, flag string }{
			{"new_name", "--new-name"}, {"password", "--password"}, {"groups", "--groups"},
			{"comment", "--comment"}, {"email", "--email"}, {"mobile", "--mobile"},
		} {
			if v := str(m, opt.key); v != "" {
				args = append(args, opt.flag, v)
			}
		}
		// 这四个是开关型参数：传了字段才生效
		if v, ok := m["allow_ssh"]; ok {
			if b, _ := v.(bool); b {
				args = append(args, "--allow-ssh")
			} else {
				args = append(args, "--disallow-ssh")
			}
		}
		if v, ok := m["disable_change_password"]; ok {
			if b, _ := v.(bool); b {
				args = append(args, "--disable-change-password")
			} else {
				args = append(args, "--enable-change-password")
			}
		}
		if v, ok := m["disable_user"]; ok {
			if b, _ := v.(bool); b {
				args = append(args, "--disable-user", "true")
			} else {
				args = append(args, "--disable-user", "false")
			}
		}
		if len(args) == 3 {
			a.fail(w, paramErr("没有要修改的内容，至少传一个：new_name / password / groups / comment / email / mobile / allow_ssh / disable_user"))
			return
		}
		a.execInto(w, append(args, "--yes"), true)
	}))

	// 改密码：不传 user 表示改当前登录用户自己的密码
	mux.HandleFunc("POST /api/user/change-password", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		np := str(m, "new_password")
		if np == "" {
			a.fail(w, paramErr("缺少 new_password 参数"))
			return
		}
		// 飞牛的 CLI 在改密码时会交互式询问「当前密码」，
		// 不传 old_password 会一直等输入直到超时，所以这里必须前置校验。
		if str(m, "old_password") == "" {
			a.fail(w, paramErr("缺少 old_password 参数：飞牛要求提供该用户当前密码才能修改"))
			return
		}
		args := []string{"user", "change-password"}
		if u := str(m, "user"); u != "" {
			args = append(args, u)
		}
		if v := str(m, "old_password"); v != "" {
			args = append(args, "--old-password", v)
		}
		args = append(args, "--new-password", np)
		if boolean(m, "remove_token") {
			args = append(args, "--remove-token")
		}
		// 改密码要求标准输入是终端，必须走 PTY；超时按普通请求算，避免久挂
		a.execIntoPTY(w, append(args, "--yes"), false)
	}))

	// 解冻被冻结的用户
	mux.HandleFunc("POST /api/user/unfreeze", a.auth(func(w http.ResponseWriter, r *http.Request) {
		u := str(body(r), "user")
		if u == "" {
			a.fail(w, paramErr("缺少 user 参数"))
			return
		}
		a.execInto(w, []string{"user", "unfreeze", u, "--yes"}, true)
	}))

	// 用户组：增 / 改 / 删 / 设置成员 / 加成员 / 移成员
	mux.HandleFunc("POST /api/user/group/add", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		g := str(m, "group")
		if g == "" {
			a.fail(w, paramErr("缺少 group 参数"))
			return
		}
		args := []string{"user", "group-add", g}
		if v := str(m, "comment"); v != "" {
			args = append(args, "--comment", v)
		}
		a.execInto(w, append(args, "--yes"), true)
	}))
	mux.HandleFunc("POST /api/user/group/mod", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		g := str(m, "group")
		if g == "" {
			a.fail(w, paramErr("缺少 group 参数"))
			return
		}
		args := []string{"user", "group-mod", g}
		if v := str(m, "new_name"); v != "" {
			args = append(args, "--new-name", v)
		}
		if v := str(m, "comment"); v != "" {
			args = append(args, "--comment", v)
		}
		if len(args) == 3 {
			a.fail(w, paramErr("没有要修改的内容，至少传 new_name 或 comment"))
			return
		}
		a.execInto(w, append(args, "--yes"), true)
	}))
	mux.HandleFunc("POST /api/user/group/del", a.auth(func(w http.ResponseWriter, r *http.Request) {
		g := str(body(r), "group")
		if g == "" {
			a.fail(w, paramErr("缺少 group 参数"))
			return
		}
		a.execInto(w, []string{"user", "group-del", g, "--yes"}, true)
	}))
	for _, act := range []string{"set-users", "add-users", "del-users"} {
		action := act
		mux.HandleFunc("POST /api/user/group/"+action, a.auth(func(w http.ResponseWriter, r *http.Request) {
			m := body(r)
			g := str(m, "group")
			if g == "" {
				a.fail(w, paramErr("缺少 group 参数"))
				return
			}
			users := strList(m, "users")
			if len(users) == 0 {
				if v := str(m, "users"); v != "" {
					users = splitCSV(v)
				}
			}
			if len(users) == 0 {
				a.fail(w, paramErr("缺少 users 参数（数组或逗号分隔字符串）"))
				return
			}
			args := []string{"user", "group-" + action, g, "--users", strings.Join(users, ",")}
			a.execInto(w, append(args, "--yes"), true)
		}))
	}

	// 网络 / 电源
	mux.HandleFunc("POST /api/network/ssh", a.auth(func(w http.ResponseWriter, r *http.Request) {
		// CLI 只认 --enable / --disable，不吃位置参数
		flag := "--disable"
		if boolean(body(r), "enable") {
			flag = "--enable"
		}
		a.execInto(w, []string{"network", "ssh", "switch", flag, "--yes"}, false)
	}))
	mux.HandleFunc("POST /api/power/reboot", a.auth(func(w http.ResponseWriter, r *http.Request) {
		a.execInto(w, []string{"power", "reboot", "--yes"}, true)
	}))
	mux.HandleFunc("POST /api/power/poweroff", a.auth(func(w http.ResponseWriter, r *http.Request) {
		a.execInto(w, []string{"power", "poweroff", "--yes"}, true)
	}))

	// 日志写
	mux.HandleFunc("POST /api/logger/export", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		level, module := numStr(m, "level"), numStr(m, "module")
		if level == "" || module == "" {
			a.fail(w, paramErr("缺少 level / module 参数（均可为数字或字符串）"))
			return
		}
		args := []string{"logger", "export", "--level", level, "--module", module}
		if v := str(m, "locale"); v != "" {
			args = append(args, "--locale", v)
		}
		args = append(args, "--yes")
		a.execInto(w, args, true)
	}))
}

func (a *App) appAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		name, _ := m["name"].(string)
		if name == "" {
			writeJSON(w, http.StatusBadRequest, apiResult{Success: false, Error: "缺少 name 参数"})
			return
		}
		a.execInto(w, []string{"app", action, name, "--yes"}, true)
	}
}

// execInto 统一执行并回写响应
// execIntoPTY 与 execInto 相同，但在伪终端里执行（用于要求 TTY 的命令）
func (a *App) execIntoPTY(w http.ResponseWriter, args []string, long bool) {
	data, err := a.cli.RunPTY(args, long)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.ok(w, data)
}

func (a *App) execInto(w http.ResponseWriter, args []string, long bool) {
	if err := validateArgs(args); err != nil {
		a.fail(w, paramErr(err.Error()))
		return
	}
	data, err := a.cli.Run(args, long)
	if err != nil {
		a.fail(w, err)
		return
	}
	a.ok(w, data)
}

func splitCSV(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// anyToStr 把 JSON 里的标量统一转成字符串。数值不加科学计数法，
// 这样 [3] 和 ["3"] 都能被下游命令行接受。
func anyToStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		if t {
			return "true"
		}
		return "false"
	}
	return ""
}

// pathMissing 判断路径是否不存在。用 file prop 的 errno 4100 作为判据；
// 判断不出来时一律返回 false（宁可放行，也不要挡住正常操作）。
func (a *App) pathMissing(p string) bool {
	data, err := a.cli.Run([]string{"file", "prop", p}, false)
	if err != nil {
		return false
	}
	code, allBad := propErrno(data)
	return allBad && code == 4100
}

// propErrno 解析 file prop 的返回：所有条目 errno 都非 0 时返回 (errno, true)
func propErrno(data any) (int, bool) {
	obj, ok := data.(map[string]any)
	if !ok {
		return 0, false
	}
	props, ok := obj["prop"].([]any)
	if !ok || len(props) == 0 {
		return 0, false
	}
	first := 0
	for i, it := range props {
		m, ok := it.(map[string]any)
		if !ok {
			return 0, false
		}
		e, has := m["errno"].(float64)
		if !has || int(e) == 0 {
			return 0, false
		}
		if i == 0 {
			first = int(e)
		}
	}
	return first, true
}

// hasShare 判断某个路径是否在共享列表中。
// 共享列表里的 path 不带前导斜杠（如 vol3/1000/xxx），这里做归一化后比较。
func hasShare(data any, path string) bool {
	list, ok := data.([]any)
	if !ok {
		return false
	}
	want := strings.TrimPrefix(strings.TrimSpace(path), "/")
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if got, ok := m["path"].(string); ok {
			if strings.TrimPrefix(got, "/") == want {
				return true
			}
		}
	}
	return false
}

// looksLikeShare 判断 share/info 的返回是否真的是一个共享
func looksLikeShare(data any) bool {
	m, ok := data.(map[string]any)
	if !ok {
		return false
	}
	if _, ok := m["shareName"]; ok {
		return true
	}
	if _, ok := m["path"]; ok {
		return true
	}
	return false
}

// shareState 返回 (是否存在, 判断是否可靠)。
// share/list 看不到「add 失败但已建记录」的半成品，所以再用 info 确认一次。
func (a *App) shareState(p string) (bool, bool) {
	shares, err := a.cli.Run([]string{"file", "share", "list"}, false)
	known := err == nil
	if err == nil && hasShare(shares, p) {
		return true, true
	}
	if data, err2 := a.cli.Run([]string{"file", "share", "info", p}, false); err2 == nil && looksLikeShare(data) {
		return true, true
	}
	return false, known
}

// downloadRejected 识别 add-uri 的失败标记：errs 非空或 ids 含 -1
func downloadRejected(data any) (string, bool) {
	m, ok := data.(map[string]any)
	if !ok {
		return "", false
	}
	var codes []string
	if errs, ok := m["errs"].([]any); ok {
		for _, e := range errs {
			if s := anyToStr(e); s != "" && s != "0" {
				codes = append(codes, s)
			}
		}
	}
	badID := false
	if ids, ok := m["ids"].([]any); ok {
		for _, v := range ids {
			if anyToStr(v) == "-1" {
				badID = true
			}
		}
	}
	if len(codes) > 0 || badID {
		msg := "上游返回错误码"
		if len(codes) > 0 {
			msg += " errs=" + strings.Join(codes, ",")
		}
		if badID {
			msg += " ids=-1"
		}
		return msg, true
	}
	return "", false
}

// downloadMissing 对照下载任务列表，返回不存在的 id。
// 列表拉不到时返回 (false, nil)，即判断不可靠就不拦。
func (a *App) downloadMissing(ids []string) (bool, []string) {
	data, err := a.cli.Run([]string{"download", "ls"}, false)
	if err != nil {
		return false, nil
	}
	m, ok := data.(map[string]any)
	if !ok {
		return false, nil
	}
	tasks, hasTasks := m["tasks"].([]any)
	if !hasTasks {
		// 上游在「一个任务都没有」时不返回 tasks 字段，只给 stats。
		// 这时所有 id 都不存在，不能当成“判断不可靠”而放行。
		if st, ok := m["stats"].(map[string]any); ok {
			if n, ok := st["all"].(float64); ok && int(n) == 0 {
				return true, ids
			}
		}
		return false, nil
	}
	exist := map[string]bool{}
	for _, t := range tasks {
		if tm, ok := t.(map[string]any); ok {
			exist[anyToStr(tm["id"])] = true
		}
	}
	var missing []string
	for _, id := range ids {
		if !exist[id] {
			missing = append(missing, id)
		}
	}
	return true, missing
}

// isAllDigits 判断是否纯数字
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// diskExists 用 storage/disks 核对磁盘名，返回 (是否存在, 判断是否可靠)
func (a *App) diskExists(name string) (bool, bool) {
	data, err := a.cli.Run([]string{"storage", "disks"}, false)
	if err != nil {
		return false, false
	}
	m, ok := data.(map[string]any)
	if !ok {
		return false, false
	}
	list, ok := m["disk"].([]any)
	if !ok {
		return false, false
	}
	for _, d := range list {
		if dm, ok := d.(map[string]any); ok {
			if anyToStr(dm["name"]) == name {
				return true, true
			}
		}
	}
	return false, true
}

// ---------- 请求体与参数读取（包级，供各路由文件共用） ----------

func body(r *http.Request) map[string]any {
	var m map[string]any
	_ = json.NewDecoder(r.Body).Decode(&m)
	if m == nil {
		m = map[string]any{}
	}
	return m
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func boolean(m map[string]any, k string) bool {
	b, _ := m[k].(bool)
	return b
}

func strList(m map[string]any, k string) []string {
	var out []string
	if arr, ok := m[k].([]any); ok {
		for _, v := range arr {
			if sv := anyToStr(v); sv != "" {
				out = append(out, sv)
			}
		}
	}
	return out
}

// numStr 数值型参数（如日志 level=0）也接受
func numStr(m map[string]any, k string) string { return anyToStr(m[k]) }
