package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// JDK 描述可选的 Java 安装，以及安装文件的归属。
type JDK struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	Major      int    `json:"major"`
	Arch       string `json:"arch"`
	Java       string `json:"java"`
	Discovered bool   `json:"discovered"`
	Managed    bool   `json:"managed"`
	Usable     bool   `json:"usable"`
}

type temurinPackage struct {
	Link     string `json:"link"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

// GetJDKs 列出已登记的 JDK，并标记已被外部移除的条目。
func (d *Desktop) GetJDKs() []JDK {
	jdks := d.snapshot().JDKs
	for i := range jdks {
		_, err := os.Stat(jdks[i].Java)
		jdks[i].Usable = err == nil && jdks[i].Major >= minJava
	}
	return jdks
}

// AddJDK 将现有安装登记到列表。路径可指向 JDK 根目录、macOS 包或 java。
func (d *Desktop) AddJDK(path string) error {
	return d.action("添加 JDK", func() error {
		jdk, err := inspectJDK(d.ctx, path)
		if err != nil {
			return err
		}
		return d.addJDK(jdk)
	})
}

// SelectJDK 设置下一次启动所用的 Java。
func (d *Desktop) SelectJDK(id string) error {
	return d.action("选择 JDK", func() error {
		if _, err := d.selectedJDK(id); err != nil {
			return err
		}
		data := d.snapshot()
		data.Settings.JDKID = id
		return d.save(data)
	})
}

// DeleteJDK 移除记录，只清理由桌面端下载的对应安装目录。
func (d *Desktop) DeleteJDK(id string) error {
	return d.action("删除 JDK", func() error {
		data := d.snapshot()
		i := slices.IndexFunc(data.JDKs, func(jdk JDK) bool { return jdk.ID == id })
		if i < 0 {
			return errors.New("JDK 条目不存在")
		}
		if d.GetStatus().RunningJDK == id {
			return errors.New("请先停止服务，或切换 JDK 后重启")
		}
		jdk := data.JDKs[i]
		var original, pending string
		if jdk.Managed {
			if !filepath.IsLocal(id) || filepath.Base(id) != id || strings.HasPrefix(id, ".") {
				return errors.New("JDK 安装目录无效")
			}
			original = filepath.Join(d.root, "jdks", id)
			pending = filepath.Join(d.root, "jdks", ".delete-"+rand.Text())
			if err := os.Rename(original, pending); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return err
				}
				pending = ""
			}
		}
		if jdk.Discovered {
			data.IgnoredJDKs = append(data.IgnoredJDKs, jdk.Java)
		}
		data.JDKs = slices.Delete(data.JDKs, i, i+1)
		if data.Settings.JDKID == id {
			data.Settings.JDKID = ""
		}
		if err := d.save(data); err != nil {
			if pending != "" {
				return errors.Join(err, os.Rename(pending, original))
			}
			return err
		}
		if pending != "" {
			return os.RemoveAll(pending)
		}
		return nil
	})
}

// DownloadJDK 获取所选正式发行版并在验证完成后登记。
func (d *Desktop) DownloadJDK(releaseName string) error {
	return d.action("下载 JDK", func() (installErr error) {
		releases, err := temurinReleases(d.ctx)
		if err != nil {
			return err
		}
		index := slices.IndexFunc(releases, func(release JDKRelease) bool { return release.ID == releaseName })
		if index < 0 {
			return errors.New("该 Temurin 版本已不在下载列表中，请刷新")
		}
		pkg := releases[index].archive
		base := filepath.Join(d.root, "jdks")
		if err := os.MkdirAll(base, 0700); err != nil {
			return err
		}
		temp, err := os.MkdirTemp(base, ".install-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		archive := filepath.Join(temp, "download")
		if err := d.download(pkg.Link, archive, pkg.Checksum, pkg.Size); err != nil {
			return err
		}

		// 直接解压到最终目录，避免 Windows 在探测 java.exe 后拒绝移动整个目录。
		id := rand.Text()
		target := filepath.Join(base, id)
		if err := os.Mkdir(target, 0700); err != nil {
			return err
		}
		defer func() {
			if installErr != nil {
				installErr = errors.Join(installErr, os.RemoveAll(target))
			}
		}()
		if err := extractJDK(archive, pkg.Name, target); err != nil {
			return fmt.Errorf("解压 JDK 失败：%w", err)
		}
		// 安装目录与 Java 路径使用相同的符号链接解析规则。
		contents, err := filepath.EvalSymlinks(target)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(contents)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			jdk, err := inspectJDK(d.ctx, filepath.Join(contents, entry.Name()))
			if err != nil {
				continue
			}
			data := d.snapshot()
			for _, installed := range data.JDKs {
				if _, err := os.Stat(installed.Java); installed.Managed && installed.Version == jdk.Version && installed.Arch == jdk.Arch && err == nil {
					return fmt.Errorf("Temurin %s 已安装", jdk.Version)
				}
			}
			relative, err := filepath.Rel(contents, jdk.Java)
			if err != nil || !filepath.IsLocal(relative) {
				return errors.New("Java 可执行文件不在安装目录内")
			}
			jdk.ID, jdk.Java, jdk.Managed = id, filepath.Join(target, relative), true
			return d.addJDK(jdk)
		}
		return errors.New("安装包中没有找到可运行的 JDK")
	})
}

// addJDK 保存新条目，并自动选中第一个兼容的 JDK。
func (d *Desktop) addJDK(jdk JDK) error {
	data := d.snapshot()
	for _, existing := range data.JDKs {
		if sameJavaPath(existing.Java, jdk.Java) {
			return errors.New("该 JDK 路径已添加")
		}
	}
	data.JDKs = append(data.JDKs, jdk)
	data.IgnoredJDKs = slices.DeleteFunc(data.IgnoredJDKs, func(path string) bool { return sameJavaPath(path, jdk.Java) })
	if data.Settings.JDKID == "" && jdk.Usable {
		data.Settings.JDKID = jdk.ID
	}
	return d.save(data)
}

// selectedJDK 返回可运行的配置项，并报告版本和路径失效。
func (d *Desktop) selectedJDK(id string) (JDK, error) {
	for _, jdk := range d.GetJDKs() {
		if jdk.ID == id {
			if !jdk.Usable {
				return JDK{}, fmt.Errorf("选中的 JDK 不可用，需要 Java %d 或更新版本", minJava)
			}
			return jdk, nil
		}
	}
	return JDK{}, fmt.Errorf("请先在 JDK 管理中下载或添加 Java %d，再选择用于启动的 JDK", minJava)
}

// resolveJava 将 JDK、.jdk 包或 java 路径解析为实际的可执行文件。
func resolveJava(path string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		return "", errors.New("请选择 JDK 或 Java 可执行文件的绝对路径")
	}
	java := "java"
	if runtime.GOOS == "windows" {
		java += ".exe"
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		found := false
		for _, candidate := range []string{filepath.Join(path, "bin", java), filepath.Join(path, "Contents", "Home", "bin", java)} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				path, found = candidate, true
				break
			}
		}
		if !found {
			return "", errors.New("该目录中没有找到 bin/java")
		}
	}
	return filepath.EvalSymlinks(path)
}

// inspectJDK 执行 Java 读取其实际版本和 CPU 架构。
func inspectJDK(ctx context.Context, path string) (JDK, error) {
	path, err := resolveJava(path)
	if err != nil {
		return JDK{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-XshowSettings:properties", "-version")
	configureJava(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return JDK{}, fmt.Errorf("无法运行 Java：%w", err)
	}
	jdk := JDK{ID: rand.Text(), Java: path}
	for line := range strings.SplitSeq(string(output), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok {
			continue
		}
		switch key {
		case "java.version":
			jdk.Version = strings.TrimSpace(value)
		case "os.arch":
			jdk.Arch = strings.TrimSpace(value)
		}
	}
	version := strings.TrimPrefix(jdk.Version, "1.")
	major := strings.SplitN(version, ".", 2)[0]
	major = strings.SplitN(major, "-", 2)[0]
	jdk.Major, _ = strconv.Atoi(major)
	if jdk.Major < 1 || jdk.Arch == "" {
		return JDK{}, errors.New("无法识别 Java 版本或架构")
	}
	jdk.Usable = jdk.Major >= minJava
	return jdk, nil
}
