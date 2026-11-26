package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// JDKRelease 是 Adoptium 为本机系统和架构提供的正式 JDK 版本。
type JDKRelease struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Major   int    `json:"major"`
	archive temurinPackage
}

// GetJDKVersions 从 Adoptium 获取可下载的版本，界面直接使用这些选项。
func (d *Desktop) GetJDKVersions() ([]JDKRelease, error) { return temurinReleases(d.ctx) }

// temurinReleases 只列出符合最低 Java 版本要求且包含对应压缩包的正式发行版。
func temurinReleases(ctx context.Context) ([]JDKRelease, error) {
	system := runtime.GOOS
	if system == "darwin" {
		system = "mac"
	}
	arch := map[string]string{"amd64": "x64", "arm64": "aarch64"}[runtime.GOARCH]
	if arch == "" {
		return nil, errors.New("此 CPU 架构请使用手动添加")
	}
	url := fmt.Sprintf("https://api.adoptium.net/v3/assets/version/%%5B%d%%2C%%29?architecture=%s&image_type=jdk&jvm_impl=hotspot&os=%s&vendor=eclipse&release_type=ga&page_size=50", minJava, arch, system)
	var entries []struct {
		Name    string `json:"release_name"`
		Version struct {
			OpenJDK string `json:"openjdk_version"`
			Major   int    `json:"major"`
		} `json:"version_data"`
		Binaries []struct {
			Package temurinPackage `json:"package"`
		} `json:"binaries"`
	}
	if err := remoteJSON(ctx, url, &entries); err != nil {
		return nil, fmt.Errorf("读取 Temurin 版本失败：%w", err)
	}
	releases := make([]JDKRelease, 0, len(entries))
	for _, entry := range entries {
		for _, binary := range entry.Binaries {
			pkg := binary.Package
			if pkg.Link != "" && (strings.HasSuffix(pkg.Name, ".zip") || strings.HasSuffix(pkg.Name, ".tar.gz")) {
				releases = append(releases, JDKRelease{ID: entry.Name, Version: entry.Version.OpenJDK, Major: entry.Version.Major, archive: pkg})
				break
			}
		}
	}
	return releases, nil
}

// DiscoverJDKs 扫描新增的系统安装并保留用户已有的选用设置。
func (d *Desktop) DiscoverJDKs() error { return d.action("扫描本机 JDK", d.discoverJDKs) }

// discoverJDKs 检查 JAVA_HOME、PATH，以及 macOS 系统注册的默认 JDK。
func (d *Desktop) discoverJDKs() error {
	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Second)
	defer cancel()
	candidates := []string{os.Getenv("JAVA_HOME")}
	if path, err := exec.LookPath("java"); err == nil {
		candidates = append(candidates, path)
	}
	if runtime.GOOS == "darwin" {
		if out, err := exec.CommandContext(ctx, "/usr/libexec/java_home", "-F").Output(); err == nil {
			candidates = append(candidates, strings.TrimSpace(string(out)))
		}
	}
	data := d.snapshot()
	count := len(data.JDKs)
	for _, candidate := range candidates {
		path, err := resolveJava(candidate)
		if err != nil || strings.HasPrefix(path, filepath.Join(d.root, "jdks")+string(filepath.Separator)) {
			continue
		}
		known := slices.ContainsFunc(data.JDKs, func(jdk JDK) bool { return sameJavaPath(jdk.Java, path) })
		ignored := slices.ContainsFunc(data.IgnoredJDKs, func(entry string) bool { return sameJavaPath(entry, path) })
		if known || ignored {
			continue
		}
		jdk, err := inspectJDK(ctx, path)
		if err != nil {
			continue
		}
		jdk.Discovered = true
		data.JDKs = append(data.JDKs, jdk)
		if data.Settings.JDKID == "" && jdk.Usable {
			data.Settings.JDKID = jdk.ID
		}
	}
	if count == len(data.JDKs) {
		return nil
	}
	return d.save(data)
}

// sameJavaPath 在 Windows 上按不区分大小写的方式比较可执行路径。
func sameJavaPath(a, b string) bool {
	return a == b || (runtime.GOOS == "windows" && strings.EqualFold(a, b))
}
