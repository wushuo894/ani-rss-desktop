package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractJDK 将 Temurin 压缩包解压到已创建的空安装目录。
func extractJDK(path, name, dest string) error {
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	if strings.HasSuffix(name, ".zip") {
		return extractZip(path, root)
	}
	if strings.HasSuffix(name, ".tar.gz") {
		return extractTar(path, root)
	}
	return errors.New("未知的安装包格式")
}

// extractZip 写入 Windows JDK 文件和目录。
func extractZip(path string, root *os.Root) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		name := filepath.FromSlash(entry.Name)
		if entry.FileInfo().IsDir() {
			if err := root.MkdirAll(name, 0755); err != nil {
				return err
			}
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("不支持的 ZIP 条目：%s", entry.Name)
		}
		body, err := entry.Open()
		if err != nil {
			return err
		}
		if err := errors.Join(writeArchiveFile(root, name, entry.Mode(), body), body.Close()); err != nil {
			return err
		}
	}
	return nil
}

// extractTar 最后添加符号链接，避免解压后续条目时跟随归档中的链接。
func extractTar(path string, root *os.Root) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	var links [][2]string
	for {
		entry, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.FromSlash(entry.Name)
		switch entry.Typeflag {
		case tar.TypeDir:
			err = root.MkdirAll(name, 0755)
		case tar.TypeReg, tar.TypeRegA:
			err = writeArchiveFile(root, name, entry.FileInfo().Mode(), reader)
		case tar.TypeSymlink:
			target := filepath.FromSlash(entry.Linkname)
			if filepath.IsAbs(target) || !filepath.IsLocal(filepath.Join(filepath.Dir(name), target)) {
				return fmt.Errorf("链接目标超出安装目录：%s", name)
			}
			links = append(links, [2]string{name, target})
		default:
			return fmt.Errorf("不支持的 TAR 条目：%s", name)
		}
		if err != nil {
			return err
		}
	}
	for _, link := range links {
		if err := root.MkdirAll(filepath.Dir(link[0]), 0755); err != nil {
			return err
		}
		if err := root.Symlink(link[1], link[0]); err != nil {
			return err
		}
	}
	return nil
}

// writeArchiveFile 保留可执行权限，避免覆盖重复条目。
func writeArchiveFile(root *os.Root, name string, mode os.FileMode, reader io.Reader) error {
	if err := root.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600|mode.Perm()&0111)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, reader)
	return errors.Join(err, file.Close())
}
