# Zashiki

跨平台文件资源管理器，基于 Wails v3 + Vue 3 + TypeScript 构建。

## 技术栈

| 层 | 技术 |
|---|---|
| 桌面框架 | Wails v3 |
| 后端 | Go |
| 前端 | Vue 3 + TypeScript + Naive UI |
| 构建 | Vite + bun.js |
| 任务编排 | Task (go-task) |

## 快速开始

```bash
# 启动开发模式（前端 + 后端热重载）
task dev

# 构建应用
task build

# 仅构建前端
cd frontend && bun run build
```

## 测试

```bash
# 前端测试（vitest）
cd frontend && bun run test

# Go 后端测试
go test ./...
```

## 快传（局域网传输）

内置局域网快传：接收端**无需安装任何软件**，用浏览器打开链接/扫码即可。

- **入口**：顶栏"快传"视图；或在文件列表选中文件后右键"局域网分享…"快速加入。
- **发送**：分享文件（图片在网页端可预览）、文本；对端浏览器可逐个下载或打包 zip 下载。
- **接收**：对端浏览器可上传文件到接收目录、发送文本（快传视图内一键复制）。
- **安全**：默认链接携带随机访问码，每次启动服务重新生成；也可在快传连接区设置固定短访问码（1–32 位英文字母、数字、下划线或连字符），留空恢复随机；无令牌请求一律 404 并限速防爆破；下载路径不暴露本地目录结构。
- **手动连接**：例如设置访问码 `abc` 后，对端输入 `http://192.168.1.10:53100/abc` 即可访问（IP 和端口以界面显示为准）。修改访问码立即生效，旧链接失效，共享内容保留。
- **配置**：设置中可修改端口（默认 53100，被占用时自动回退随机端口）与接收目录（默认 `下载/Zashiki`）。

平台说明：

- **Windows**：首次启动服务时系统防火墙会弹出允许访问提示，请勾选"专用网络"。
- **macOS**：首次使用时系统会询问"本地网络"权限，请允许（Info.plist 已声明 `NSLocalNetworkUsageDescription`）。
- 传输为局域网明文 HTTP，请勿在不信任的网络中使用。

## 项目结构

```
zashiki/
├── main.go              # 应用入口，窗口与服务注册
├── Taskfile.yml         # task 编排 (dev/build/run)
├── build/               # 各平台构建配置
├── internal/
│   ├── lanshare/        # 局域网快传 (HTTP 服务 + 内嵌接收网页)
│   └── ...              # 其他服务包
├── frontend/
│   ├── src/
│   │   ├── main.ts      # Vue 入口
│   │   ├── App.vue      # 根组件
│   │   └── components/  # 组件
│   ├── bindings/        # Wails 自动生成的 TS 绑定
│   └── dist/            # 构建产物 (embed 到 Go binary)
└── bin/                 # 编译产物
```

更多约定见 [AGENTS.md](AGENTS.md)。

## 自动更新

使用 Wails `v3.0.0-beta.34`，需要 Go 1.26 或更新版本。正式应用启动时检查当前仓库的 GitHub Releases，之后每 6 小时检查一次；设置底部也可手动检查。发现新版本后显示版本说明，确认后才下载，SHA-256 校验通过后自动重启安装。网络检查失败不会打断启动；下载或校验失败不会重启。

发布流程保留首次安装包，并追加以下更新包和 `SHA256SUMS.txt`：

- macOS：`Zashiki-darwin-universal-update.zip`，仅包含一个 `Zashiki.app`，支持 Intel 和 Apple Silicon。
- Windows：`Zashiki-windows-amd64-update.zip`，仅包含 `Zashiki.exe`。
- Linux：`Zashiki-linux-amd64-update.tar.gz`，仅包含便携二进制 `Zashiki`，运行环境需要相应 GTK/WebKit 系统库。

开发模式关闭自动更新。Linux 的 deb/rpm 系统安装及 AppImage 使用发布页或包管理器更新，避免把应用本体写进错误的安装位置。不受支持的架构会显示发布页入口。应用安装目录需要当前用户具有写入权限。

推送 `v1.2.3` 形式的版本标签会触发现有发布工作流，CI 用标签同步应用和安装包版本。带预发布后缀的标签会标为 prerelease，默认更新检查只读取正式版本。首次使用本次接入的版本仍需手动安装，之后的新版本才支持应用内更新。

## macOS 毛玻璃

macOS 使用原生 `NSVisualEffectView` 提供全窗口磨砂毛玻璃，支持深浅主题。`task dev`、`task build` 和 Docker 交叉构建已启用 Wails 的 `private_mac_apis` 构建标签：该版本框架需要通过 WebKit 的非公开 `drawsBackground` 属性关闭 WebView 的实色背景。直接使用 Go 命令运行时也需添加此标签，例如 `go run -tags private_mac_apis .`。修改窗口材质或构建标签后需要重新编译并重启原生进程，前端热更新不会生效。

该非公开 API 不适用于 Mac App Store 发布；后续若要上架，需要重新评估透明方案。
