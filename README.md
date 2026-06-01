# File Explorer

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

## 项目结构

```
file-explorer/
├── main.go              # 应用入口，窗口与服务注册
├── Taskfile.yml         # task 编排 (dev/build/run)
├── build/               # 各平台构建配置
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
