package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	appName    = "fnos-api-gw"
	appVersion = "1.5.3"
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[fnos-api-gw] ")

	dataDir := envOr("DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		log.Fatalf("无法创建数据目录 %s: %v", dataDir, err)
	}

	cfgStore, err := newConfigStore(dataDir)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	bin := envOr("TRIM_CLI_BIN", "")
	if bin == "" {
		// 与二进制同目录优先
		self, _ := os.Executable()
		cand := filepath.Join(filepath.Dir(self), "trim-cli")
		if _, err := os.Stat(cand); err == nil {
			bin = cand
		} else {
			bin = "trim-cli"
		}
	}

	cli := &TrimCLI{
		Bin:       bin,
		ConfigDir: filepath.Join(dataDir, "trim-cli"),
		Store:     cfgStore,
	}
	if err := os.MkdirAll(cli.ConfigDir, 0o750); err != nil {
		log.Fatalf("无法创建 session 目录: %v", err)
	}

	app := &App{cfg: cfgStore, cli: cli, logs: newReqLogger(dataDir, 1000)}

	port := envOr("PORT", "9932")
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatalf("PORT 非法: %s", port)
	}

	srv := &http.Server{
		Addr:              "0.0.0.0:" + port,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second, // 含请求体读取，慢链路也不会无限挂着
		WriteTimeout:      360 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		c := cfgStore.Get()
		log.Printf("启动: port=%s trim-cli=%s target=%s://%s:%s profile=%s",
			port, bin, c.NasScheme, c.NasHost, c.NasPort, c.Profile)
		if _, err := os.Stat(bin); err != nil && filepath.IsAbs(bin) {
			log.Printf("警告: trim-cli 不存在于 %s", bin)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("监听失败: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "关闭异常: %v\n", err)
	}
	log.Println("已停止")
}
