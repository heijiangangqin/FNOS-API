//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// TIOCGPTN / TIOCSPTLCK 是 asm-generic 的 ioctl 编号，x86_64 与 arm64 一致
const (
	tiocgptn   = 0x80045430
	tiocsptlck = 0x40045431
)

// openPTYPair 打开一对伪终端：master 用于读写，slave 交给子进程当控制终端
func openPTYPair() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("打开 /dev/ptmx 失败: %w", err)
	}

	var unlock int32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(),
		tiocsptlck, uintptr(unsafe.Pointer(&unlock))); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("解锁 pty 失败: %v", errno)
	}

	var n uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(),
		tiocgptn, uintptr(unsafe.Pointer(&n))); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("获取 pty 编号失败: %v", errno)
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", n)
	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("打开 %s 失败: %w", slavePath, err)
	}
	return master, slave, nil
}

// startWithPTY 让 cmd 在伪终端里运行，返回的主设备句柄可读可写
func startWithPTY(cmd *exec.Cmd) (io.ReadWriteCloser, error) {
	master, slave, err := openPTYPair()
	if err != nil {
		return nil, err
	}

	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	// 新建会话并把 slave 设为控制终端，子进程才能通过 isatty 检查
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}

	if err := cmd.Start(); err != nil {
		slave.Close()
		master.Close()
		return nil, err
	}
	slave.Close() // 父进程只保留 master
	return master, nil
}

// canOpenPTY 用于诊断：只验证能否分配伪终端，不启动任何进程
func canOpenPTY() error {
	master, slave, err := openPTYPair()
	if err != nil {
		return err
	}
	slave.Close()
	master.Close()
	return nil
}
