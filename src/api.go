package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os/user"
	"strconv"
	"strings"
	"time"
)

// App 汇总 HTTP 层依赖
type App struct {
	cfg  *configStore
	cli  *TrimCLI
	logs *reqLogger
}

type apiResult struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
	ErrNo   any    `json:"errno,omitempty"`
	Stderr  string `json:"stderr,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *App) ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, apiResult{Success: true, Data: data})
}

func (a *App) fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	res := apiResult{Success: false}
	if ce, isCLI := err.(*CLIError); isCLI {
		if ce.Status != 0 {
			code = ce.Status
		}
		res.Error = ce.Message
		res.ErrMsg = ce.ErrMsg
		res.ErrNo = ce.ErrNo
		res.Stderr = truncate(ce.Stderr, 2000)
		if ce.Timeout {
			code = http.StatusGatewayTimeout
		}
		if strings.Contains(strings.ToLower(res.ErrMsg+" "+res.Error), "login") {
			res.Hint = "session 可能已失效，请在「登录」页重新授权"
		}
	} else {
		res.Error = err.Error()
	}
	writeJSON(w, code, res)
}

// 常用状态码（避免在 trimcli.go 里再导入一次 net/http）
const (
	httpStatusBadRequest           = 400
	httpStatusForbidden            = 403
	httpStatusNotFound             = 404
	httpStatusConflict             = 409
	httpStatusPreconditionRequired = 428
	httpStatusBadGateway           = 502
	httpStatusUnavailable          = 503
	httpStatusTimeout              = 504
)

// paramErr 表示调用方参数问题，返回 400 而不是 500
func paramErr(msg string) *CLIError {
	return &CLIError{Message: msg, Status: http.StatusBadRequest}
}

// forbid 表示被策略拦下，返回 403
func forbid(msg string) *CLIError {
	return &CLIError{Message: msg, Status: httpStatusForbidden}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

func (a *App) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.cfg.NeedsSetup() {
			writeJSON(w, http.StatusPreconditionRequired, apiResult{
				Success: false,
				Error:   "应用尚未设置访问令牌",
				Hint:    "请在页面上完成首次设置",
			})
			return
		}
		token := a.cfg.Get().APIToken
		got := r.Header.Get("Authorization")
		if got == "" {
			got = r.URL.Query().Get("token")
			if got != "" {
				got = "Bearer " + got
			}
		}
		if token != "" && got != "Bearer "+token {
			writeJSON(w, http.StatusUnauthorized, apiResult{
				Success: false,
				Error:   "令牌不匹配",
				Hint:    "令牌保存在应用数据目录的 data/config.json 里（api_token 字段）。忘记的话，删掉该文件并重启应用即可重新设置。",
			})
			return
		}
		next(w, r)
	}
}

// run 是静态路由的通用执行器
func (a *App) run(args []string, long bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := a.cli.Run(args, long)
		if err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, data)
	}
}

func q(r *http.Request, key string, def string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return def
}

func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()

	// ---------- 无需令牌 ----------
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		c := a.cfg.Get()
		a.ok(w, map[string]any{
			"app":      appName,
			"version":  appVersion,
			"nas_host": c.NasHost,
			"nas_port": c.NasPort,
			"scheme":   c.NasScheme,
		})
	})

	// ---------- 首次设置（无需令牌）----------
	mux.HandleFunc("GET /api/setup", func(w http.ResponseWriter, r *http.Request) {
		a.ok(w, map[string]any{
			"needs_setup": a.cfg.NeedsSetup(),
			"app":         appName,
			"version":     appVersion,
		})
	})
	mux.HandleFunc("POST /api/setup", func(w http.ResponseWriter, r *http.Request) {
		if !a.cfg.NeedsSetup() {
			writeJSON(w, http.StatusConflict, apiResult{
				Success: false,
				Error:   "已经设置过令牌，不能再通过该接口修改",
				Hint:    "如需重置，请删除应用数据目录下的 data/config.json 后重启应用",
			})
			return
		}
		var body struct {
			Token     string `json:"token"`
			NasHost   string `json:"nas_host"`
			NasPort   string `json:"nas_port"`
			NasScheme string `json:"nas_scheme"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		generated := false
		if strings.TrimSpace(body.Token) == "" {
			body.Token = randomToken()
			generated = true
		}
		body.Token = strings.TrimSpace(body.Token)
		if !validToken(body.Token) {
			a.fail(w, &CLIError{Message: "令牌至少 8 位，且不能使用安装模板占位符"})
			return
		}

		// 连接参数一并落盘，保证删掉 config.json 后能完整恢复
		if err := a.cfg.Update(func(c *Config) {
			if body.NasHost != "" {
				c.NasHost = body.NasHost
			}
			if body.NasPort != "" {
				c.NasPort = body.NasPort
			}
			if body.NasScheme == "http" || body.NasScheme == "https" {
				c.NasScheme = body.NasScheme
			}
		}); err != nil {
			a.fail(w, err)
			return
		}
		if err := a.cfg.SetToken(body.Token); err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, map[string]any{"token": body.Token, "generated": generated})
	})

	// ---------- 登录 ----------
	mux.HandleFunc("GET /api/auth/status", a.auth(func(w http.ResponseWriter, r *http.Request) {
		ok, msg := a.cli.LoginStatus()
		a.ok(w, map[string]any{"logged_in": ok, "message": msg})
	}))
	mux.HandleFunc("POST /api/auth/start", a.auth(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.cli.StartLogin()
		if err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, map[string]any{
			"auth_url": u,
			"hint":     "在浏览器打开该链接并完成授权，把页面显示的一次性 code 粘贴回来",
			"expires":  "约 5 分钟",
		})
	}))
	mux.HandleFunc("POST /api/auth/code", a.auth(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			a.fail(w, paramErr("请求体格式不对，args 必须是字符串数组："+err.Error()))
			return
		}
		msg, err := a.cli.SubmitCode(body.Code)
		if err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, map[string]any{"message": msg})
	}))
	mux.HandleFunc("POST /api/auth/logout", a.auth(func(w http.ResponseWriter, r *http.Request) {
		data, err := a.cli.Run([]string{"logout"}, false)
		if err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, data)
	}))
	// 登录链路自检
	mux.HandleFunc("GET /api/auth/diagnose", a.auth(func(w http.ResponseWriter, r *http.Request) {
		a.ok(w, a.cli.LoginDiagnose())
	}))
	// 兜底：给出可在 SSH 里手敲的登录命令（伪终端不可用时使用）
	mux.HandleFunc("GET /api/auth/manual", a.auth(func(w http.ResponseWriter, r *http.Request) {
		c := a.cfg.Get()
		cmd := "TRIM_CLI_CONFIG_DIR=" + a.cli.ConfigDir +
			" TRIM_CLI_SESSION_STORAGE=file " + a.cli.Bin +
			" --host " + c.NasHost + " --port " + c.NasPort +
			" --scheme " + c.NasScheme
		if c.AllowInsecureHTTP && c.NasScheme == "http" {
			cmd += " --allow-insecure-http"
		}
		cmd += " --profile " + c.Profile + " login"
		u := "fnosapigw"
		if cur, err := user.Current(); err == nil && cur.Username != "" {
			u = cur.Username
		}
		a.ok(w, map[string]any{
			"command":      cmd,
			"sudo_command": "sudo -u " + u + " " + cmd,
			"run_as":       u,
			"session_dir":  a.cli.ConfigDir,
			"hint":         "在 SSH 里用 root 执行 sudo_command；按提示打开链接、粘贴授权码即可。session 写入同一目录，完成后刷新本页。",
		})
	}))

	// ---------- 配置 ----------
	mux.HandleFunc("GET /api/config", a.auth(func(w http.ResponseWriter, r *http.Request) {
		c := a.cfg.Get()
		c.APIToken = "••••••••"
		// 转成 map 再补 client_ip，避免多出来的字段被写回配置文件
		b, _ := json.Marshal(c)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if ip := clientIP(r); ip != nil {
			m["client_ip"] = ip.String()
		}
		a.ok(w, m)
	}))
	mux.HandleFunc("POST /api/config", a.auth(func(w http.ResponseWriter, r *http.Request) {
		var patch map[string]any
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			a.fail(w, err)
			return
		}
		allow, hasAllow, aerr := parseAllowlist(patch)
		if aerr != nil {
			a.fail(w, paramErr(aerr.Error()))
			return
		}
		// 防呆：保存后如果连自己都进不来，直接拦下
		if hasAllow && len(allow) > 0 {
			if ip := clientIP(r); ip != nil && !ip.IsLoopback() && !ipAllowed(ip, allow) {
				a.fail(w, paramErr("保存后当前 IP（"+ip.String()+"）将无法访问，已阻止保存。"+
					"请先把自己加进白名单，或从飞牛桌面（本机）保存。"))
				return
			}
		}
		err := a.cfg.Update(func(c *Config) {
			if hasAllow {
				c.IPAllowlist = allow
			}
			if v, ok := patch["nas_host"].(string); ok && v != "" {
				c.NasHost = v
			}
			if v, ok := patch["nas_port"].(string); ok && v != "" {
				c.NasPort = v
			}
			if v, ok := patch["nas_scheme"].(string); ok && (v == "http" || v == "https") {
				c.NasScheme = v
			}
			if v, ok := patch["allow_insecure_http"].(bool); ok {
				c.AllowInsecureHTTP = v
			}
			if v, ok := patch["tls_insecure"].(bool); ok {
				c.TLSInsecure = v
			}
			if v, ok := patch["allow_passthrough"].(bool); ok {
				c.AllowPassthrough = v
			}
			if v, ok := patch["api_token"].(string); ok && len(v) >= 8 {
				c.APIToken = v
			}
		})
		if err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, map[string]any{"message": "已保存"})
	}))

	// ---------- 请求日志 ----------
	mux.HandleFunc("GET /api/logs", a.auth(func(w http.ResponseWriter, r *http.Request) {
		limit := 200
		if v := q(r, "limit", ""); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				limit = n
			}
		}
		onlyErr := q(r, "only_error", "") == "1" || q(r, "only_error", "") == "true"
		list := a.logs.List(limit, onlyErr, q(r, "q", ""))
		total, failed := a.logs.Stats()
		a.ok(w, map[string]any{
			"total":  total,
			"failed": failed,
			"items":  list,
		})
	}))
	mux.HandleFunc("POST /api/logs/clear", a.auth(func(w http.ResponseWriter, r *http.Request) {
		a.logs.Clear()
		a.ok(w, map[string]any{"message": "已清空"})
	}))

	// ---------- 通用命令（覆盖全部 trim-cli 命令）----------
	mux.HandleFunc("POST /api/cmd", a.auth(func(w http.ResponseWriter, r *http.Request) {
		if !a.cfg.Get().AllowPassthrough {
			a.fail(w, &CLIError{Message: "通用命令通道已关闭，请在设置中启用"})
			return
		}
		var body struct {
			Args []string `json:"args"`
			Long bool     `json:"long"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			a.fail(w, paramErr("请求体格式不对，args 必须是字符串数组："+err.Error()))
			return
		}
		if err := checkTopLevel(body.Args); err != nil {
			a.fail(w, err)
			return
		}
		if err := validateArgs(body.Args); err != nil {
			a.fail(w, paramErr(err.Error()))
			return
		}
		data, err := a.cli.Run(body.Args, body.Long)
		if err != nil {
			a.fail(w, err)
			return
		}
		a.ok(w, data)
	}))

	a.registerReadRoutes(mux)
	a.registerWriteRoutes(mux)
	a.registerExtraRoutes(mux)

	// 静态前端
	mux.HandleFunc("GET /", serveUI)

	// 兜底：/api/* 下未匹配的路径与方法统一返回 JSON 信封，而不是纯文本 404
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, httpStatusNotFound, apiResult{
				Success: false,
				Error:   "接口不存在",
				Hint:    "核对路径与方法：" + r.Method + " " + r.URL.Path,
			})
			return
		}
		http.NotFound(w, r)
	})

	return a.requestLogger(a.ipFilter(limitBody(retryTransientGET(mux))))
}

// checkTopLevel 拦截不允许通过通用通道执行的命令
func checkTopLevel(args []string) error {
	if len(args) == 0 {
		return paramErr("args 不能为空")
	}
	// 注意：必须扫描全部参数。之前遇到第一个子命令就 return，导致
	// ["system","info","--profile=x"] 这种把连接参数放在后面的写法能绕过检查。
	sawCmd := false
	for _, a := range args {
		for _, banned := range []string{
			"--host", "--port", "--scheme", "--profile",
			"--allow-insecure-http", "--tls-insecure",
		} {
			if a == banned || strings.HasPrefix(a, banned+"=") {
				return forbid("连接参数由应用配置管理，不能通过命令通道覆盖：" + banned)
			}
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		switch a {
		case "login", "logout":
			return forbid("登录/登出请使用 /api/auth/* 接口")
		case "media":
			return forbid("影音接口不在本应用的能力范围内")
		}
		sawCmd = true
	}
	if !sawCmd {
		return paramErr("args 缺少命令")
	}
	return nil
}

// limitBody 给请求体加上限，避免超大 JSON 把内存打满。
// 正常调用最多几 KB，1MB 足够。
func limitBody(next http.Handler) http.Handler {
	const maxBody = 1 << 20
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		}
		next.ServeHTTP(w, r)
	})
}

// ipFilter 按来源 IP 做白名单过滤。白名单为空表示不限制（默认）。
// 本机回环地址始终放行，避免在飞牛桌面里把自己锁在门外。
func (a *App) ipFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		list := a.cfg.Get().IPAllowlist
		if len(list) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		if ip == nil || ip.IsLoopback() || ipAllowed(ip, list) {
			next.ServeHTTP(w, r)
			return
		}
		writeJSON(w, httpStatusForbidden, apiResult{
			Success: false,
			Error:   "来源 IP 不在白名单内：" + ip.String(),
			Hint:    "在设置页「IP 白名单」里加入该地址，或清空白名单关闭限制",
		})
	})
}

// clientIP 取请求来源 IP（直连场景用 RemoteAddr，不信任可伪造的 XFF 头）
func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

// ipAllowed 判断 ip 是否命中白名单。条目可以是单个 IP，也可以是 CIDR 网段。
func ipAllowed(ip net.IP, list []string) bool {
	for _, raw := range list {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			if _, n, err := net.ParseCIDR(e); err == nil && n.Contains(ip) {
				return true
			}
			continue
		}
		if p := net.ParseIP(e); p != nil && p.Equal(ip) {
			return true
		}
	}
	return false
}

// validateAllowlist 校验并清洗白名单条目，非法项直接报错（不做静默丢弃）
func validateAllowlist(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, raw := range items {
		e := strings.TrimSpace(raw)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			_, _, err := net.ParseCIDR(e)
			if err != nil {
				return nil, fmt.Errorf("网段格式不对：%s（应形如 192.168.1.0/24）", e)
			}
		} else if net.ParseIP(e) == nil {
			return nil, fmt.Errorf("IP 格式不对：%s", e)
		}
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out, nil
}

// parseAllowlist 从请求体里取出白名单：支持数组，也支持换行/逗号分隔的字符串
func parseAllowlist(patch map[string]any) ([]string, bool, error) {
	v, ok := patch["ip_allowlist"]
	if !ok {
		return nil, false, nil
	}
	var items []string
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			if sv, ok := x.(string); ok {
				items = append(items, sv)
			}
		}
	case string:
		items = strings.FieldsFunc(t, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ' ' || r == '\t' || r == ';'
		})
	}
	cleaned, err := validateAllowlist(items)
	if err != nil {
		return nil, true, err
	}
	return cleaned, true, nil
}

// retryTransientGET 对只读请求做一次瞬时错误重试。
// 背景：fnOS 的 /ogh/ac/w 代理在重查询之后偶发连接失败（实测 file/usage 在
// file/search 拉回 5000+ 条之后会失败一次，单独重放即成功）。
// 只对 GET 生效——写操作绝不自动重试，避免副作用被执行两次。
func retryTransientGET(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		if rec.Code >= 500 && isTransientBody(rec.Body.String()) {
			time.Sleep(500 * time.Millisecond)
			rec = httptest.NewRecorder()
			next.ServeHTTP(rec, r)
		}

		for k, vs := range rec.Header() {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}

// isTransientBody 判断是否是底层代理的瞬时故障
func isTransientBody(body string) bool {
	for _, p := range []string{
		"failed to call HTTP proxy",
		"connection reset by peer",
		"connection refused",
		"i/o timeout",
	} {
		if strings.Contains(body, p) {
			return true
		}
	}
	return false
}
