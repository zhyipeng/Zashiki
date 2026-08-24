import { computed, ref } from 'vue'
import { FileTransferService } from '../../bindings/zashiki/internal/nativefs'
import type { ClipboardMode, ClipboardSnapshot } from './clipboardState'
import { cutPathSetOf, hasPasteableContent, snapshotFor } from './clipboardState'

// useFileClipboard 维护「系统剪贴板」的 UI 镜像。
//
// 关键变化（调研结论）：系统剪贴板是唯一事实来源。Ctrl+C/X 先写入系统
// 剪贴板（FileTransferService.Copy/Cut），成功后这里只记录 (paths, mode,
// seq) 快照用于 UI 半透明显示；粘贴时也从系统剪贴板读取。
//
// seq（剪贴板变更序号）用于检测外部改写：定期/粘贴前调用
// ClipboardSequence()，若与本地记录不一致则清空镜像，避免 UI 一直显示
// 早已失效的剪切状态。

const clipboard = ref<ClipboardSnapshot | null>(null)

export function useFileClipboard() {
  const hasClipboard = computed(() => hasPasteableContent(clipboard.value))
  const cutPathSet = computed(() => cutPathSetOf(clipboard.value))

  /** 写入系统剪贴板成功后记录 UI 镜像（mode 只在写成功后才可信）。 */
  function recordClipboard(paths: string[], mode: ClipboardMode, seq: number) {
    clipboard.value = snapshotFor(paths, mode, seq)
  }

  /** 写入系统剪贴板（复制/剪切），成功才更新 UI 镜像；返回是否成功。 */
  async function copyToSystem(paths: string[], mode: ClipboardMode): Promise<boolean> {
    if (paths.length === 0) return false
    try {
      if (mode === 'cut') {
        await FileTransferService.Cut(paths)
      } else {
        await FileTransferService.Copy(paths)
      }
      const seq = await FileTransferService.ClipboardSequence()
      recordClipboard(paths, mode, seq)
      return true
    } catch (err) {
      console.error('Write to system clipboard failed:', err)
      return false
    }
  }

  /** 查询系统剪贴板当前序号，外部改写时清空本地镜像。 */
  async function refreshSequence() {
    try {
      const seq = await FileTransferService.ClipboardSequence()
      if (clipboard.value && clipboard.value.seq !== seq) {
        clipboard.value = null
      }
    } catch {
      // 查询失败不阻断
    }
  }

  /** 粘贴成功后清空镜像（cut 已消费；copy 也应清以免误导）。 */
  function clearClipboard() {
    clipboard.value = null
  }

  return {
    clipboard,
    hasClipboard,
    cutPathSet,
    copyToSystem,
    refreshSequence,
    clearClipboard,
  }
}
