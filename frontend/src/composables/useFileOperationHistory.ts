import { computed, ref } from 'vue'
import { FileService } from '../../bindings/zashiki/internal/filemanager'
import type { EntryOperationResult, EntryPathPair } from '../../bindings/zashiki/internal/filemanager'
import { notifyDirectoriesChanged } from './useDirectoryEvents'
import { parentPath } from '../components/path'

type HistoryOperation =
  | { type: 'create-folder', path: string, affectedDirs: string[] }
  | { type: 'rename', beforePath: string, afterPath: string, beforeName: string, afterName: string, affectedDirs: string[] }
  | { type: 'copy', pairs: EntryPathPair[], affectedDirs: string[] }
  | { type: 'move', pairs: EntryPathPair[], affectedDirs: string[] }

const undoStack = ref<HistoryOperation[]>([])
const redoStack = ref<HistoryOperation[]>([])
const running = ref(false)

export function useFileOperationHistory(separator: () => string) {
  const canUndo = computed(() => undoStack.value.length > 0 && !running.value)
  const canRedo = computed(() => redoStack.value.length > 0 && !running.value)

  function recordCreateFolder(path: string) {
    pushOperation({
      type: 'create-folder',
      path,
      affectedDirs: dirsForPaths([path]),
    })
  }

  function recordRename(beforePath: string, afterPath: string) {
    pushOperation({
      type: 'rename',
      beforePath,
      afterPath,
      beforeName: basename(beforePath),
      afterName: basename(afterPath),
      affectedDirs: dirsForPaths([beforePath, afterPath]),
    })
  }

  function recordCopy(results: EntryOperationResult[]) {
    const pairs = pairsForSafeResults(results)
    if (pairs.length === 0) return
    pushOperation({
      type: 'copy',
      pairs,
      affectedDirs: uniqueStrings(pairs.map(pair => dirForPath(pair.targetPath))),
    })
  }

  function recordMove(results: EntryOperationResult[]) {
    const pairs = pairsForSafeResults(results)
    if (pairs.length === 0) return
    pushOperation({
      type: 'move',
      pairs,
      affectedDirs: uniqueStrings(pairs.flatMap(pair => [dirForPath(pair.sourcePath), dirForPath(pair.targetPath)])),
    })
  }

  async function undo() {
    if (!canUndo.value) return
    const operation = undoStack.value[undoStack.value.length - 1]
    running.value = true
    try {
      await undoOperation(operation)
      undoStack.value = undoStack.value.slice(0, -1)
      redoStack.value = [...redoStack.value, operation]
      notifyDirectoriesChanged(operation.affectedDirs)
    } finally {
      running.value = false
    }
  }

  async function redo() {
    if (!canRedo.value) return
    const operation = redoStack.value[redoStack.value.length - 1]
    running.value = true
    try {
      await redoOperation(operation)
      redoStack.value = redoStack.value.slice(0, -1)
      undoStack.value = [...undoStack.value, operation]
      notifyDirectoriesChanged(operation.affectedDirs)
    } finally {
      running.value = false
    }
  }

  function pushOperation(operation: HistoryOperation) {
    undoStack.value = [...undoStack.value, operation]
    redoStack.value = []
  }

  function pairsForSafeResults(results: EntryOperationResult[]): EntryPathPair[] {
    if (results.some(result => result.overwritten)) return []
    return results
      .filter(result => !result.skipped && result.sourcePath && result.targetPath)
      .map(result => ({ sourcePath: result.sourcePath, targetPath: result.targetPath }))
  }

  async function undoOperation(operation: HistoryOperation) {
    switch (operation.type) {
      case 'create-folder':
        await FileService.DeleteEmptyFolder(operation.path)
        return
      case 'rename':
        await FileService.RenameEntry(operation.afterPath, operation.beforeName)
        return
      case 'copy':
        await FileService.DeleteEntries(operation.pairs.map(pair => pair.targetPath))
        return
      case 'move':
        await FileService.MoveEntriesToTargets(reversePairs(operation.pairs))
        return
    }
  }

  async function redoOperation(operation: HistoryOperation) {
    switch (operation.type) {
      case 'create-folder':
        await FileService.CreateFolderAt(operation.path)
        return
      case 'rename':
        await FileService.RenameEntry(operation.beforePath, operation.afterName)
        return
      case 'copy':
        await FileService.CopyEntriesToTargets(operation.pairs)
        return
      case 'move':
        await FileService.MoveEntriesToTargets(operation.pairs)
        return
    }
  }

  function reversePairs(pairs: EntryPathPair[]): EntryPathPair[] {
    return pairs.map(pair => ({
      sourcePath: pair.targetPath,
      targetPath: pair.sourcePath,
    }))
  }

  function dirsForPaths(paths: string[]): string[] {
    return uniqueStrings(paths.map(dirForPath))
  }

  function dirForPath(path: string): string {
    return parentPath(path, separator()) || path
  }

  return {
    canUndo,
    canRedo,
    undo,
    redo,
    recordCreateFolder,
    recordRename,
    recordCopy,
    recordMove,
  }
}

function basename(path: string): string {
  const slashIndex = path.lastIndexOf('/')
  const backslashIndex = path.lastIndexOf('\\')
  const index = Math.max(slashIndex, backslashIndex)
  return index < 0 ? path : path.slice(index + 1)
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean)))
}
