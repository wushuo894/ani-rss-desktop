package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// javaArgs 为启动和预览提供同一组参数，保持 JVM 选项位于 -jar 之前。
func (d *Desktop) javaArgs(settings Settings) []string {
	args := []string{
		fmt.Sprintf("-Xms%dm", settings.MinHeap), fmt.Sprintf("-Xmx%dm", settings.MaxHeap),
		"-Xss256k", "-XX:+UseG1GC", "-XX:+UseStringDeduplication", "-XX:+UseCompactObjectHeaders",
		"-XX:TieredStopAtLevel=1", "-XX:+IgnoreUnrecognizedVMOptions", "--enable-native-access=ALL-UNNAMED",
		"--add-opens=java.base/java.net=ALL-UNNAMED", "--add-opens=java.base/sun.net.www.protocol.https=ALL-UNNAMED",
		"-Dfile.encoding=UTF-8",
	}
	args = append(args, settings.JVMArgs...)
	if runtime.GOOS == "darwin" {
		args = append(args, "-Dapple.awt.enableTemplateImages=true")
	}
	return append(args, "-jar", d.jar(),
		"--config="+filepath.Clean(settings.DataDir), "--server.address="+settings.Address,
		fmt.Sprintf("--server.port=%d", settings.Port), "--ani-rss-desktop=true",
		fmt.Sprintf("--swagger-enabled=%t", settings.Swagger), fmt.Sprintf("--mcp-enabled=%t", settings.MCP))
}

// GetLaunchCommand 预览当前进程或下次启动会用到的命令。
func (d *Desktop) GetLaunchCommand(current bool) (string, error) {
	if current {
		d.mu.Lock()
		run := d.proc
		d.mu.Unlock()
		if run == nil {
			return "", errors.New("当前没有托管的 Java 进程")
		}
		return formatCommand(run.jdk.Java, d.javaArgs(run.settings)), nil
	}
	settings := d.GetSettings()
	jdk, err := d.selectedJDK(settings.JDKID)
	if err != nil {
		return "", err
	}
	return formatCommand(jdk.Java, d.javaArgs(settings)), nil
}

// formatCommand 生成可以复制到 POSIX shell 或 PowerShell 的命令。
func formatCommand(java string, args []string) string {
	parts := append([]string{java}, args...)
	for i, part := range parts {
		if runtime.GOOS == "windows" {
			parts[i] = "'" + strings.ReplaceAll(part, "'", "''") + "'"
		} else {
			parts[i] = "'" + strings.ReplaceAll(part, "'", "'\"'\"'") + "'"
		}
	}
	command := strings.Join(parts, " ")
	if runtime.GOOS == "windows" {
		return "& " + command
	}
	return command
}
