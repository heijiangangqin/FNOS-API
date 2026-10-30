package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// TrimCLI 封装对 trim-cli 二进制的调用
type TrimCLI struct {
	Bin       string
	ConfigDir string // TRIM_CLI_CONFIG_DIR，session 落这里
	Store     *configStore
}

// CLIError 是给前端返回的统一错误结构
type CLIError struct {
	Message string `json:"error"`
	ErrMsg  string `json:"errmsg,omitempty"`
	ErrNo   any    `json:"errno,omitempty"`
	Stderr  string `json:"stderr,omitempty"`
	Timeout bool   `json:"timeout,omitempty"`
	Status  int    `json:"-"` // 非 0 时覆盖默认的 500
}

func (e *CLIError) Error() string {
	if e.ErrMsg != "" {
		return e.ErrMsg
	}
	return e.Message
}

func isLoopback(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return strings.HasPrefix(host, "127.")
}

// baseArgs 构造全局连接参数，必须排在子命令之前
func (t *TrimCLI) baseArgs() []string {
	c := t.Store.Get()
	args := []string{
		"--host", c.NasHost,
		"--port", c.NasPort,
		"--scheme", c.NasScheme,
		"--profile", c.Profile,
	}
	if c.AllowInsecureHTTP && c.NasScheme == "http" {
		args = append(args, "--allow-insecure-http")
	}
	if c.TLSInsecure {
		args = append(args, "--tls-insecure")
	}
	return args
}

func (t *TrimCLI) env() []string {
	env := os.Environ()
	env = append(env,
		"TRIM_CLI_CONFIG_DIR="+t.ConfigDir,
		"TRIM_CLI_SESSION_STORAGE=file",
		"HOME="+filepathDir(t.ConfigDir),
	)
	return env
}

func filepathDir(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i > 0 {
		return p[:i]
	}
	return p
}

// validateArgs 拒绝控制字符、空值与超长参数，防止参数注入与下游解析异常
func validateArgs(args []string) error {
	if len(args) == 0 {
		return errors.New("args 不能为空")
	}
	if len(args) > 40 {
		return errors.New("参数数量超出限制")
	}
	for _, a := range args {
		if a == "" {
			return errors.New("参数不能为空字符串")
		}
		if len(a) > 1024 {
			return errors.New("单个参数长度超出限制")
		}
		for _, r := range a {
			if r == 0 || unicode.IsControl(r) {
				return errors.New("参数包含控制字符")
			}
		}
	}
	return nil
}

// Run 执行 trim-cli，成功时返回解析后的 JSON（非 JSON 则以字符串返回）
func (t *TrimCLI) Run(args []string, long bool) (any, error) {
	tc := t.Store.Get()
	timeout := time.Duration(tc.RequestTimeoutSec) * time.Second
	if long {
		timeout = time.Duration(tc.LongTimeoutSec) * time.Second
	}
	return t.runWithTimeout(args, timeout)
}

func (t *TrimCLI) runWithTimeout(args []string, timeout time.Duration) (any, error) {
	full := append(t.baseArgs(), args...)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.Bin, full...)
	cmd.Env = t.env()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, &CLIError{Message: "执行超时", Timeout: true, Stderr: strings.TrimSpace(stderr.String())}
	}
	if err != nil {
		return nil, buildCLIError(err, stdout.String(), stderr.String())
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return map[string]any{}, nil
	}
	var parsed any
	if json.Unmarshal([]byte(out), &parsed) == nil {
		return parsed, nil
	}
	// 多包 JSON 流（部分命令逐行输出）
	if lines := splitJSONLines(out); len(lines) > 0 {
		return lines, nil
	}
	return out, nil
}

func splitJSONLines(out string) []any {
	var res []any
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var v any
		if json.Unmarshal([]byte(ln), &v) != nil {
			return nil
		}
		res = append(res, v)
	}
	return res
}

func buildCLIError(err error, stdout, stderr string) *CLIError {
	ce := &CLIError{Message: err.Error(), Stderr: strings.TrimSpace(stderr)}
	payload := strings.TrimSpace(stderr)
	if payload == "" {
		payload = strings.TrimSpace(stdout)
	}
	var obj map[string]any
	if json.Unmarshal([]byte(payload), &obj) == nil {
		if v, ok := obj["errmsg"].(string); ok {
			ce.ErrMsg = v
		}
		if v, ok := obj["errno"]; ok {
			ce.ErrNo = v
		}
		if v, ok := obj["code"]; ok && ce.ErrNo == nil {
			ce.ErrNo = v
		}
	}
	if ce.ErrNo == nil {
		if m := errnoRe.FindStringSubmatch(payload); len(m) == 2 {
			if n, err := strconv.Atoi(m[1]); err == nil {
				ce.ErrNo = n
			}
		}
	}
	if ce.ErrMsg == "" && payload != "" && len(payload) < 600 {
		ce.ErrMsg = firstLine(payload)
	}
	ce.Status = classifyStderr(payload)
	if ce.Status == httpStatusNotFound && ce.Message == "exit status 1" {
		ce.Message = "路径不存在或不可访问"
	}
	if ce.Status == httpStatusBadRequest && strings.Contains(payload, "errno 327848") {
		ce.Message = "permset（ACL 权限集）非法，共享未创建"
	}
	if ce.Status == httpStatusNotFound && strings.Contains(payload, "errno 131073") {
		ce.Message = "用户不存在"
	}
	if ce.Status == httpStatusConflict {
		switch {
		case strings.Contains(payload, "errno 33555719"):
			ce.Message = "SSH 已经是目标状态，无需重复操作"
		case strings.Contains(payload, "downloadcenter.task."):
			ce.Message = "下载任务当前状态不支持该操作（可能已完成或已在目标状态）"
		}
	}
	if strings.Contains(payload, "setAcl2 failed with result fail") {
		ce.Status = httpStatusBadRequest
		ce.Message = "该路径所在卷使用 trimacl 类型，不支持 POSIX ACL 设置（可用 /api/file/trimacl/* 处理）"
	}
	return ce
}

