//go:build !linux

package main

import (
	"errors"
	"io"
	"os/exec"
)

// 非 Linux 平台不提供原生伪终端，登录流程会回退到 script(1)
func startWithPTY(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	return nil, errors.New("当前平台不支持原生伪终端")
}

func canOpenPTY() error {
	return errors.New("当前平台不支持原生伪终端")
}
