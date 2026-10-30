package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Config 是应用的持久配置，落在 ${TRIM_PKGVAR}/data/config.json
type Config struct {
	APIToken          string `json:"api_token"`
	NasHost           string `json:"nas_host"`
	NasPort           string `json:"nas_port"`
	NasScheme         string `json:"nas_scheme"`
	AllowInsecureHTTP bool   `json:"allow_insecure_http"`
	TLSInsecure       bool   `json:"tls_insecure"`
	AllowPassthrough  bool   `json:"allow_passthrough"`
	AllowMedia        bool   `json:"allow_media"`
	RequestTimeoutSec int    `json:"request_timeout_sec"`
	LongTimeoutSec    int    `json:"long_timeout_sec"`
	Profile           string `json:"profile"`
	// IPAllowlist 是来源 IP 白名单，支持单个 IP 或 CIDR 网段。
	// 空数组表示不限制（默认关闭）。
	IPAllowlist []string `json:"ip_allowlist"`
}

type configStore struct {
	mu         sync.RWMutex
	path       string
	cfg        Config
	needsSetup bool // 令牌尚未设置（首次运行，或安装向导没写入有效值）
}

// placeholderToken 是安装模板里的占位符。若安装器没做替换，它会被原样写进配置，
// 这种情况必须当作"未设置"处理，否则用户永远拿不到能用的令牌。
const placeholderToken = "WIZARD_API_TOKEN"

func defaultConfig() Config {
	return Config{
		NasHost:           "localhost",
		NasPort:           "5666",
		NasScheme:         "http",
		AllowInsecureHTTP: true,
		AllowPassthrough:  true,
		AllowMedia:        false,
		RequestTimeoutSec: 60,
		LongTimeoutSec:    300,
		Profile:           "default",
		IPAllowlist:       []string{},
	}
}

func newConfigStore(dataDir string) (*configStore, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}
	cs := &configStore{path: filepath.Join(dataDir, "config.json"), cfg: defaultConfig()}

	if raw, err := os.ReadFile(cs.path); err == nil {
		// 从磁盘恢复，缺失字段保留默认值
		if err := json.Unmarshal(raw, &cs.cfg); err != nil {
			return nil, err
		}
	}
	cs.normalize()

	// 令牌缺失/是占位符/过短，都视为"待设置"，交给页面首次设置流程处理。
	// 这里不再偷偷生成随机值——那样用户永远不知道令牌是什么。
	if !validToken(cs.cfg.APIToken) {
		cs.needsSetup = true
	}
	if err := cs.save(); err != nil {
		return nil, err
	}
	return cs, nil
}

func validToken(t string) bool {
	return len(t) >= 8 && t != placeholderToken
}

// NeedsSetup 报告是否需要首次设置令牌
func (cs *configStore) NeedsSetup() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.needsSetup
}

// SetToken 设置访问令牌并落盘
func (cs *configStore) SetToken(token string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if !validToken(token) {
		return errors.New("令牌至少 8 位")
	}
	cs.cfg.APIToken = token
	cs.needsSetup = false
	return cs.save()
}

func (cs *configStore) normalize() {
	if cs.cfg.IPAllowlist == nil {
		cs.cfg.IPAllowlist = []string{}
	}
	d := defaultConfig()
	if cs.cfg.NasHost == "" {
		cs.cfg.NasHost = d.NasHost
	}
	if cs.cfg.NasPort == "" {
		cs.cfg.NasPort = d.NasPort
	}
	if cs.cfg.NasScheme == "" {
		cs.cfg.NasScheme = d.NasScheme
	}
	if cs.cfg.Profile == "" {
		cs.cfg.Profile = d.Profile
	}
	if cs.cfg.RequestTimeoutSec <= 0 {
		cs.cfg.RequestTimeoutSec = d.RequestTimeoutSec
	}
	if cs.cfg.LongTimeoutSec <= 0 {
		cs.cfg.LongTimeoutSec = d.LongTimeoutSec
	}
}

func (cs *configStore) Get() Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg
}

func (cs *configStore) Update(fn func(c *Config)) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	fn(&cs.cfg)
	cs.normalize()
	return cs.save()
}

func (cs *configStore) save() error {
	raw, err := json.MarshalIndent(cs.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := cs.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, cs.path)
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "please-change-me"
	}
	return hex.EncodeToString(b)
}
