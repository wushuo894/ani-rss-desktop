# ani-rss 桌面端实施计划

在仓库根目录的 `ani-rss-desktop/` 下新增独立 Go 模块。桌面端面向 macOS、Windows 和 Linux，首版管理一个本机 `ani-rss.jar`
实例，支持使用已有的 ani-rss 数据目录。

## 技术方案

- MyGo 提供系统 WebView、托盘、窗口管理和原生文件选择。Vue 3、TypeScript、Vite 和 Vuetify 构建前端，通过 MyGo 生成的
  TypeScript 绑定调用
  Go。 [MyGo 文档](https://mygo.run/zh/docs/getting-started) · [Vuetify 文档](https://vuetifyjs.com/getting-started/installation/)
- Go 统一管理服务进程、JDK、启动配置、更新、日志和系统自启。桌面端的配置文件统一存放在当前用户主目录下的 `ani-rss-desktop/`，即
  `~/ani-rss-desktop/`。程序文件和下载的 JDK 存放在系统用户目录下。ani-rss 服务的订阅、配置和应用日志目录由 `--config` 指定。
- 主窗口提供「概览」「日志」「启动设置」「JDK 管理」。概览页展示运行状态、当前版本、订阅数量和下载摘要，并提供服务操作和打开 Web UI
  的入口。

## 启动设置

| 设置项         | 参数               | 行为                                                                                                        |
|----------------|--------------------|-------------------------------------------------------------------------------------------------------------|
| 配置与数据目录 | `--config`         | 支持浏览选择或手动输入。选择已有目录时使用其中的配置和订阅数据。启动前检查目录可写。                        |
| 服务端口       | `--server.port`    | 默认 `7789`。校验端口范围，启动失败时提示端口占用。                                                         |
| 绑定 IP 地址   | `--server.address` | 支持 `127.0.0.1`、`0.0.0.0` 或指定本机 IP。界面说明地址对应的访问范围。                                     |
| JVM 参数       | `java` 启动参数    | 沿用 `run.sh` 的默认值，支持修改初始堆内存、最大堆内存和额外参数。参数按独立项传给 Java，兼容带空格的路径。 |
| Swagger        | `SWAGGER_ENABLED`  | 默认关闭，开启后可从设置页打开接口文档。                                                                    |
| MCP            | `MCP_ENABLED`      | 默认关闭，开启后展示 `/api/mcp` 接入地址。                                                                  |

持续保存设置，服务运行时提供「保存并重启」。确认服务就绪后，桌面端按实际生效的地址和端口访问 API 及 Web UI。绑定 `0.0.0.0`
时，桌面端使用回环地址连接。 [启动参数与环境变量](../ani-rss-docs/docs/args-and-env.md)

## JDK 管理

列表展示 JDK 版本、CPU 架构、来源、安装路径和可用状态，分别标明当前服务使用的 JDK 与下次启动选用的 JDK。

- **下载**。首版支持 Eclipse Temurin，默认推荐 JDK 25。按操作系统和 CPU 架构选择安装包，显示进度，校验 SHA-256
  后解压到桌面端管理的目录。安装成功后登记到列表。 [Adoptium 下载与校验文档](https://adoptium.net/installation/ci-scripts/)
- **手动添加**。选择本机 `JAVA_HOME`、macOS 的 `.jdk` 包或 `java` 可执行文件，读取版本和架构，并识别重复路径。低于 Java 25
  的条目标记为不兼容。 [仓库 Java 版本配置](../pom.xml)
- **选择与切换**。保存选中的 JDK，启动、重启及开机自启均使用该 JDK。首次启动缺少可用 JDK 时，提供「下载 JDK 25」和「手动添加」入口。
- **删除**。删除桌面端下载的 JDK 时同步清理安装文件。手动添加的 JDK 通过「移除」删除列表记录。删除正在使用的 JDK
  前先停止服务，或切换到其他 JDK 并重启服务。

## 服务管理与更新

- **启动**。用已选 JDK 和当前配置启动 JAR，由 Go 接管 Java 子进程。通过 `/api/ping` 检测服务就绪，概览和托盘同步显示启动中、运行中、已停止或启动失败。
- **停止与重启**。将 `run.sh` 的循环逻辑移植到 Go。允许重启时，子进程退出码为 `0` 则重新拉起，以兼容 Web UI
  的重启约定。主动停止时先取消重启，再结束桌面端持有的进程并等待退出。桌面端主动重启按停止、启动顺序执行。异常退出时显示原因及日志。 [现有启动脚本](../ani-rss-macos/ani-rss.app/Contents/MacOS/run.sh#L39) · [现有重启接口](../ani-rss-application/src/main/java/ani/rss/controller/AboutController.java#L36)
- **首次安装与更新**。首次启动缺少 JAR 时从项目 GitHub Releases 下载。更新先将新 JAR 下载到临时文件并校验，再停止旧进程、备份并替换
  JAR。原先在运行的服务更新后重新启动，启动失败时回滚并尝试恢复旧版本。原先已停止的服务更新后继续保持停止。桌面托管模式下统一协调桌面端、Web
  UI 及定时更新，防止并发替换同一个
  JAR。 [现有下载脚本](../docker/run.sh#L10) · [现有更新逻辑](../ani-rss-application/src/main/java/ani/rss/service/UpdateService.java#L54)

## 托盘、日志与开机自启

- **托盘**。关闭窗口时隐藏到托盘，服务继续运行。托盘菜单提供显示主窗口、打开 Web UI、启动、重启、停止和退出。主动退出桌面端时停止它管理的服务。Linux
  托盘不可用时保留窗口入口。 [MyGo 托盘文档](https://mygo.run/zh/docs/menus)
- **日志**。Go 实时采集 Java 标准输出和错误输出。日志页支持搜索、复制、打开日志目录，并可在服务停止或启动失败后查看最近日志。
- **开机自启**。对接 macOS、Windows 和 Linux 的用户登录自启机制。启用后，用户登录系统时桌面端后台启动，并按已保存的配置拉起
  ani-rss。

## 密码与概览

- **修改密码**。服务在线时通过现有登录和设置接口修改 Web UI
  密码，成功后用新密码重新建立桌面端会话。 [登录接口](../ani-rss-application/src/main/java/ani/rss/controller/LoginController.java#L24) · [设置接口](../ani-rss-application/src/main/java/ani/rss/controller/ConfigController.java#L29)
- **订阅与下载**。通过 `/api/listAni` 统计订阅数量，通过 `/api/torrentsInfos`
  展示当前下载任务数、名称、进度和状态。服务离线、会话失效和下载器不可用时显示对应提示。 [订阅接口](../ani-rss-application/src/main/java/ani/rss/controller/AniController.java#L55) · [下载接口](../ani-rss-application/src/main/java/ani/rss/controller/TorrentsInfosController.java#L16)

## 实施顺序

1. 搭建 MyGo 与 Vuetify 项目，先在 macOS 打通 JDK 列表、下载、手动添加、删除和选用，再接入启动设置及服务启动。
2. 完成停止、重启、托盘、日志、JAR 首次安装和更新回滚。
3. 接入登录、密码修改、订阅统计和下载概览。
4. 完成 macOS、Windows 和 Linux 的开机自启、进程退出适配和发布包。

代码中的方法与关键逻辑按要求添加注释。
