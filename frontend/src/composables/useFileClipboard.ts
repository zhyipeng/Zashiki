import { computed, ref } from 'vue'

export type FileClipboardMode = 'copy' | 'cut'

interface FileClipboardState {
  paths: string[]
  mode: FileClipboardMode
}

const clipboard = ref<FileClipboardState | null>(null)

export function useFileClipboard() {
  const hasClipboard = computed(() => !!clipboard.value && clipboard.value.paths.length > 0)

  function setClipboard(paths: string[], mode: FileClipboardMode) {
    clipboard.value = { paths: [...paths], mode }
  }

  function clearClipboard() {
    clipboard.value = null
  }

  return {
    clipboard,
    hasClipboard,
    setClipboard,
    clearClipboard,
  }
}
