package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var downloadClient = &http.Client{Timeout: 20 * time.Minute}

// remoteJSON 读取发行源的元数据，对响应体设置大小限制。
func remoteJSON(ctx context.Context, url string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res, err := remoteGet(ctx, url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(out)
}

// remoteGet 只接受成功的响应，避免将错误网页保存为安装文件。
func remoteGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ani-rss-desktop")
	res, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("下载源返回 HTTP %d", res.StatusCode)
	}
	return res, nil
}

type downloadProgress struct {
	desktop *Desktop
	total   int64
	current int64
	updated time.Time
}

// Write 累计下载字节数，限制界面进度更新频率。
func (p *downloadProgress) Write(data []byte) (int, error) {
	p.current += int64(len(data))
	if time.Since(p.updated) >= 150*time.Millisecond || p.current == p.total {
		p.desktop.mu.Lock()
		p.desktop.status.Received, p.desktop.status.Total = p.current, p.total
		p.desktop.mu.Unlock()
		p.updated = time.Now()
	}
	return len(data), nil
}

// download 下载到临时位置，并核对发行源提供的大小和 SHA-256。
func (d *Desktop) download(url, path, digest string, size int64) error {
	const limit = 2 << 30
	digest = strings.TrimPrefix(strings.ToLower(digest), "sha256:")
	if _, err := hex.DecodeString(digest); len(digest) != 64 || err != nil || size <= 0 || size > limit {
		return errors.New("发行文件缺少有效的大小或 SHA-256")
	}
	res, err := remoteGet(d.ctx, url)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	checksum := sha256.New()
	progress := &downloadProgress{desktop: d, total: size}
	written, err := io.Copy(io.MultiWriter(file, checksum, progress), io.LimitReader(res.Body, size+1))
	if err := errors.Join(err, file.Close()); err != nil {
		return err
	}
	if written != size || hex.EncodeToString(checksum.Sum(nil)) != digest {
		return errors.New("文件大小或 SHA-256 校验失败")
	}
	return nil
}