// classifyStderr 按错误内容判断合适的 HTTP 状态码。
// 目的是区分「调用方写错了」和「NAS 侧失败了」，前者应该是 4xx。
func classifyStderr(payload string) int {
	t := strings.TrimSpace(payload)
	low := strings.ToLower(t)
	switch {
	case strings.Contains(t, "errno 4100"):
		return httpStatusNotFound
	case strings.Contains(t, "errno 327848"):
		return httpStatusBadRequest // permset / ACL 非法，属调用方参数问题
	case strings.Contains(t, "request user.info failed with errno 131073"):
		return httpStatusNotFound // 用户不存在
	case strings.Contains(t, "errno 33555719"):
		return httpStatusConflict // SSH 已经是目标状态
	case strings.Contains(t, "downloadcenter.task.") && strings.Contains(t, "errno 400"):
		return httpStatusConflict // 下载任务当前状态不支持该操作
	case strings.Contains(t, "403 Forbidden") || strings.Contains(low, "permission denied"):
		return httpStatusForbidden
	case strings.Contains(t, "HTTP 502"):
		return httpStatusBadGateway
	case strings.Contains(t, "HTTP 503"):
		return httpStatusUnavailable
	case strings.Contains(t, "HTTP 504"):
		return httpStatusTimeout
	case strings.Contains(low, "saved session") ||
		strings.Contains(low, "run login first") ||
		strings.Contains(low, "session is incompatible") ||
		strings.Contains(low, "not logged in"):
		return httpStatusPreconditionRequired
	case strings.HasPrefix(t, "error: "): // clap 用法错误（缺参数、未知选项等）
		return httpStatusBadRequest
	case strings.HasPrefix(t, "Error: "):
		// 这些是真正打到了 NAS 业务接口后失败
		for _, up := range []string{"failed to get", "failed to call HTTP proxy",
			"request appcgi.", "request file.", "request user.", "request storage.",
			"request download.", "request appc.", "request dockermgr."} {
			if strings.Contains(t, up) {
				return 0 // 交给默认的 500
			}
		}
		return httpStatusBadRequest
	}
	return 0
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// LoginStatus 通过一次轻量只读调用判断 session 是否可用
func (t *TrimCLI) LoginStatus() (bool, string) {
	_, err := t.runWithTimeout([]string{"system", "info"}, 12*time.Second)
	if err == nil {
		return true, "ok"
	}
	var ce *CLIError
	if errors.As(err, &ce) {
		msg := ce.Error()
		if strings.Contains(strings.ToLower(msg), "login") ||
			strings.Contains(msg, "401") ||
			strings.Contains(msg, "session") ||
			strings.Contains(msg, "unauthorized") ||
			strings.Contains(msg, "授权") {
			return false, msg
		}
		return false, msg
	}
	return false, fmt.Sprint(err)
}

// RunPTY 在伪终端里执行命令。有些命令（如 user change-password）会检查
// 标准输入是不是终端，普通管道会直接报 "IO error: not a terminal"。
func (t *TrimCLI) RunPTY(args []string, long bool) (any, error) {
	tc := t.Store.Get()
	timeout := time.Duration(tc.RequestTimeoutSec) * time.Second
	if long {
		timeout = time.Duration(tc.LongTimeoutSec) * time.Second
	}
	if timeout > 45*time.Second {
		timeout = 45 * time.Second // 交互式命令不该长时间挂着
	}
	full := append(t.baseArgs(), args...)
	cmd := exec.Command(t.Bin, full...)
	cmd.Env = append(t.env(), "TERM=xterm-256color")

	master, err := startWithPTY(cmd)
	if err != nil {
		// 拿不到伪终端就退回普通执行，至少把上游的真实错误暴露出来
		return t.runWithTimeout(args, timeout)
	}

	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() { _, _ = io.Copy(&buf, master) }()
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(timeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return nil, &CLIError{Message: "执行超时", Timeout: true}
	case werr := <-done:
		_ = master.Close()
		out := strings.TrimSpace(cleanTerminal(buf.String()))
		if werr != nil {
			return nil, buildCLIError(werr, out, out)
		}
		if out == "" {
			return map[string]any{}, nil
		}
		var parsed any
		if json.Unmarshal([]byte(out), &parsed) == nil {
			return parsed, nil
		}
		if lines := splitJSONLines(out); len(lines) > 0 {
			return lines, nil
		}
		return out, nil
	}
}
