# AGENTS.md

## 项目概述

跨平台文件资源管理器，基于 Wails v3 构建桌面 GUI 应用。目前处于 Wails3 demo 阶段，目标是扩展为完整的文件管理工具。

**重要：**
- Wails 3 还处于 alpha 阶段，使用时应避免经验判断，而是根据文档和框架源码进行开发。
- 开发时需优先考虑性能优化。
- 新增或修改代码应为其编写覆盖测试用例。
- 考虑代码质量和可维护性。
- 做任何功能都需要考虑跨平台兼容性。

## 技术栈

| 层 | 技术 | 版本 |
|---|---|---|
| 桌面框架 | Wails v3 | `v3.0.0-alpha.97` |
| 后端语言 | Go | `1.25.0` |
| 前端运行时 | bun.js | latest |
| 前端语言 | TypeScript | `^4.9.3` |
| 前端框架 | Vue 3 | `^3.2.45` |
| UI 组件库 | Naive UI | `^2.44.1` |
| 构建工具 | Vite | `^8.0.5` |
| 类型检查 | vue-tsc | `^1.0.11` |
| 任务编排 | Task (go-task) | `3.x` |

## 项目结构

```
file-explorer/
├── main.go                  # 应用入口，窗口创建，事件注册
├── fileservice.go           # 文件系统 Service（ListDir / GetFileInfo / GetHomeDir / GetSeparator）
├── go.mod / go.sum          # Go 模块定义
├── Taskfile.yml             # 顶层 task 编排（dev/build/run 等）
├── build/                   # 平台构建配置（darwin/windows/linux/ios/android）
├── frontend/
│   ├── src/
│   │   ├── main.ts          # Vue 入口，createApp
│   │   ├── App.vue          # 根组件
│   │   └── components/      # Sidebar（目录树）、FileTable（文件列表）
│   ├── bindings/            # Wails 自动生成的 TS 绑定（勿手动修改）
│   ├── public/              # 静态资源
│   ├── dist/                # 构建产物（embed 到 Go binary）
│   ├── vite.config.ts       # Vite + Vue + Wails 插件配置
│   └── tsconfig.json        # TypeScript 配置
└── bin/                     # 编译产物
```

## 开发命令

```bash
# 启动开发模式（wails3 dev，自动热重载）
task dev

# 构建前端（开发模式，不压缩）
cd frontend && bun run build:dev

# 构建前端（生产模式）
cd frontend && bun run build

# 构建应用
task build

# 仅运行前端 dev server
cd frontend && bun run dev
```

## 测试

### 前端 — Vitest + @vue/test-utils

```bash
cd frontend
bun run test          # 单次运行
bun run test:watch    # watch 模式
bun run test:coverage # 覆盖率报告
```

**技术选型理由**：Vitest 与 Vite 共享 transform pipeline 和配置，原生支持 Vue SFC、TypeScript、ESM，无需额外配置。

**测试文件约定**：
- 测试文件放在 `frontend/src/__tests__/` 或与源文件同目录的 `*.test.ts`
- 组件测试使用 `@vue/test-utils` 的 `mount`，仅测试公开 props/events/slots，不测试内部实现细节
- 纯逻辑/工具函数优先测试，组件测试覆盖关键交互路径

**约束**：
- 不要在测试中 mock 文件系统——使用临时目录（通过 `os.tmpdir()` / `mkdtemp`）
- 不要因为测试而破坏组件封装（不测试 `<script setup>` 内部未暴露的变量）
- Wails 的绑定层（`bindings/` 目录）由工具自动生成，无需为其编写测试

### 后端 — Go testing

```bash
go test ./...                    # 运行所有测试
go test -v -count=1 ./...        # 详细输出，禁用缓存
go test -coverprofile=coverage.out ./...  # 覆盖率
wails3 generate bindings -ts  # 生成 wails binding
```

**约束**：
- 测试文件命名为 `*_test.go`，与源文件同目录
- 优先使用标准库 `testing` 包，不引入第三方断言库
- Service 层测试使用真实 Go 结构体实例，不 mock 内部依赖
- 涉及文件系统的测试使用 `t.TempDir()` 创建临时目录

## 代码约定

### 通用
- 新功能尽量追加在已有代码后面，不做大规模重构
- 最小化改动范围，不为了美观调整无关代码
- 不得提交任何 secrets（.env、key、token、credentials、password）

### 前端（TypeScript + Vue 3）
- 使用 `<script setup lang="ts">` 语法
- 组件命名采用 PascalCase，文件名与组件名一致
- 使用 Naive UI 组件时，按需引入（tree-shaking 友好）
- Props 使用 `defineProps<{ ... }>()` 类型推导
- 不写无意义的注释，命名即文档

### 后端（Go）
- 遵循 Go 标准代码风格（`gofmt`）
- Service 结构体通过 `application.NewService()` 注册到 Wails
- Wails 绑定的方法必须为导出方法（PascalCase），且接收者为指针类型
- 新 Service 在 `main.go` 的 `Services` 列表中注册

### 依赖管理
- Go 依赖：`go mod tidy`
- 前端依赖：`bun install`（使用 bun 作为包管理器）
- 前端 lock 文件为 `bun.lock`
