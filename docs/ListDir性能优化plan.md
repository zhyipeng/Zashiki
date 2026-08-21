# ListDir 性能优化方案：首屏分页 + 后台补齐 + syscall 削减

## 问题与依据（已验证）

`ListDir`（`internal/filemanager/fileservice.go:11`）对每个 entry 调用 `os.Lstat(fullPath)`：
- N 个文件 = N 次 lstat（每次做**全路径解析**）+ N 次 `filepath.Join` 分配
- 全量 JSON 序列化 + 传输 + 前端整体赋值，超大目录（node_modules、10 万+条）首屏阻塞明显
- Windows 上每条还额外有一次 `GetFileAttributes`（隐藏判断）syscall

Go 源码验证（go1.25.8，遵循「不凭经验、看源码」约束）：
- darwin/linux：`File.Readdir(-1)` 走 `fstatat(dirfd, name)`（`os/statat_unix.go:13`），相对目录 fd，免全路径解析
- Windows：`File.Readdir(-1)` 的 FileInfo 直接来自 `GetFileInformationByHandleEx` 缓冲区（`os/dir_windows.go:96`），**零额外 syscall**，且 `Sys().(*syscall.Win32FileAttributeData).FileAttributes` 含隐藏属性（`os/types_windows.go:276`）
- `os.ReadDir` 已按名称排序（`os/dir.go:122`）；`unixDirent` 持有的 parent 是字符串，`File.Close()` 后调用 `Info()` 安全，**dirent 可跨分页调用缓存**

## 方案总览（用户已确认：首屏分页+后台补齐；纳入 dirent 缓存）

### 一、后端：分页 API + 枚举缓存（新文件 `internal/filemanager/pagedlist.go`）

**1. 新增返回模型**（追加到 `model.go`）：
```go
type DirPage struct {
    Entries []FileEntry `json:"entries"`
    Total   int         `json:"total"`
}
```

**2. 新增方法**（ListDir 保留不动，供 Sidebar 树用）：
```go
func (f *FileService) ListDirPage(path string, offset int, limit int) (DirPage, error)
```
- offset<0 / limit<=0 / limit>5000 归一化（默认 limit 500，上限 5000）
- 返回 `{entries: 分页切片, total: 目录总条数}`；offset 超界返回空 entries + 正确 total

**3. 枚举缓存**（`FileService` 增加字段，指针注册不变）：
```go
type FileService struct {
    mu         sync.Mutex
    dirCache   map[string]*dirCacheEntry // path -> dirents
    cacheOrder []string                  // LRU 序
}
const dirCacheLimit = 16 // 目录数上限，~100B/条
```
- `loadSortedDirents(path)`：先 `os.Stat(path)` 校验目录 mtime（**1 次 syscall**）：
  - 命中且 mtime 未变 → 复用缓存的 `[]fs.DirEntry`（名称+类型）
  - 未命中 → `os.ReadDir(path)` 枚举（unix 上惰性零 stat）→ 缓存 → 更新 LRU
- slice 分页天然共享底层数组，跨页无重复拷贝

**4. 每页构建 FileEntry**（`fileEntryFromDirEntry`）：
```go
info, err := entry.Info()  // unix: fstatat(fd相对)；Windows: 0 syscall
if err != nil { continue } // 与现状一致：竞态消失的条目跳过
```
- size/modTime 每页现查，数据始终新鲜，无需缓存失效
- symlink 仍走 `os.Readlink`（保持现状，通常占比小）

**5. Windows 隐藏判断优化**（`fileservice_hidden_windows.go` 新增）：
```go
func isHiddenInfo(info os.FileInfo) bool {
    if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
        return d.FileAttributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
    }
    return isHiddenEntry("", path)
}
```
每条省一次 `GetFileAttributes` syscall。unix 路径：`isHiddenInfo` 直接用 `name[0]=='.'`（与现状一致）。

**6. 边界情况**：
- 条目读取中途消失（`Info()` 失败）→ 跳过该条但计入 total（与现状「continue」一致）
- 缓存 mtime 校验失败/目录被删 → 重新枚举（自然降级）
- mtime 秒级/毫秒级文件系统精度差异 → 仅影响缓存新鲜度，不影响正确性（分页期间目录变更本就由前端 `useDirectoryEvents` refresh 兜底）

### 二、前端：首屏分页 + 后台补齐（FileTable.vue）

