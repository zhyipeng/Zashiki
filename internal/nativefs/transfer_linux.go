//go:build linux || freebsd || openbsd || netbsd

package nativefs

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// linuxTransfer 基于文本剪贴板约定（text/uri-list）与 CLI 剪贴板工具
// （xclip → xsel 回退）实现文件引用剪贴板。
//
// 与 darwin/windows 不同，Linux 没有系统级「多文件引用」剪贴板语义；
// 这是 GNOME/文件管理器之间的惯例近似：
//   - 复制：text/uri-list 写入 file:// URI 列表
//   - 剪切：x-special/gnome-copied-files 写入 "cut\nfile://..."（Nautilus 约定），
//     同时写入 text/uri-list 供不支持该格式的程序读取
//
// 已知限制（调研中已记录）：非 GNOME 文件管理器可能不识别移动语义。
type linuxTransfer struct{}

func newPlatformTransfer() FileTransfer {
	return &linuxTransfer{}
}

// Copy 以复制语义写入剪贴板（text/uri-list）。
func (t *linuxTransfer) Copy(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths to copy")
	}
	uriList := buildURIText(paths)
	return writeClipboardText(uriList, "text/uri-list")
}

// Cut 以移动语义写入剪贴板（x-special/gnome-copied-files + text/uri-list）。
func (t *linuxTransfer) Cut(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no paths to cut")
	}
	uriList := buildURIText(paths)
	gnome := "cut\n" + uriList
	return writeClipboardText(gnome, "x-special/gnome-copied-files")
}

// ClipboardFiles 读取剪贴板文件引用（text/uri-list 或 GNOME 格式）。
func (t *linuxTransfer) ClipboardFiles() (ClipboardContent, error) {
	content := ClipboardContent{Paths: []string{}, Op: ClipboardCopy}

	// 优先读 GNOME 格式（带移动语义）
	gnome, err := readClipboardText("x-special/gnome-copied-files")
	if err == nil && strings.TrimSpace(gnome) != "" {
		if move, paths := parseGNOMEText(gnome); len(paths) > 0 {
			content.Paths = paths
			if move {
				content.Op = ClipboardMove
			}
			return content, nil
		}
	}

	// 回退：text/uri-list
	uriList, err := readClipboardText("text/uri-list")
	if err == nil && strings.TrimSpace(uriList) != "" {
		content.Paths = parseURIText(uriList)
	}
	return content, nil
}

// CurrentSequence 返回剪贴板内容指纹（截断 hash）作为变更序号。
// Linux 没有 GetClipboardSequenceNumber；用内容 hash 近似，前端据此判断
// 剪贴板是否被外部改写。
func (t *linuxTransfer) CurrentSequence() (uint64, error) {
	uriList, err := readClipboardText("text/uri-list")
	if err != nil {
		return 0, err
	}
	return fnv64a([]byte(uriList)), nil
}

// ClearClipboard 清空剪贴板（写入空内容）。
func (t *linuxTransfer) ClearClipboard() error {
	return writeClipboardText("", "text/uri-list")
}

// StartDrag 启动原生拖出。Linux 暂无系统级原生拖出（GTK WebKit 拖拽
// 语义与 Finder/Explorer 不同）；返回 ErrNotImplemented 并保留能力扩展点。
func (t *linuxTransfer) StartDrag(paths []string, x, y int, effects DropEffect) (DropEffect, error) {
	return 0, ErrNotImplemented
}

// ---- 文本编解码 ----

// buildURIText 生成 text/uri-list 内容（每行一个 file:// URI）。
func buildURIText(paths []string) string {
	var b bytes.Buffer
	for _, p := range paths {
		u := url.URL{Scheme: "file", Path: p}
		b.WriteString(u.String())
		b.WriteString("\r\n")
	}
	return b.String()
}

// parseURIText 解析 text/uri-list（忽略注释与空行）。
func parseURIText(text string) []string {
	var paths []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, "\r") {
			line = strings.TrimSuffix(line, "\r")
		}
		u, err := url.Parse(line)
		if err != nil || u.Scheme != "file" {
			continue
		}
		paths = append(paths, u.Path)
	}
	return paths
}

// parseGNOMEText 解析 x-special/gnome-copied-files：
// 第一行 "cut" 或 "copy"，后续每行一个 file:// URI。
func parseGNOMEText(text string) (move bool, paths []string) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 0 {
		return false, nil
	}
	op := strings.ToLower(strings.TrimSpace(lines[0]))
	rest := strings.Join(lines[1:], "\n")
	return op == "cut", parseURIText(rest)
}

// fnv64a 简单哈希用于剪贴板内容指纹。
func fnv64a(data []byte) uint64 {
	const offset = 14695981039346656037
	const prime = 1099511628211
	h := uint64(offset)
	for _, b := range data {
		h ^= uint64(b)
		h *= prime
	}
	return h
}

// ---- CLI 剪贴板工具 ----

// clipboardTool 探测可用的剪贴板 CLI（xclip 优先，xsel 回退）。
func clipboardTool() (name string, args []string, err error) {
	if _, err := exec.LookPath("xclip"); err == nil {
		return "xclip", nil, nil
	}
	if _, err := exec.LookPath("xsel"); err == nil {
		return "xsel", nil, nil
	}
	return "", nil, fmt.Errorf("no clipboard tool found (install xclip or xsel)")
}

// writeClipboardText 用剪贴板工具写入指定 MIME 的文本。
func writeClipboardText(text string, mime string) error {
	tool, _, err := clipboardTool()
	if err != nil {
		return err
	}
	var cmd *exec.Cmd
	if tool == "xclip" {
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", mime)
	} else {
		// xsel 不支持 -t；以 text/uri-list 写入（xsel 无 MIME 区分）
		cmd = exec.Command("xsel", "--clipboard", "--input")
	}
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clipboard write failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// readClipboardText 用剪贴板工具读取指定 MIME 的文本。
func readClipboardText(mime string) (string, error) {
	tool, _, err := clipboardTool()
	if err != nil {
		return "", err
	}
	var cmd *exec.Cmd
	if tool == "xclip" {
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", mime, "-o")
	} else {
		cmd = exec.Command("xsel", "--clipboard", "--output")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("clipboard read failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
