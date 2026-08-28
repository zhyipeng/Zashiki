### p0 - 必须做的
- [x] 主题切换
  - [x] 至少支持 dark mode
- [x] 图标
- [x] 扩充字段
- [x] 排序
- [x] 键盘操作
- [x] 重构后端代码
- [x] 回收站

### p1 - 慢慢迭代
- [ ] 构建
- [x] 预览
  - [x] 编辑
- [x] 搜索
- [x] 路径补全
- [x] 撤销/重做
  - [x] bug:删除后无法撤销
  - [x] vim 快捷键
- [x] 默认终端、编辑器
- [x] 快速访问自定义
- [x] 状态栏
- [ ] 命令行参数
- [ ] windows 右键菜单

### p2 - 没那么重要的
  - [ ] 自定义快捷键
  - [ ] 更多主题
  - [x] 和系统文件资源管理器交互
    - [x] 系统文件拖入（WindowFilesDropped + data-file-drop-target）
    - [x] Ctrl+C/X → 系统剪贴板（darwin/windows/linux）
    - [x] 从系统剪贴板粘贴（含 Finder 复制到 Zashiki）
    - [x] 原生拖出（Wails → Finder/Explorer）
      - [x] darwin：prepare + LeftMouseDragged monitor + beginDraggingSession
      - [x] windows：SHCreateDataObject + SHDoDragDrop（OLE STA 线程）
      - [ ] linux：GTK 拖出（暂无系统级方案，保留能力点）
  - [x] 文件传输
    - [x] 局域网快传（浏览器即接收端：文件/文本互传、zip 打包、扫码/链接访问）
      - [ ] 增强：HTTPS、传输历史、上传前确认

### p3 - 暂时没想到解决方案
  - [x] FileTable 空白区域
