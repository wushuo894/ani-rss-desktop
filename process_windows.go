package main

import (
	"context"
	"os/exec"
	"syscall"
)

// configureJava 避免 Windows 桌面应用拉起额外的控制台。
func configureJava(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

// requestStop 使用当前服务的 API Key 调用停止接口触发 Java 关闭钩子。
func requestStop(ctx context.Context, _ *exec.Cmd, settings Settings) error {
	apiKey, err := readAPIKey(settings.DataDir)
	if err != nil {
		return err
	}
	return apiRequest(ctx, settings.serviceURL(), "stop?status=1", apiKey, nil)
}
