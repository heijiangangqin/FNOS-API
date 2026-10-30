package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

// 本文件补齐 trim-cli 剩余的写/管理能力。
// 高风险操作（存储池建/加盘/移盘/换盘/扩容）必须显式传 confirm:true。

// jsonString 把任意值序列化成字符串（ACL 之类的字段需要 JSON 文本）
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// normalizeVol 把 "1" / "vol1" / "/vol1" 统一成 "/vol1"
func normalizeVol(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(v, "/vol") {
		return v
	}
	if strings.HasPrefix(v, "vol") {
		return "/" + v
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return v // 不是纯数字就原样传
		}
	}
	return "/vol" + v
}

// normalizeOverwrite 把 true/false 映射成 CLI 认识的 replace/skip，并校验取值
func normalizeOverwrite(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "replace", "true", "1", "yes":
		return "replace", true
	case "skip", "false", "0", "no":
		return "skip", true
	case "rename":
		return "rename", true
	}
	return "", false
}

// volNumber 把 "1" / "vol1" / "/vol1" 都取成 "1"。
// 注意 trimacl 这类命令要的就是卷号数字，不能传 /vol1。
func volNumber(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "/")
	v = strings.TrimPrefix(v, "vol")
	out := ""
	for _, r := range v {
		if r >= '0' && r <= '9' {
			out += string(r)
		}
	}
	return out
}

func withYes(args []string) []string {
	return append(args, "--yes")
}

func needConfirm(m map[string]any, what string) error {
	if b, _ := m["confirm"].(bool); !b {
		return paramErr(what + " 属于高风险操作，确认后请再传 \"confirm\": true")
	}
	return nil
}

// csvField 取一个可能是数组、也可能是逗号分隔字符串的字段
func csvField(m map[string]any, key string) []string {
	if list := strList(m, key); len(list) > 0 {
		return list
	}
	if v := str(m, key); v != "" {
		return splitCSV(v)
	}
	return nil
}

