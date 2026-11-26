package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChangePassword 直接更新 config.v2.json。运行中的实例先停止，避免覆盖文件。
func (d *Desktop) ChangePassword(next string) error {
	if strings.TrimSpace(next) == "" {
		return errors.New("请输入新密码")
	}
	return d.action("修改密码", func() error {
		d.mu.Lock()
		run, dir := d.proc, d.data.Settings.DataDir
		d.mu.Unlock()
		if run != nil {
			dir = run.settings.DataDir
		}

		path, err := filepath.EvalSymlinks(filepath.Join(dir, "config.v2.json"))
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("尚未生成 config.v2.json，请先启动 ani-rss 完成初始化")
		}
		if err != nil {
			return err
		}
		if run != nil {
			if err := d.stop(); err != nil {
				return err
			}
		}
		err = replacePassword(path, next)
		if run != nil {
			if restartErr := d.startWith(run.settings, run.jdk); restartErr != nil {
				if err == nil {
					return fmt.Errorf("密码已写入，服务重新启动失败：%w", restartErr)
				}
				return errors.Join(err, restartErr)
			}
		}
		return err
	})
}

// replacePassword 保留其他 JSON 字段，并刷新 tokenId 以使旧会话失效。
func replacePassword(path, password string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("读取 config.v2.json 失败：%w", err)
	}
	var login map[string]json.RawMessage
	if err := json.Unmarshal(config["login"], &login); err != nil || login == nil {
		return errors.New("config.v2.json 缺少有效的 login 配置")
	}
	login["password"], _ = json.Marshal(passwordMD5(password))
	login["isDefault"] = json.RawMessage("false")
	config["login"], err = json.Marshal(login)
	if err != nil {
		return err
	}
	var token [16]byte
	rand.Read(token[:])
	token[6] = token[6]&0x0f | 0x40
	token[8] = token[8]&0x3f | 0x80
	config["tokenId"], _ = json.Marshal(fmt.Sprintf("%x-%x-%x-%x-%x", token[:4], token[4:6], token[6:8], token[8:10], token[10:]))
	return writeJSON(path, config)
}

// passwordMD5 生成 ani-rss 配置文件使用的密码摘要。
func passwordMD5(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}
