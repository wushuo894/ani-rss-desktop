package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

type javaRun struct {
	cancel   context.CancelFunc
	done     chan struct{}
	ready    chan error
	settings Settings
	jdk      JDK
}

// Start 使用保存的配置拉起本机服务。
func (d *Desktop) Start() error { return d.action("启动服务", d.start) }

// Stop 停止当前托管的进程。
func (d *Desktop) Stop() error { return d.action("停止服务", d.stop) }

// Restart 先等待旧 Java 退出，再使用已保存设置启动。
func (d *Desktop) Restart() error { return d.action("重启服务", d.restart) }

// active 表示桌面端已持有一轮启动，包括等待就绪的阶段。
func (d *Desktop) active() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.proc != nil
}

// start 在必要时下载 JAR，并启用与 run.sh 同语义的监管循环。
func (d *Desktop) start() error {
	if d.active() {
		return nil
	}
	settings := d.GetSettings()
	jdk, err := d.selectedJDK(settings.JDKID)
	if err != nil {
		return err
	}
	return d.startWith(settings, jdk)
}

// startWith 启动指定的配置快照，改密重启时使用正在运行的设置。
func (d *Desktop) startWith(settings Settings, jdk JDK) error {
	if err := settings.validate(); err != nil {
		return err
	}
	settings.DataDir = filepath.Clean(settings.DataDir)
	if err := os.MkdirAll(settings.DataDir, 0700); err != nil {
		return fmt.Errorf("无法创建配置目录：%w", err)
	}
	probe, err := os.CreateTemp(settings.DataDir, ".desktop-write-*")
	if err != nil {
		return fmt.Errorf("配置目录不可写：%w", err)
	}
	probe.Close()
	os.Remove(probe.Name())
	if _, err := os.Stat(d.jar()); errors.Is(err, os.ErrNotExist) {
		release, err := latestRelease(d.ctx)
		if err != nil {
			return err
		}
		replacement, version, err := d.stageJar(release)
		if err != nil {
			return err
		}
		defer os.Remove(replacement)
		if err := os.Rename(replacement, d.jar()); err != nil {
			return err
		}
		d.setVersion(version)
	} else if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	run := &javaRun{cancel: cancel, done: make(chan struct{}), ready: make(chan error, 1), settings: settings, jdk: jdk}
	d.mu.Lock()
	d.proc = run
	d.status.State, d.status.Message = "starting", ""
	d.status.URL, d.status.RunningJDK, d.status.RunningDir = settings.serviceURL(), jdk.ID, settings.DataDir
	d.status.Swagger, d.status.MCP = settings.Swagger, settings.MCP
	d.mu.Unlock()
	go d.supervise(ctx, run, settings, jdk)
	return nil
}

// restart 保证新旧进程不会同时使用同一个数据目录。
func (d *Desktop) restart() error {
	if err := d.stop(); err != nil {
		return err
	}
	return d.start()
}

// stop 取消拉起意图，等待监管循环收尾。
func (d *Desktop) stop() error {
	d.mu.Lock()
	run := d.proc
	if run != nil {
		d.status.State = "stopping"
		run.cancel()
	}
	d.mu.Unlock()
	if run != nil {
		<-run.done
	}
	return nil
}

// shutdown 先取消网络下载，再等待管理操作释放进程。
func (d *Desktop) shutdown() {
	d.shutdownForUpdate(false)
}

