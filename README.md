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
- **安全**：链接携带随机访问令牌，每次启动服务重新生成；无令牌请求一律 404 并限速防爆破；下载路径不暴露本地目录结构。
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
