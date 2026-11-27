# ani-rss Desktop

使用 MyGo 和 Vuetify 管理本机的 ani-rss 服务。桌面端配置保存在当前用户主目录下的 `ani-rss-desktop/settings.json`
。启动设置中的「配置与数据目录」对应服务的 `--config`，可以选用已有的 ani-rss 数据目录。

首次使用时，可选用自动发现的 Java 25，也可以在「JDK 管理」里手动添加或下载 Eclipse Temurin 25，然后在「概览」启动服务。桌面端会下载项目的正式发行
JAR，并校验文件大小和 SHA-256。

## 使用 Homebrew 安装

适用于搭载 Apple Silicon 的 Mac，要求 macOS 12 或更高版本。安装命令如下。

```sh
brew install --cask wushuo894/tap/ani-rss-desktop
```

通过 Homebrew 升级时运行以下命令。

```sh
brew upgrade --cask wushuo894/tap/ani-rss-desktop
```

## 开发与打包

需要 Go 1.27.3、Node.js 22.13 或更新版本和 pnpm。在 `ani-rss-desktop/` 中运行以下命令。

```sh
pnpm install --frozen-lockfile
go tool mygo dev
```

MyGo 会重新生成 `src/mygo.ts`。生成当前平台的桌面包可运行 `go tool mygo build`，产物位于 `build/`。在 macOS 交叉编译 Linux
软件包和 Windows 可执行文件时，可运行以下命令。

```sh
go tool mygo build -platform linux/amd64,windows/amd64 -skip-installer
```

- macOS 使用系统 WKWebView，用户登录自启需要从 `.app` 打开应用。
- Windows 需要 WebView2。Linux 需要 GTK 3 和 WebKitGTK 4.1，托盘还需要 AppIndicator
  支持。 [MyGo 系统依赖](https://mygo.run/docs/getting-started) · [托盘依赖](https://mygo.run/docs/menus)
- 带有 DesktopUpdate 的 ani-rss 发行版支持从 Web UI 发起更新，也支持服务定时自更新。Java 退出后，桌面端替换 JAR 并拉起服务。
