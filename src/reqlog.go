package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// reqLog 是一条请求记录
type reqLog struct {
	TS     string `json:"ts"`              // 2026-10-27 11:46:19
	IP     string `json:"ip"`              // 来源 IP
	Method string `json:"method"`          //
	Path   string `json:"path"`            // /api/monitor/cpu
	Query  string `json:"query,omitempty"` // 去掉 token 后的查询串
	Status int    `json:"status"`          // HTTP 状态码
	MS     int64  `json:"ms"`              // 耗时毫秒
	OK     bool   `json:"ok"`              // 2xx/3xx 且 success!=false
	Err    string `json:"err,omitempty"`   // 失败时的简短原因
}

// reqLogger 环形缓冲 + JSONL 落盘
type reqLogger struct {
	mu      sync.RWMutex
	ring    []reqLog
	max     int
	path    string
	file    *os.File
	size    int64
	maxFile int64
}

func newReqLogger(dataDir string, max int) *reqLogger {
	l := &reqLogger{
		max:     max,
		path:    filepath.Join(dataDir, "requests.log"),
		maxFile: 5 << 20, // 单文件 5MB，超出后轮转成 .1
	}
	l.loadTail()
	l.openAppend()
	return l
}

// loadTail 启动时把历史日志的尾部读回内存，避免重启后页面空白
func (l *reqLogger) loadTail() {
	f, err := os.Open(l.path)
	if err != nil {
		return
	}
	defer f.Close()

	var all []reqLog
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e reqLog
		if json.Unmarshal([]byte(line), &e) == nil && e.Path != "" {
			all = append(all, e)
		}
	}
	if len(all) > l.max {
		all = all[len(all)-l.max:]
	}
	l.ring = all
}

func (l *reqLogger) openAppend() {
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	if st, err := f.Stat(); err == nil {
		l.size = st.Size()
	}
	l.file = f
}

// Add 记录一条日志
func (l *reqLogger) Add(e reqLog) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.ring = append(l.ring, e)
	if len(l.ring) > l.max {
		l.ring = l.ring[len(l.ring)-l.max:]
	}

	if l.file == nil {
		return
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	b = append(b, '\n')
	n, err := l.file.Write(b)
	if err != nil {
		return
	}
	l.size += int64(n)

	if l.size >= l.maxFile {
		l.rotateLocked()
	}
}

// rotateLocked 轮转日志文件，只保留上一份
func (l *reqLogger) rotateLocked() {
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
	_ = os.Rename(l.path, l.path+".1")
	l.size = 0
	l.openAppend()
}

// List 返回倒序（最新在前）的日志，可按关键字与"只看错误"过滤
func (l *reqLogger) List(limit int, onlyErr bool, q string) []reqLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > l.max {
		limit = l.max
	}
	q = strings.ToLower(strings.TrimSpace(q))
	out := make([]reqLog, 0, limit)
	for i := len(l.ring) - 1; i >= 0 && len(out) < limit; i-- {
		e := l.ring[i]
		if onlyErr && e.OK {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Path+" "+e.Err+" "+e.Method), q) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Clear 清空内存与文件
func (l *reqLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ring = nil
	if l.file != nil {
		_ = l.file.Truncate(0)
		_, _ = l.file.Seek(0, 0)
		l.size = 0
	}
}

// Stats 返回总条数与错误条数，供页面显示概览
func (l *reqLogger) Stats() (total, failed int) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	total = len(l.ring)
	for _, e := range l.ring {
		if !e.OK {
			failed++
		}
	}
	return
}

// ---------- 中间件 ----------

// logWriter 包一层响应写入器，记录状态码并缓存前 512 字节用于提取错误
type logWriter struct {
	http.ResponseWriter
	code int
	buf  []byte
}

func (w *logWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *logWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	if len(w.buf) < 512 {
		room := 512 - len(w.buf)
		if len(b) < room {
			room = len(b)
		}
		w.buf = append(w.buf, b[:room]...)
	}
	return w.ResponseWriter.Write(b)
}

// requestLogger 记录所有 /api/* 请求（静态页面与日志查询本身不记，避免噪音与自激）
func (a *App) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.logs == nil || !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/logs" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		lw := &logWriter{ResponseWriter: w}
		next.ServeHTTP(lw, r)

		code := lw.code
		if code == 0 {
			code = http.StatusOK
		}
		entry := reqLog{
			TS:     start.Format("2006-01-02 15:04:05"),
			IP:     ipString(r),
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  redactQuery(r.URL.RawQuery),
			Status: code,
			MS:     time.Since(start).Milliseconds(),
			OK:     code < 400,
		}
		if !entry.OK {
			entry.Err = extractErr(lw.buf)
		}
		a.logs.Add(entry)
	})
}

func ipString(r *http.Request) string {
	if ip := clientIP(r); ip != nil {
		return ip.String()
	}
	return ""
}

// redactQuery 去掉 token 参数，避免日志里出现明文令牌
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "&")
	out := parts[:0]
	for _, p := range parts {
		if strings.HasPrefix(p, "token=") {
			out = append(out, "token=***")
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, "&")
}

// extractErr 从响应体里提取一句可读的错误原因
func extractErr(buf []byte) string {
	if len(buf) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(buf, &m) == nil {
		errStr, _ := m["error"].(string)
		msgStr, _ := m["errmsg"].(string)
		// error 常常只是 "exit status 1"，这时优先用 errmsg 里的上游原文
		if isGenericErr(errStr) && msgStr != "" {
			return truncate(oneLine(msgStr), 120)
		}
		if errStr != "" {
			return truncate(oneLine(errStr), 120)
		}
		if msgStr != "" {
			return truncate(oneLine(msgStr), 120)
		}
	}
	return truncate(oneLine(string(buf)), 120)
}

func isGenericErr(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.HasPrefix(s, "exit status")
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
