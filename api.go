package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var apiClient = &http.Client{
	Timeout: 12 * time.Second,
	// 服务接口只访问已启动的本机实例。
	Transport:     &http.Transport{},
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
}

var errAPIKeyRejected = errors.New("ani-rss 拒绝访问，请检查当前数据目录的 API Key")

type Overview struct {
	Subscriptions     int        `json:"subscriptions"`
	SubscriptionError string     `json:"subscriptionError"`
	Downloads         []Download `json:"downloads"`
	DownloadError     string     `json:"downloadError"`
}

type Download struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Progress float64 `json:"progress"`
	Size     string  `json:"formatSize"`
}

// apiRequest 使用 ani-rss 的 API Key 请求头和 JSON 响应格式。
func apiRequest(ctx context.Context, base, endpoint, apiKey string, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/"+endpoint, nil)
	if err != nil {
		return err
	}
	if apiKey != "" {
		req.Header.Set("api-key", apiKey)
	}
	res, err := apiClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return errAPIKeyRejected
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ani-rss 返回 HTTP %d", res.StatusCode)
	}
	var result struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<20)).Decode(&result); err != nil {
		return err
	}
	if result.Code == http.StatusForbidden || result.Code == http.StatusUnauthorized {
		return errAPIKeyRejected
	}
	if result.Code != http.StatusOK {
		return fmt.Errorf("ani-rss：%s", result.Message)
	}
	if output != nil {
		return json.Unmarshal(result.Data, output)
	}
	return nil
}

// waitHTTP 轮询官方健康接口，直到服务就绪或启动超时。
func waitHTTP(ctx context.Context, base string) error {
	for {
		probe, cancel := context.WithTimeout(ctx, 1200*time.Millisecond)
		err := apiRequest(probe, base, "ping", "", nil)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
}

// GetOverview 从当前实例的数据目录读取 API Key，分别获取订阅和下载状态。
func (d *Desktop) GetOverview() (Overview, error) {
	status := d.GetStatus()
	if status.State != "running" {
		return Overview{}, errors.New("请先启动 ani-rss")
	}

	// 每次刷新都读取最新配置，使 Web UI 修改 API Key 后能自动恢复。
	apiKey, err := readAPIKey(status.RunningDir)
	if err != nil {
		return Overview{}, err
	}
	result := Overview{Downloads: []Download{}}
	var list struct {
		Total int `json:"total"`
	}
	if err := apiRequest(d.ctx, status.URL, "listAni", apiKey, &list); err != nil {
		result.SubscriptionError = err.Error()
	}
	result.Subscriptions = list.Total
	if err := apiRequest(d.ctx, status.URL, "torrentsInfos", apiKey, &result.Downloads); err != nil {
		result.DownloadError = err.Error()
	}
	return result, nil
}

// readAPIKey 只解析服务配置中的 API Key，供 Go 后端请求使用。
func readAPIKey(dataDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dataDir, "config.v2.json"))
	if err != nil {
		return "", fmt.Errorf("读取 ani-rss 配置失败：%w", err)
	}
	var config struct {
		APIKey string `json:"apiKey"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return "", errors.New("无法读取 config.v2.json 中的 API Key")
	}
	if strings.TrimSpace(config.APIKey) == "" {
		return "", errors.New("当前数据目录的 config.v2.json 缺少 API Key")
	}
	return config.APIKey, nil
}

// runningURL 只允许管理桌面端当前已启动的服务。
func (d *Desktop) runningURL() (string, error) {
	status := d.GetStatus()
	if status.State != "running" {
		return "", errors.New("请先启动 ani-rss")
	}
	return status.URL, nil
}
