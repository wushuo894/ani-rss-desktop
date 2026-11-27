# ani-rss-desktop

使用 MyGo 和 Vuetify 管理本机的 ani-rss 服务。桌面端配置保存在当前用户主目录下的 `ani-rss-desktop/settings.json`
。启动设置中的「配置与数据目录」对应服务的 `--config`，可以选用已有的 ani-rss 数据目录。

Linux 和 Windows 主窗口使用系统原生标题栏、按钮、边框、圆角和阴影。只有 macOS 使用沉浸式标题栏，
保留原生窗口按钮，侧栏和正文延伸到窗口顶部。关闭主窗口仍收起到托盘。

首次使用时，可选用自动发现的 Java 25，也可以在「JDK 管理」里手动添加或下载 Eclipse Temurin 25，然后在「概览」启动服务。桌面端会下载项目的正式发行
JAR，并校验文件大小和 SHA-256。

## 托盘快捷面板

托盘所有服务操作统一放在 MyGo `NewFlyout` 面板中，显示服务状态、版本和操作进度，并提供打开 Web UI、
启动、重启、停止、显示主窗口、桌面端更新及退出。按钮随服务状态禁用；点击外部或按 Esc 收起面板。
macOS 和 Windows 左右键点击托盘均打开同一面板。Linux 受 AppIndicator 限制，托盘菜单仅保留「打开快捷面板」入口，
面板显示在主屏幕中央。面板沿用系统深浅色，单独加载 `tray.html`，关闭后释放页面。

## 开发与打包

需要 Go 1.27.3、Node.js 22.13 或更新版本和 pnpm。在 `ani-rss-desktop/` 中运行以下命令。

```sh
pnpm install --frozen-lockfile
go tool mygo dev
```

MyGo 会重新生成 `src/mygo.ts`。生成当前平台的桌面包可运行 `go tool mygo build`，产物位于 `build/`。在 macOS 交叉编译 Linux
软件包和 Windows 可执行文件时，可运行以下命令。

```sh
go tool mygo build -platform linux/amd64,windows/amd64,darwin/arm64 -skip-installer
```

- macOS 使用系统 WKWebView，用户登录自启需要从 `.app` 打开应用。
- Windows 需要 WebView2。Linux 需要 GTK 3 和 WebKitGTK 4.1，托盘还需要 AppIndicator
  支持。 [MyGo 系统依赖](https://mygo.run/docs/getting-started) · [托盘依赖](https://mygo.run/docs/menus)
- 带有 DesktopUpdate 的 ani-rss 发行版支持从 Web UI 发起更新，也支持服务定时自更新。Java 退出后，桌面端替换 JAR 并拉起服务。

## 桌面端自更新

在「启动设置 → 桌面与账户 → 桌面端更新」或托盘菜单中点击「检查桌面端更新」。桌面端使用 MyGo 的中文更新窗口，
启动 10 秒后自动检查，此后每天检查一次；窗口支持查看发行说明、跳过版本和自动下载安装。
安装后可以立即重启，也可以下次启动时使用新版。更新退出前正在运行的 ANI-RSS 服务会在下次启动桌面端时恢复，
并沿用退出前实际生效的 JDK 和启动参数。概览中的「检查更新」仍用于更新 ANI-RSS JAR。

发布前在桌面端仓库 `wushuo894/ani-rss-desktop` 的 Actions Secrets 中设置 `MYGO_UPDATER_PRIVATE_KEY`，
值为与 `mygo.json` 中公钥对应的私钥文件完整内容。私钥只需配置一次，不要提交到 Git。
修改 `mygo.json` 中的版本与 `UPDATE.md` 发行说明后，运行 **Build desktop** 工作流，
工作流会生成签名更新包、增量包及各平台清单，上传完成后发布 `v版本号`。
本地构建也需要通过同名环境变量提供私钥才能生成签名更新文件。

开发版不支持自更新。Linux 用户需要使用 `install.sh` 或解压后的可写目录版本；deb/rpm 由包管理器更新，AppImage 不支持此更新方式。
