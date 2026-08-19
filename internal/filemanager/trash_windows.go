//go:build windows

package filemanager

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	foDelete          = 0x0003
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofNoErrorUI      = 0x0400
	fofSilent         = 0x0004
)

var (
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procSHFileOperation = shell32.NewProc("SHFileOperationW")
)

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

func getTrashInfo() TrashInfo {
	return TrashInfo{Label: "回收站", Available: true}
}

func trashEntry(_ context.Context, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	from := append(syscall.StringToUTF16(abs), 0)
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofNoErrorUI | fofSilent,
	}
	ret, _, _ := procSHFileOperation.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return "", syscall.Errno(ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return "", os.ErrPermission
	}
	return "", nil
}

func openTrash() error {
	return exec.Command("explorer.exe", "shell:RecycleBinFolder").Start()
}
