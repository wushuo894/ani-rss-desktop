package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type jarRelease struct {
	Version string
	URL     string
	Digest  string
	Size    int64
}

type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
}

// latestRelease 读取项目的正式发行版及其 JAR 校验信息。
func latestRelease(ctx context.Context) (jarRelease, error) {
	const url = "https://api.github.com/repos/wushuo894/ani-rss/releases/latest"
	var release struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"assets"`
	}
	if err := remoteJSON(ctx, url, &release); err != nil {
		return jarRelease{}, fmt.Errorf("读取 ani-rss 发行版本失败：%w", err)
	}
	for _, asset := range release.Assets {
		if asset.Name == "ani-rss.jar" {
			return jarRelease{Version: strings.TrimPrefix(release.Tag, "v"), URL: asset.URL, Digest: asset.Digest, Size: asset.Size}, nil
		}
	}
	return jarRelease{}, errors.New("发行版中缺少 ani-rss.jar")
}

// CheckUpdate 返回当前版本与 GitHub 最新正式版本。
func (d *Desktop) CheckUpdate() (UpdateInfo, error) {
	release, err := latestRelease(d.ctx)
	if err != nil {
		return UpdateInfo{}, err
	}
	current := d.GetStatus().Version
	return UpdateInfo{Current: current, Latest: release.Version, Available: newerVersion(release.Version, current)}, nil
}

// Update 校验下载文件，再串行执行停机、替换和必要的回滚。
func (d *Desktop) Update() error {
	return d.action("更新 ani-rss", func() error {
		release, err := latestRelease(d.ctx)
		if err != nil {
			return err
		}
		if !newerVersion(release.Version, d.GetStatus().Version) {
			return nil
		}
		staged, version, err := d.stageJar(release)
		if err != nil {
			return err
		}
		defer os.Remove(staged)

		wasActive := d.active()
		if err := d.stop(); err != nil {
			return err
		}
		previous, err := d.replaceJar(staged)
		if err != nil {
			if wasActive {
				return errors.Join(err, d.start())
			}
			return err
		}
		d.setVersion(version)
		if !wasActive {
			return nil
		}
		if err := d.startReady(); err != nil {
			_ = d.stop()
			if previous == "" {
				return err
			}
			if restoreErr := d.restoreJar(previous); restoreErr != nil {
				return errors.Join(err, restoreErr)
			}
			return errors.Join(fmt.Errorf("新版启动失败，已恢复旧 JAR：%w", err), d.start())
		}
		return nil
	})
}

// startReady 等候新 Java 就绪，使更新能感知启动失败。
func (d *Desktop) startReady() error {
	if err := d.start(); err != nil {
		return err
	}
	d.mu.Lock()
	run := d.proc
	message := d.status.Message
	d.mu.Unlock()
	if run == nil {
		return fmt.Errorf("服务启动失败：%s", message)
	}
	select {
	case err := <-run.ready:
		return err
	case <-d.ctx.Done():
		return d.ctx.Err()
	}
}

// replaceJar 留下一份旧版，替换失败时恢复原路径。
func (d *Desktop) replaceJar(staged string) (string, error) {
	backup := d.jar() + ".bak"
	if err := os.Rename(d.jar(), backup); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		backup = ""
	}
	if err := os.Rename(staged, d.jar()); err != nil {
		if backup != "" {
			return "", errors.Join(err, os.Rename(backup, d.jar()))
		}
		return "", err
	}
	return backup, nil
}

// stageJar 下载并确认 Spring Boot 构建元数据与发行标签一致。
func (d *Desktop) stageJar(release jarRelease) (string, string, error) {
	stage, err := os.MkdirTemp(d.root, ".jar-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(stage)
	path := filepath.Join(stage, "ani-rss.jar")
	if err := d.download(release.URL, path, release.Digest, release.Size); err != nil {
		return "", "", err
	}
	version, err := jarVersion(path)
	if err != nil {
		return "", "", err
	}
	if version != release.Version {
		return "", "", fmt.Errorf("JAR 版本 %s 与发行版本 %s 不符", version, release.Version)
	}
	temp, err := os.CreateTemp(d.root, ".update-*.jar")
	if err != nil {
		return "", "", err
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return "", "", err
	}
	if err := os.Rename(path, temp.Name()); err != nil {
		os.Remove(temp.Name())
		return "", "", err
	}
	return temp.Name(), version, nil
}

// jarVersion 读取 JAR 内 Spring Boot build-info，无需运行 Java。
func jarVersion(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, name := range []string{"BOOT-INF/classes/META-INF/build-info.properties", "META-INF/build-info.properties", "META-INF/MANIFEST.MF"} {
		entry, err := reader.Open(name)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(entry, 64<<10))
		entry.Close()
		if err != nil {
			return "", err
		}
		for line := range strings.SplitSeq(string(body), "\n") {
			for _, prefix := range []string{"build.version=", "Implementation-Version:"} {
				if value, ok := strings.CutPrefix(line, prefix); ok {
					return strings.TrimSpace(value), nil
				}
			}
		}
	}
	return "", errors.New("JAR 缺少版本信息")
}

// newerVersion 比较项目使用的数字版本号，避免覆盖更新的本地版本。
func newerVersion(latest, current string) bool {
	if current == "" {
		return latest != ""
	}
	left := strings.Split(strings.SplitN(latest, "-", 2)[0], ".")
	right := strings.Split(strings.SplitN(current, "-", 2)[0], ".")
	for i := 0; i < max(len(left), len(right)); i++ {
		part := func(parts []string) int {
			if i >= len(parts) {
				return 0
			}
			n, _ := strconv.Atoi(parts[i])
			return n
		}
		if a, b := part(left), part(right); a != b {
			return a > b
		}
	}
	return strings.Contains(current, "-") && !strings.Contains(latest, "-")
}

// jar 返回由桌面端管理的发行 JAR 路径。
func (d *Desktop) jar() string { return filepath.Join(d.root, "ani-rss.jar") }

// setVersionFromJar 读取本地版本，用于重启或还原备份后同步界面。
func (d *Desktop) setVersionFromJar() {
	version, _ := jarVersion(d.jar())
	d.setVersion(version)
}

// setVersion 更新界面中的已安装版本。
func (d *Desktop) setVersion(version string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.status.Version = version
}
