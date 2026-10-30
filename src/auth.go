package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// safeBuf 是并发安全的输出缓冲
type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

var (
	errnoRe = regexp.MustCompile(`errno\s+(\d+)`)
	urlRe   = regexp.MustCompile(`https?://[^\s"'<>]+`)
	ansiRe  = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")
)

func cleanTerminal(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// LoginSession 保持一个等待授权码的 trim-cli 进程
type LoginSession struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     *safeBuf
	done    chan error
	url     string
	method  string // pty / script
	started time.Time
}

var (
	loginMu      sync.Mutex
	loginCurrent *LoginSession
)

// shellQuote 用单引号包裹参数，避免 shell 解释（仅在 script 回退路径使用）
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (t *TrimCLI) loginArgs() []string {
	return append(t.baseArgs(), "login", "--no-open")
}

// findScript 依次在 PATH 与常见绝对路径里查找 script(1)。
// 服务进程的 PATH 往往很精简，所以不能只靠 LookPath。
func findScript() string {
	if p, err := exec.LookPath("script"); err == nil {
		return p
	}
	for _, p := range []string{
		"/usr/bin/script", "/bin/script", "/usr/sbin/script", "/sbin/script",
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// spawnLogin 优先用原生伪终端；拿不到再退回 script(1)
func (t *TrimCLI) spawnLogin() (*LoginSession, error) {
	out := &safeBuf{}

	// 路径 1：进程内 openpty，不依赖任何外部命令
	cmd := exec.Command(t.Bin, t.loginArgs()...)
	cmd.Env = append(t.env(), "TERM=xterm-256color")
	if master, err := startWithPTY(cmd); err == nil {
		s := &LoginSession{
			cmd: cmd, in: master, out: out,
			done: make(chan error, 1), method: "pty", started: time.Now(),
		}
		go func() { _, _ = io.Copy(out, master) }()
		go func() { s.done <- cmd.Wait() }()
		return s, nil
	}

	// 路径 2：用 script(1) 包一层伪终端
	sp := findScript()
	if sp == "" {
		return nil, errors.New("无法创建伪终端：系统既没有可用的 /dev/ptmx，也没有 script 命令。请在 SSH 中手动执行登录（应用内「备用方式」可生成命令）")
	}

	inner := shellQuote(t.Bin)
	for _, a := range t.loginArgs() {
		inner += " " + shellQuote(a)
	}
	cmd2 := exec.Command(sp, "-q", "-e", "-f", "-c", inner, "/dev/null")
	cmd2.Env = append(t.env(), "TERM=xterm-256color")

	stdin, err := cmd2.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd2.Stdout = out
	cmd2.Stderr = out
	if err := cmd2.Start(); err != nil {
		return nil, err
	}

	s := &LoginSession{
		cmd: cmd2, in: stdin, out: out,
		done: make(chan error, 1), method: "script", started: time.Now(),
	}
	go func() { s.done <- cmd2.Wait() }()
	return s, nil
}

// StartLogin 启动等待授权码的登录进程，返回授权链接
func (t *TrimCLI) StartLogin() (string, error) {
	loginMu.Lock()
	defer loginMu.Unlock()

	if loginCurrent != nil {
		select {
		case <-loginCurrent.done:
			loginCurrent = nil
		default:
			if loginCurrent.url != "" && time.Since(loginCurrent.started) < 4*time.Minute {
				return loginCurrent.url, nil
			}
			loginCurrent.kill()
			loginCurrent = nil
		}
	}

	sess, err := t.spawnLogin()
	if err != nil {
		return "", err
	}

	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		text := cleanTerminal(sess.out.String())
		if u := urlRe.FindString(text); u != "" &&
			(strings.Contains(u, "/signin") || strings.Contains(u, "oauth") ||
				strings.Contains(u, "code_challenge")) {
			sess.url = u
			loginCurrent = sess
			return u, nil
		}
		select {
		case werr := <-sess.done:
			sess.done <- werr // 放回，供后续判断
			tail := text
			if len(tail) > 500 {
				tail = tail[len(tail)-500:]
			}
			return "", fmt.Errorf("登录进程提前退出（%s）：%s", sess.method, strings.TrimSpace(tail))
		case <-time.After(250 * time.Millisecond):
		}
	}

	sess.kill()
	return "", errors.New("等待授权链接超时，请检查 NAS 地址与端口配置")
}

func (s *LoginSession) kill() {
	if s.in != nil {
		_ = s.in.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
}

// SubmitCode 把用户拿到的授权码写入伪终端，等待进程结束
func (t *TrimCLI) SubmitCode(code string) (string, error) {
	loginMu.Lock()
	sess := loginCurrent
	loginMu.Unlock()

	if sess == nil {
		return "", errors.New("没有等待中的登录会话，请先获取授权链接")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return "", errors.New("授权码不能为空")
	}
	if len(code) > 8192 {
		return "", errors.New("授权码长度异常")
	}
	for _, r := range code {
		if r == '\n' || r == '\r' || r == 0 {
			return "", errors.New("授权码包含非法字符")
		}
	}

	if _, err := io.WriteString(sess.in, code+"\n"); err != nil {
		return "", fmt.Errorf("写入授权码失败：%v", err)
	}

	var err error
	select {
	case err = <-sess.done:
	case <-time.After(90 * time.Second):
		sess.kill()
		return "", errors.New("等待授权结果超时")
	}

	loginMu.Lock()
	if loginCurrent == sess {
		loginCurrent = nil
	}
	loginMu.Unlock()

	text := cleanTerminal(sess.out.String())
	if err != nil {
		return "", fmt.Errorf("授权失败：%s", lastMeaningful(text))
	}
	if ok, _ := t.LoginStatus(); !ok {
		return "", fmt.Errorf("授权流程已结束但 session 不可用：%s", lastMeaningful(text))
	}
	return "登录成功（通道：" + sess.method + "）", nil
}

func lastMeaningful(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		ln := strings.TrimSpace(lines[i])
		if ln != "" {
			return ln
		}
	}
	return ""
}

// LoginDiagnose 汇总登录链路的自检信息，便于远程定位
func (t *TrimCLI) LoginDiagnose() map[string]any {
	p := os.Getenv("PATH")
	if len(p) > 400 {
		p = p[:400] + "...(truncated)"
	}
	info := map[string]any{
		"trim_cli":    t.Bin,
		"session_dir": t.ConfigDir,
		"script":      findScript(),
		"path":        p,
	}

	if _, err := os.Stat(t.Bin); err != nil {
		info["trim_cli_exists"] = false
		info["trim_cli_error"] = err.Error()
	} else {
		info["trim_cli_exists"] = true
	}

	if err := canOpenPTY(); err != nil {
		info["pty"] = "不可用: " + err.Error()
	} else {
		info["pty"] = "可用"
	}

	// 真实写一次，确认 session 目录可用
	probe := filepath.Join(t.ConfigDir, ".write-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		info["session_dir_writable"] = false
		info["session_dir_error"] = err.Error()
	} else {
		info["session_dir_writable"] = true
		_ = os.Remove(probe)
	}
	if entries, err := os.ReadDir(t.ConfigDir); err == nil {
		info["session_files"] = len(entries)
	}
	return info
}
