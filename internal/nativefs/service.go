package nativefs

// FileTransferService 是暴露给前端（Wails binding）的文件剪贴板/拖拽服务。
// 它委托给平台实现（darwin/windows/linux），前端只需调用 Copy/Cut/
// ClipboardFiles/ClipboardSequence，无需感知平台差异。
type FileTransferService struct {
	transfer FileTransfer
}

// NewFileTransferService 创建服务实例并注入平台实现。
func NewFileTransferService() *FileTransferService {
	return &FileTransferService{
		transfer: NewFileTransfer(),
	}
}

// Copy 将 paths 指向的文件引用以复制语义写入系统剪贴板。
func (s *FileTransferService) Copy(paths []string) error {
	return s.transfer.Copy(paths)
}

// Cut 将 paths 指向的文件引用以移动语义写入系统剪贴板。
func (s *FileTransferService) Cut(paths []string) error {
	return s.transfer.Cut(paths)
}

// ClipboardFiles 返回系统剪贴板中的文件引用与操作语义。
func (s *FileTransferService) ClipboardFiles() (ClipboardContent, error) {
	return s.transfer.ClipboardFiles()
}

// ClipboardSequence 返回系统剪贴板变更序号，供前端判断剪贴板是否被外部改写。
func (s *FileTransferService) ClipboardSequence() (uint64, error) {
	return s.transfer.CurrentSequence()
}

// ClearClipboard 清空系统剪贴板（剪切粘贴完成后消费剪切态）。
func (s *FileTransferService) ClearClipboard() error {
	return s.transfer.ClearClipboard()
}

// StartDrag 启动原生拖出，阻塞直到拖拽结束。返回最终 effect（copy/move/link），
// 供前端判断是否需要刷新源目录（如 move 后源文件已消失）。
// x/y 为前端指针坐标（CSS 逻辑点，左上原点），用于合成平台拖拽事件。
func (s *FileTransferService) StartDrag(paths []string, x, y int) (DropEffect, error) {
	return s.transfer.StartDrag(paths, x, y, DropEffectCopy|DropEffectMove)
}
