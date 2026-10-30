package main

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed ui/index.html
var indexHTML []byte

func serveUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		// "GET /" 在 Go 1.22 的路由里会匹配所有 GET 路径，
		// 所以 /api/* 的未知路由会落到这里，必须回 JSON 而不是纯文本 404
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, httpStatusNotFound, apiResult{
				Success: false,
				Error:   "接口不存在",
				Hint:    "核对路径与方法：" + r.Method + " " + r.URL.Path,
			})
			return
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(indexHTML)
}
