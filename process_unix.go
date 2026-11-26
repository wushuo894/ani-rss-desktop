//go:build !windows

package main

import (
	"context"
	"os/exec"
	"syscall"
)

// configureJava 在 Unix 上沿用系统默认的子进程配置。
func configureJava(_ *exec.Cmd) {}

// requestStop 发送 SIGTERM，让 JVM 执行关闭钩子。
func requestStop(_ context.Context, cmd *exec.Cmd, _ Settings) error {
	return cmd.Process.Signal(syscall.SIGTERM)
}