// supervise 按退出码决定重启，并避免启动阶段异常形成无限重启。
func (d *Desktop) supervise(ctx context.Context, run *javaRun, settings Settings, jdk JDK) {
	var lastErr error
	defer func() {
		d.mu.Lock()
		if lastErr != nil {
			d.status.State, d.status.Message = "failed", lastErr.Error()
		} else {
			d.status.State, d.status.Message = "stopped", ""
		}
		d.status.URL, d.status.RunningJDK, d.status.RunningDir = "", "", ""
		d.status.Swagger, d.status.MCP = false, false
		d.proc = nil
		d.mu.Unlock()
		if lastErr == nil {
			lastErr = errors.New("服务已停止")
		}
		select {
		case run.ready <- lastErr:
		default:
		}
		close(run.done)
	}()

	for ctx.Err() == nil {
		backup, err := d.applyPendingJar()
		if err != nil {
			// 移交失败时仍尝试启动本地可用的版本，具体原因写到日志。
			d.appendLog([]byte(fmt.Sprintf("\n应用服务更新失败：%v\n", err)))
		}
		ready, err := d.runJava(ctx, run, settings, jdk)
		if !ready && backup != "" {
			if restoreErr := d.restoreJar(backup); restoreErr != nil {
				lastErr = errors.Join(err, restoreErr)
				return
			}
			d.appendLog([]byte(fmt.Sprintf("\n新版未就绪，已恢复旧 JAR：%v\n", err)))
			continue
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			lastErr = err
			return
		}
		d.appendLog([]byte("\n服务请求重启，正在重新启动。\n"))
		d.setVersionFromJar()
	}
}

// runJava 执行一次 JVM，返回它是否曾就绪及其退出原因。
func (d *Desktop) runJava(ctx context.Context, run *javaRun, settings Settings, jdk JDK) (bool, error) {
	d.mu.Lock()
	d.status.State = "starting"
	d.mu.Unlock()

	bind := net.JoinHostPort(settings.Address, strconv.Itoa(settings.Port))
	listener, err := net.Listen("tcp", bind)
	if err != nil {
		return false, fmt.Errorf("无法绑定 %s，检查地址和端口占用：%w", bind, err)
	}
	listener.Close()

	cmd := exec.Command(jdk.Java, d.javaArgs(settings)...)
	cmd.Dir = d.root
	cmd.Stdout, cmd.Stderr = serviceLog{d}, serviceLog{d}
	configureJava(cmd)
	d.appendLog([]byte(fmt.Sprintf("\n%s 启动 ani-rss，Java %s，地址 %s\n", time.Now().Format(time.DateTime), jdk.Version, bind)))
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("无法启动 Java：%w", err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	pingCtx, cancelPing := context.WithTimeout(ctx, 75*time.Second)
	defer cancelPing()
	ping := make(chan error, 1)
	go func() { ping <- waitHTTP(pingCtx, settings.serviceURL()) }()
	select {
	case <-ctx.Done():
		d.terminate(cmd, waited, settings)
		return false, ctx.Err()
	case err := <-waited:
		return false, fmt.Errorf("Java 在服务就绪前退出：%v", err)
	case err := <-ping:
		if err != nil {
			d.terminate(cmd, waited, settings)
			return false, fmt.Errorf("等待服务就绪失败：%w", err)
		}
	}
	d.mu.Lock()
	d.status.State = "running"
	d.mu.Unlock()
	select {
	case run.ready <- nil:
	default:
	}
	select {
	case <-ctx.Done():
		d.terminate(cmd, waited, settings)
		return true, ctx.Err()
	case err := <-waited:
		if err != nil {
			return true, fmt.Errorf("Java 进程退出：%w", err)
		}
		return true, nil
	}
}

// terminate 优先让 Java 正常退出，超时后结束本次启动的进程。
func (d *Desktop) terminate(cmd *exec.Cmd, waited <-chan error, settings Settings) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	err := requestStop(ctx, cmd, settings)
	cancel()
	if err == nil {
		select {
		case <-waited:
			return
		case <-time.After(8 * time.Second):
		}
	}
	_ = cmd.Process.Kill()
	<-waited
}

// GetLogs 返回有界的最近输出。重新打开应用后可以查看 ani-rss 持久化日志。
func (d *Desktop) GetLogs() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return string(bytes.ToValidUTF8(d.logText, []byte("�")))
}

type serviceLog struct{ desktop *Desktop }

// Write 接收 Java 的标准输出或错误输出。
func (w serviceLog) Write(data []byte) (int, error) {
	w.desktop.appendLog(data)
	return len(data), nil
}

// appendLog 保留最近 256 KiB，避免长时间运行累积内存。
func (d *Desktop) appendLog(data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logText = append(d.logText, data...)
	const capacity = 256 * 1024
	if len(d.logText) > capacity {
		d.logText = bytes.Clone(d.logText[len(d.logText)-capacity:])
	}
}