func (a *App) registerExtraRoutes(mux *http.ServeMux) {

	// ===================== 文件 =====================

	// 压缩：compress <outputPath> <sourcePath>...
	mux.HandleFunc("POST /api/file/compress", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		out := str(m, "output")
		srcs := csvField(m, "sources")
		if out == "" || len(srcs) == 0 {
			a.fail(w, paramErr("缺少 output 或 sources 参数（sources 可为数组）"))
			return
		}
		args := append([]string{"file", "compress", out}, srcs...)
		if v := anyToStr(m["level"]); v != "" {
			args = append(args, "--level", v)
		}
		if v := str(m, "password"); v != "" {
			args = append(args, "--password", v)
		}
		a.execInto(w, withYes(args), true)
	}))

	// 解压：extract <archivePath> <outputDir>
	mux.HandleFunc("POST /api/file/extract", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		arc, out := str(m, "archive"), str(m, "output_dir")
		if arc == "" || out == "" {
			a.fail(w, paramErr("缺少 archive 或 output_dir 参数"))
			return
		}
		args := []string{"file", "extract", arc, out}
		if v := anyToStr(m["overwrite"]); v != "" {
			args = append(args, "--overwrite", v)
		}
		if v := str(m, "password"); v != "" {
			args = append(args, "--password", v)
		}
		if v := str(m, "charset"); v != "" {
			args = append(args, "--from-charset", v)
		}
		a.execInto(w, withYes(args), true)
	}))

	// 改属主：chown <PATH> --uid
	mux.HandleFunc("POST /api/file/chown", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		p := str(m, "path")
		uid := anyToStr(m["uid"])
		if p == "" || uid == "" {
			a.fail(w, paramErr("缺少 path 或 uid 参数"))
			return
		}
		args := []string{"file", "chown", p, "--uid", uid}
		if boolean(m, "sub") {
			args = append(args, "--sub")
		}
		a.execInto(w, withYes(args), true)
	}))

	// 设置 ACL：acl set <PATH> --permset
	mux.HandleFunc("POST /api/file/acl/set", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		p := str(m, "path")
		permset := ""
		switch v := m["permset"].(type) {
		case string:
			permset = v
		case []any, map[string]any:
			permset = jsonString(v)
		}
		if p == "" || permset == "" {
			a.fail(w, paramErr("缺少 path 或 permset 参数（permset 为 ACL 对象数组）"))
			return
		}
		args := []string{"file", "acl", "set", p, "--permset", permset}
		if v := anyToStr(m["sub"]); v != "" {
			args = append(args, "--sub", v)
		}
		a.execInto(w, withYes(args), true)
	}))

	// 回收站搜索 / 还原 / 清空
	mux.HandleFunc("GET /api/file/trash/search", a.auth(func(w http.ResponseWriter, r *http.Request) {
		key := q(r, "key", "")
		if key == "" {
			a.fail(w, paramErr("缺少 key 参数"))
			return
		}
		args := []string{"file", "trash", "search", key}
		if v := q(r, "uid", ""); v != "" {
			args = append(args, "--uid", v)
		}
		a.execInto(w, args, false)
	}))
	mux.HandleFunc("POST /api/file/trash/restore", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := str(body(r), "path")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		data, err := a.cli.Run([]string{"file", "trash", "restore", p, "--yes"}, true)
		if err != nil {
			a.fail(w, err)
			return
		}
		// 上游对「不在回收站」的路径也回 Restored，这里事后核验一次
		if a.pathMissing(p) {
			a.fail(w, &CLIError{
				Message: "还原未生效：" + p + " 还原后仍不存在，可能不在回收站中",
				Status:  httpStatusConflict,
			})
			return
		}
		a.ok(w, data)
	}))
	mux.HandleFunc("POST /api/file/trash/clear", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		if err := needConfirm(m, "清空回收站"); err != nil {
			a.fail(w, err)
			return
		}
		args := []string{"file", "trash", "clear"}
		if p := str(m, "path"); p != "" {
			args = append(args, p)
		}
		a.execInto(w, withYes(args), true)
	}))

	// 收藏
	mux.HandleFunc("POST /api/file/fav/add", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := str(body(r), "path")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		a.execInto(w, withYes([]string{"file", "fav", "add", p}), true)
	}))
	mux.HandleFunc("POST /api/file/fav/del", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := str(body(r), "path")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		a.execInto(w, withYes([]string{"file", "fav", "del", p}), true)
	}))

	// 团队空间
	mux.HandleFunc("GET /api/file/team/list", a.auth(a.run([]string{"file", "team", "list"}, false)))
	mux.HandleFunc("POST /api/file/team/trash-restore", a.auth(func(w http.ResponseWriter, r *http.Request) {
		paths := csvField(body(r), "paths")
		if len(paths) == 0 {
			a.fail(w, paramErr("缺少 paths 参数（数组或逗号分隔）"))
			return
		}
		args := append([]string{"file", "team", "trash-restore"}, paths...)
		a.execInto(w, withYes(args), true)
	}))
	mux.HandleFunc("POST /api/file/team/trash-clear", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		vol, td := normalizeVol(str(m, "vol")), str(m, "teamdir")
		if vol == "" || td == "" {
			a.fail(w, paramErr("缺少 vol 或 teamdir 参数"))
			return
		}
		if err := needConfirm(m, "清空团队空间回收站"); err != nil {
			a.fail(w, err)
			return
		}
		a.execInto(w, withYes([]string{"file", "team", "trash-clear", vol, td}), true)
	}))

	// 从 NAS 本地上传（CLI 只接受 NAS 上已存在的文件路径）==========
	mux.HandleFunc("POST /api/file/upload", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		dir, local := str(m, "remote_dir"), str(m, "local_file")
		if dir == "" || local == "" {
			a.fail(w, paramErr("缺少 remote_dir 或 local_file 参数（local_file 必须是 NAS 上已存在的路径）"))
			return
		}
		args := []string{"file", "upload", dir, local}
		if v := str(m, "overwrite"); v != "" {
			ov, ok := normalizeOverwrite(v)
			if !ok {
				a.fail(w, paramErr("overwrite 只能是 replace / skip / rename"))
				return
			}
			args = append(args, "--overwrite", ov)
		}
		a.execInto(w, withYes(args), true)
	}))

	// 生成下载链接 / 取消任务 / 上传预检
	mux.HandleFunc("GET /api/file/download-url", a.auth(func(w http.ResponseWriter, r *http.Request) {
		p := q(r, "path", "")
		if p == "" {
			a.fail(w, paramErr("缺少 path 参数"))
			return
		}
		data, err := a.cli.Run([]string{"file", "download-url", p}, false)
		if err != nil {
			a.fail(w, err)
			return
		}
		// 上游对不存在的路径同样回 result:succ，错误码藏在 download[].errno 里
		if code, allBad := listErrno(data, "download"); allBad {
			a.fail(w, &CLIError{
				Message: "无法生成下载链接（路径不存在或为目录）：" + p + "（errno " + anyToStr(code) + "）",
				ErrNo:   code, Status: httpStatusNotFound,
			})
			return
		}
		a.ok(w, data)
	}))
	mux.HandleFunc("POST /api/file/cancel", a.auth(func(w http.ResponseWriter, r *http.Request) {
		id := str(body(r), "task_id")
		if id == "" {
			a.fail(w, paramErr("缺少 task_id 参数"))
			return
		}
		a.execInto(w, withYes([]string{"file", "cancel", id}), true)
	}))
	mux.HandleFunc("POST /api/file/check-upload", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		p, size := str(m, "path"), anyToStr(m["size"])
		if p == "" || size == "" {
			a.fail(w, paramErr("缺少 path 或 size 参数"))
			return
		}
		args := []string{"file", "check-upload", p, size}
		if v := anyToStr(m["overwrite"]); v != "" {
			ov, ok := normalizeOverwrite(v)
			if !ok {
				a.fail(w, paramErr("overwrite 只能是 replace / skip / rename"))
				return
			}
			args = append(args, "--overwrite", ov)
		}
		a.execInto(w, withYes(args), true)
	}))

	// TrimACL 转换 / 报告
	mux.HandleFunc("POST /api/file/trimacl/conv", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		vol := volNumber(anyToStr(m["vol"]))
		if vol == "" {
			a.fail(w, paramErr("缺少 vol 参数（卷号，如 1）"))
			return
		}
		args := []string{"file", "trimacl", "conv", vol}
		if v := str(m, "csv_path"); v != "" {
			args = append(args, "--csv-path", v)
		}
		a.execInto(w, withYes(args), true)
	}))
	mux.HandleFunc("GET /api/file/trimacl/report", a.auth(func(w http.ResponseWriter, r *http.Request) {
		vol, csv := volNumber(q(r, "vol", "")), q(r, "csv_path", "")
		if vol == "" || csv == "" {
			a.fail(w, paramErr("缺少 vol 或 csv_path 参数"))
			return
		}
		a.execInto(w, withYes([]string{"file", "trimacl", "report", vol, csv}), true)
	}))

	// ===================== 存储（高风险，需 confirm） =====================

	mux.HandleFunc("POST /api/storage/disk-mount", a.auth(func(w http.ResponseWriter, r *http.Request) {
		d := str(body(r), "disk")
		if d == "" {
			a.fail(w, paramErr("缺少 disk 参数"))
			return
		}
		a.execInto(w, withYes([]string{"storage", "disk-mount", d}), true)
	}))
	mux.HandleFunc("POST /api/storage/disk-umount", a.auth(func(w http.ResponseWriter, r *http.Request) {
		d := str(body(r), "disk")
		if d == "" {
			a.fail(w, paramErr("缺少 disk 参数"))
			return
		}
		a.execInto(w, withYes([]string{"storage", "disk-umount", d}), true)
	}))

	// 新建存储池
	mux.HandleFunc("POST /api/storage/create", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		level := anyToStr(m["level"])
		if level == "" {
			a.fail(w, paramErr("缺少 level 参数（RAID 级别，如 1 / 5 / single）"))
			return
		}
		if err := needConfirm(m, "新建存储池"); err != nil {
			a.fail(w, err)
			return
		}
		args := []string{"storage", "create", "--level", level}
		if v := strings.Join(csvField(m, "disks"), ","); v != "" {
			args = append(args, "--disks", v)
		}
		if v := str(m, "comment"); v != "" {
			args = append(args, "--comment", v)
		}
		if v := str(m, "fstype"); v != "" {
			args = append(args, "--fstype", v)
		}
		if boolean(m, "check_disk") {
			args = append(args, "--check-disk")
		}
		a.execInto(w, withYes(args), true)
	}))

	// 加盘 / 移盘 / 换盘 / 扩容
	for _, act := range []string{"add-disk", "remove-disk"} {
		action := act
		label := map[string]string{"add-disk": "给存储池加盘", "remove-disk": "从存储池移除磁盘"}[action]
		mux.HandleFunc("POST /api/storage/"+action, a.auth(func(w http.ResponseWriter, r *http.Request) {
			m := body(r)
			uuid := str(m, "uuid")
			disks := csvField(m, "disks")
			if uuid == "" || len(disks) == 0 {
				a.fail(w, paramErr("缺少 uuid 或 disks 参数"))
				return
			}
			if err := needConfirm(m, label); err != nil {
				a.fail(w, err)
				return
			}
			args := []string{"storage", action, uuid, "--disks", strings.Join(disks, ",")}
			if v := str(m, "password"); v != "" {
				args = append(args, "--password", v)
			}
			a.execInto(w, withYes(args), true)
		}))
	}
	mux.HandleFunc("POST /api/storage/replace-disk", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		uuid := str(m, "uuid")
		disk, nd := strings.Join(csvField(m, "disks"), ","), strings.Join(csvField(m, "new_disks"), ",")
		if uuid == "" || disk == "" || nd == "" {
			a.fail(w, paramErr("缺少 uuid / disks / new_disks 参数"))
			return
		}
		if err := needConfirm(m, "替换磁盘"); err != nil {
			a.fail(w, err)
			return
		}
		args := []string{"storage", "replace-disk", uuid, "--disks", disk, "--new-disks", nd}
		if v := str(m, "password"); v != "" {
			args = append(args, "--password", v)
		}
		a.execInto(w, withYes(args), true)
	}))
	mux.HandleFunc("POST /api/storage/resize", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		uuid := str(m, "uuid")
		if uuid == "" {
			a.fail(w, paramErr("缺少 uuid 参数"))
			return
		}
		if err := needConfirm(m, "扩容存储池"); err != nil {
			a.fail(w, err)
			return
		}
		args := []string{"storage", "resize", uuid}
		for _, opt := range []struct{ key, flag string }{
			{"disks", "--disks"}, {"vd_name", "--vd-name"}, {"level", "--level"}, {"password", "--password"},
		} {
			v := str(m, opt.key)
			if opt.key == "disks" {
				v = strings.Join(csvField(m, "disks"), ",")
			}
			if v == "" {
				v = anyToStr(m[opt.key])
			}
			if v != "" {
				args = append(args, opt.flag, v)
			}
		}
		a.execInto(w, withYes(args), true)
	}))

	// ===================== Docker =====================

	mux.HandleFunc("POST /api/docker/image/rm", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		ref := str(m, "ref")
		if ref == "" {
			a.fail(w, paramErr("缺少 ref 参数"))
			return
		}
		args := []string{"docker", "image", "rm", ref}
		if boolean(m, "force") {
			args = append(args, "--force")
		}
		a.execInto(w, withYes(args), true)
	}))
	mux.HandleFunc("POST /api/docker/container/create", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		img := str(m, "image")
		if img == "" {
			a.fail(w, paramErr("缺少 image 参数"))
			return
		}
		args := []string{"docker", "container", "create", "--image", img}
		if v := str(m, "name"); v != "" {
			args = append(args, "--name", v)
		}
		if boolean(m, "start") {
			args = append(args, "--start")
		}
		if v := str(m, "restart"); v != "" {
			args = append(args, "--restart", v)
		}
		if v := anyToStr(m["memory"]); v != "" {
			args = append(args, "--memory", v)
		}
		a.execInto(w, args, true) // create 不需要 --yes（无该开关），失败会走错误分支
	}))
	mux.HandleFunc("POST /api/docker/container/rm", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		id := str(m, "id")
		if id == "" {
			a.fail(w, paramErr("缺少 id 参数"))
			return
		}
		args := []string{"docker", "container", "rm", id}
		if boolean(m, "force") {
			args = append(args, "--force")
		}
		a.execInto(w, withYes(args), true)
	}))

	// ===================== 日志 =====================

	mux.HandleFunc("POST /api/logger/clear", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		level, module := anyToStr(m["level"]), anyToStr(m["module"])
		if level == "" || module == "" {
			a.fail(w, paramErr("缺少 level 或 module 参数"))
			return
		}
		if err := needConfirm(m, "清空日志"); err != nil {
			a.fail(w, err)
			return
		}
		a.execInto(w, withYes([]string{"logger", "clear", "--level", level, "--module", module}), true)
	}))
	mux.HandleFunc("POST /api/logger/debuglog/start", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		out := str(m, "output_dir")
		if out == "" {
			a.fail(w, paramErr("缺少 output_dir 参数"))
			return
		}
		args := []string{"logger", "debuglog", "copy-start", "--output-dir", out}
		if v := str(m, "srv_type"); v != "" {
			// 上游只接受 0/1/2，提前挡住其它值
			if v != "0" && v != "1" && v != "2" {
				a.fail(w, paramErr("srv_type 只能是 0 / 1 / 2"))
				return
			}
			args = append(args, "--srv-type", v)
		}
		a.execInto(w, withYes(args), true)
	}))
	mux.HandleFunc("POST /api/logger/debuglog/stop", a.auth(func(w http.ResponseWriter, r *http.Request) {
		id := str(body(r), "task_id")
		if id == "" {
			a.fail(w, paramErr("缺少 task_id 参数"))
			return
		}
		a.execInto(w, withYes([]string{"logger", "debuglog", "copy-stop", "--task-id", id}), true)
	}))

	// ===================== 下载 =====================

	mux.HandleFunc("POST /api/download/add-path", a.auth(func(w http.ResponseWriter, r *http.Request) {
		m := body(r)
		p, dir := str(m, "path"), str(m, "save_dir")
		if p == "" || dir == "" {
			a.fail(w, paramErr("缺少 path 或 save_dir 参数"))
			return
		}
		data, err := a.cli.Run([]string{"download", "add-path", p, dir, "--yes"}, true)
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
}

// listErrno 解析 data[key] 数组里的 errno：全部非 0 时返回 (errno, true)。
// 上游很多接口把错误塞在数组元素里而不是顶层，必须提出来。
func listErrno(data any, key string) (int, bool) {
	obj, ok := data.(map[string]any)
	if !ok {
		return 0, false
	}
	list, ok := obj[key].([]any)
	if !ok || len(list) == 0 {
		return 0, false
	}
	first := 0
	for i, it := range list {
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