**1. loadDir 改造**（`FileTable.vue:70`）：
```ts
const PAGE_SIZE = 500
async function loadDir(p: string) {
  loading.value = true; entries.value = []; hasMore.value = false
  const first = await FileService.ListDirPage(p, 0, PAGE_SIZE)
  entries.value = first.entries
  hasMore.value = first.entries.length < first.total
  loading.value = false                    // 首屏立即解锁
  fillRemaining(p, first.entries.length, first.total)  // 后台补齐，不 await
}
async function fillRemaining(p: string, from: number, total: number) {
  for (let offset = from; offset < total; offset += PAGE_SIZE) {
    if (loadGeneration !== currentGeneration) return  // 路径切换竞态保护
    const page = await FileService.ListDirPage(p, offset, PAGE_SIZE)
    if (loadGeneration !== currentGeneration) return
    entries.value = entries.value.concat(page.entries)  // concat 新数组，规避 inline cache 抖动
  }
  hasMore.value = false
}
```
- `loadGeneration` 计数器：每次 `loadDir` / `refresh` 递增，旧循环自动终止（与 `pathAutocompleteRequestId` 同模式）
- 补齐期间 `visibleEntries` 每页重算一次（500 条/页的粒度，vue 计算属性天然节流）

**2. 状态新增**：`hasMore = ref(false)`；补齐期间工具栏显示轻量加载指示（复用现有 `NSpin` 小尺寸）

**3. 现有交互逐一兼容**（验证过的关键路径）：
- `refresh()`（目录变更事件）→ 重新走分页 loadDir，行为不变
- rename/delete 的原地数组替换（`:1299/:1367`）→ 都生成新数组，与 concat 追加无冲突
- 光标恢复 `cursorMemory`、`End` 键 `selectLastEntry`、`scrollTo({index})` → 补齐完成后均正常；补齐期间 End/全选仅作用于已加载部分（Finder 同款行为，可接受）
- 搜索/排序/隐藏过滤仍全量客户端，语义不变
- 路径自动补全（`:598`）与 Sidebar 树 → **继续用原 ListDir**，零改动

**4. 绑定更新**：`wails3 generate bindings -ts` 重新生成（`bindings/` 自动产物，不手改）

### 三、测试（AGENTS.md 要求全覆盖）

**后端**（追加到 `fileservice_test.go`，沿用 `t.TempDir()` + 标准 testing 风格）：
- `TestFileService_ListDirPage_Pagination`：建 1200 个文件，验证 total、页切片、offset 越界返回空
- `TestFileService_ListDirPage_ConsistentOrdering`：与 ListDir 结果逐条对比
- `TestFileService_ListDirPage_CacheReuse`：重复调用同目录，验证 mtime 变化后枚举刷新（目录中新建文件后再查，total 变化）
- `TestFileService_ListDirPage_NotFound` / 超大 limit 归一化
- symlink 条目 linkTarget 正确性
- hidden 文件标记（unix：dotfile）

**前端**（新文件 `src/components/dirPaging.ts`，纯逻辑抽取 + 测试，符合现有约定——不 mock Wails 绑定、不 mount 组件）：
```ts
export function nextPageRange(loaded: number, total: number, pageSize: number): { offset: number, limit: number } | null
export function shouldContinueFetch(loaded: number, total: number): boolean
```
- 覆盖：恰好整页、余数页、total=0、offset 越界、pageSize 边界

### 四、性能预期

| 场景 | 现状 | 优化后 |
|---|---|---|
| darwin 10 万条目录首屏 | ~2-4s 全量阻塞 | **~40ms**（500 条 fstatat + 序列化） |
| Windows 大目录 | N×(Lstat+GetFileAttributes) | **0 stat syscall**（缓冲区自带） |
| 后退/前进重访目录 | 全量重新枚举+stat | 枚举复用（1 次 mtime 校验），仅重查当页 Info |
| node_modules 打开 | 卡顿数秒 | 首屏即时，后台补齐 |

### 五、实施顺序

1. `model.go` 加 DirPage → `pagedlist.go` 实现 ListDirPage + 缓存 → hidden windows 优化
2. Go 测试全绿（`go test ./internal/filemanager/`）
3. `wails3 generate bindings -ts`
4. `dirPaging.ts` 纯逻辑 + 测试 → FileTable.vue 改造 loadDir
5. 前端 `bun run test` + `vue-tsc` 类型检查通过
6. 手动验证：大目录（node_modules）首屏/补齐/搜索/排序/选择/End 键

### 六、明确不做（本轮范围外）

- Windows `GetFileAttributes` 之外的 stat 语义变更（`Info()` 的 Windows 行为已是最优）
- Sidebar 树 / 自动补全切换到分页 API（数据量小，无收益）
- 后端排序/过滤（客户端全量排序语义保留）
- fsnotify 实时监听（前端 `notifyDirectoriesChanged` 事件总线已覆盖刷新场景）