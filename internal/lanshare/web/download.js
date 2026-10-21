const DB_NAME = 'zashiki-lanshare-downloads'
const DB_VERSION = 1
const TASK_STORE = 'tasks'
const CHUNK_STORE = 'chunks'

export const DOWNLOAD_CHUNK_SIZE = 8 * 1024 * 1024
export const DOWNLOAD_MAX_CONCURRENCY = 4
export const DOWNLOAD_MAX_RETRIES = 3
export const DOWNLOAD_MAX_CHUNKED_SIZE = 256 * 1024 * 1024

const activeDownloads = new Map()

export class DownloadError extends Error {
  constructor(message, code = 'download') {
    super(message)
    this.name = 'DownloadError'
    this.code = code
  }
}

export class DownloadCancelledError extends DownloadError {
  constructor() {
    super('下载已取消', 'cancelled')
    this.name = 'DownloadCancelledError'
  }
}

export class DownloadStaleError extends DownloadError {
  constructor() {
    super('共享文件已发生变化，请重新下载', 'stale')
    this.name = 'DownloadStaleError'
  }
}

export function planRanges(size, chunkSize = DOWNLOAD_CHUNK_SIZE) {
  if (!Number.isSafeInteger(size) || size < 0) {
    throw new DownloadError('文件大小无效', 'metadata')
  }
  if (!Number.isSafeInteger(chunkSize) || chunkSize <= 0) {
    throw new DownloadError('分片大小无效', 'configuration')
  }

  const ranges = []
  for (let start = 0, index = 0; start < size; start += chunkSize, index += 1) {
    const end = Math.min(size - 1, start + chunkSize - 1)
    ranges.push({ index, start, end, length: end - start + 1 })
  }
  return ranges
}

export function completedBytes(ranges, completed) {
  const completedSet = completed instanceof Set ? completed : new Set(completed || [])
  return ranges.reduce((total, part) => total + (completedSet.has(part.index) ? part.length : 0), 0)
}

export function parseContentRange(value) {
  const match = /^bytes\s+(\d+)-(\d+)\/(\d+|\*)$/i.exec(String(value || '').trim())
  if (!match) return null
  return {
    start: Number(match[1]),
    end: Number(match[2]),
    total: match[3] === '*' ? null : Number(match[3]),
  }
}

export function shouldUseChunkedDownload(size, options = {}) {
  const maxSize = options.maxSize ?? DOWNLOAD_MAX_CHUNKED_SIZE
  const indexedDBAvailable = options.indexedDBAvailable ?? typeof indexedDB !== 'undefined'
  return indexedDBAvailable && Number.isSafeInteger(size) && size > 0 && size <= maxSize
}

export function downloadModeLabel(mode, parallelism = 0) {
  if (mode === 'native') return '浏览器原生下载'
  if (mode === 'chunked' && parallelism > 1) return `并发分片 · ${parallelism} 路`
  if (mode === 'chunked') return 'Range 分片 · 1 路'
  return '检测中'
}

export function isRetryableStatus(status) {
  return status === 408 || status === 425 || status === 429 || status >= 500
}

function currentScope() {
  if (typeof location === 'undefined') return '/'
  return location.pathname.replace(/\/$/, '') || '/'
}

export function taskKeyForItem(itemId, scope = currentScope()) {
  return `${scope}|${itemId}`
}

function chunkKey(taskKey, index) {
  return `${taskKey}|${index}`
}

function downloadUrl(itemId) {
  return `download/${encodeURIComponent(itemId)}`
}

export function basename(name) {
  const normalized = String(name || '').replaceAll('\\', '/')
  const value = normalized.slice(normalized.lastIndexOf('/') + 1)
  return value || 'download'
}

function openDatabase() {
  if (typeof indexedDB === 'undefined') {
    return Promise.reject(new DownloadError('浏览器不支持 IndexedDB', 'storage'))
  }
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(TASK_STORE)) {
        db.createObjectStore(TASK_STORE, { keyPath: 'key' })
      }
      if (!db.objectStoreNames.contains(CHUNK_STORE)) {
        db.createObjectStore(CHUNK_STORE, { keyPath: 'key' })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(new DownloadError('无法打开浏览器下载存储', 'storage'))
  })
}

function requestResult(request) {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error || new DownloadError('浏览器存储操作失败', 'storage'))
  })
}

async function getTask(key) {
  const db = await openDatabase()
  try {
    const tx = db.transaction(TASK_STORE, 'readonly')
    return await requestResult(tx.objectStore(TASK_STORE).get(key))
  } finally {
    db.close()
  }
}

async function getTasks() {
  const db = await openDatabase()
  try {
    const tx = db.transaction(TASK_STORE, 'readonly')
    const tasks = await requestResult(tx.objectStore(TASK_STORE).getAll())
    return tasks.sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0))
  } finally {
    db.close()
  }
}

async function putTask(task) {
  const db = await openDatabase()
  try {
    const tx = db.transaction(TASK_STORE, 'readwrite')
    tx.objectStore(TASK_STORE).put(task)
    await new Promise((resolve, reject) => {
      tx.oncomplete = resolve
      tx.onerror = () => reject(tx.error || new DownloadError('无法保存下载进度', 'storage'))
      tx.onabort = () => reject(tx.error || new DownloadError('无法保存下载进度', 'storage'))
    })
  } catch (err) {
    if (err?.name === 'QuotaExceededError') {
      throw new DownloadError('浏览器存储空间不足', 'storage')
    }
    throw err
  } finally {
    db.close()
  }
}

async function putChunkAndTask(task, part, blob) {
  const db = await openDatabase()
  try {
    const tx = db.transaction([TASK_STORE, CHUNK_STORE], 'readwrite')
    tx.objectStore(CHUNK_STORE).put({ key: chunkKey(task.key, part.index), data: blob })
    tx.objectStore(TASK_STORE).put(task)
    await new Promise((resolve, reject) => {
      tx.oncomplete = resolve
      tx.onerror = () => reject(tx.error || new DownloadError('无法保存下载分片', 'storage'))
      tx.onabort = () => reject(tx.error || new DownloadError('无法保存下载分片', 'storage'))
    })
  } catch (err) {
    if (err?.name === 'QuotaExceededError') {
      throw new DownloadError('浏览器存储空间不足', 'storage')
    }
    throw err
  } finally {
    db.close()
  }
}

async function getChunk(taskKey, index) {
  const db = await openDatabase()
  try {
    const tx = db.transaction(CHUNK_STORE, 'readonly')
    const record = await requestResult(tx.objectStore(CHUNK_STORE).get(chunkKey(taskKey, index)))
    return record?.data || null
  } finally {
    db.close()
  }
}

async function deleteTaskRecord(task) {
  if (!task) return
  const db = await openDatabase()
  try {
    const tx = db.transaction([TASK_STORE, CHUNK_STORE], 'readwrite')
    tx.objectStore(TASK_STORE).delete(task.key)
    for (let index = 0; index < task.parts; index += 1) {
      tx.objectStore(CHUNK_STORE).delete(chunkKey(task.key, index))
    }
    await new Promise((resolve, reject) => {
      tx.oncomplete = resolve
      tx.onerror = () => reject(tx.error || new DownloadError('无法清理下载缓存', 'storage'))
      tx.onabort = () => reject(tx.error || new DownloadError('无法清理下载缓存', 'storage'))
    })
  } finally {
    db.close()
  }
}

async function storageHasRoom(bytes) {
  if (typeof navigator === 'undefined' || !navigator.storage?.estimate) return true
  try {
    const estimate = await navigator.storage.estimate()
    if (!Number.isFinite(estimate.quota) || !Number.isFinite(estimate.usage)) return true
    const available = estimate.quota - estimate.usage
    return available >= Math.ceil(bytes * 1.05)
  } catch (_) {
    return true
  }
}

async function fetchMetadata(item, signal) {
  let response
  try {
    response = await fetch(downloadUrl(item.id), { method: 'HEAD', cache: 'no-store', signal })
  } catch (err) {
    if (signal.aborted) throw new DownloadCancelledError()
    throw new DownloadError('无法读取文件信息', 'network')
  }
  if (!response.ok) {
    if (response.status === 404) throw new DownloadError('文件已不可用', 'missing')
    throw new DownloadError(`读取文件信息失败（${response.status}）`, 'http')
  }
  const contentLengthHeader = response.headers.get('Content-Length')
  const contentLength = contentLengthHeader === null ? NaN : Number(contentLengthHeader)
  const size = Number.isSafeInteger(contentLength) && contentLength >= 0
    ? contentLength
    : Number(item.size)
  if (!Number.isSafeInteger(size) || size < 0) {
    throw new DownloadError('服务器返回的文件大小无效', 'metadata')
  }
  return {
    size,
    etag: response.headers.get('ETag') || '',
    mimeType: response.headers.get('Content-Type') || item.mimeType || 'application/octet-stream',
  }
}

function wait(ms, signal) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms)
    if (!signal) return
    signal.addEventListener('abort', () => {
      clearTimeout(timer)
      reject(new DownloadCancelledError())
    }, { once: true })
  })
}

function validatePartResponse(response, part, total) {
  if (response.status !== 206) {
    throw new DownloadError(`服务器未返回分片数据（${response.status}）`, response.status >= 500 ? 'retry' : 'http')
  }
  const contentRange = parseContentRange(response.headers.get('Content-Range'))
  if (!contentRange || contentRange.start !== part.start || contentRange.end !== part.end || contentRange.total !== total) {
    throw new DownloadError('服务器返回了错误的分片范围', 'retry')
  }
  const contentLength = response.headers.get('Content-Length')
  if (contentLength !== null && Number(contentLength) !== part.length) {
    throw new DownloadError('服务器返回了错误的分片长度', 'retry')
  }
}

export async function fetchPart(item, metadata, part, signal, options, hooks) {
  const maxRetries = options.maxRetries ?? DOWNLOAD_MAX_RETRIES
  const headers = { Range: `bytes=${part.start}-${part.end}` }
  if (metadata.etag) headers['If-Match'] = metadata.etag

  for (let attempt = 0; attempt <= maxRetries; attempt += 1) {
    if (signal.aborted) throw new DownloadCancelledError()
    try {
      const response = await fetch(downloadUrl(item.id), {
        method: 'GET',
        headers,
        cache: 'no-store',
        signal,
      })
      if (response.status === 412) throw new DownloadStaleError()
      if (!response.ok && !isRetryableStatus(response.status)) {
        throw new DownloadError(`下载分片失败（${response.status}）`, 'http')
      }
      validatePartResponse(response, part, metadata.size)
      const blob = await response.blob()
      if (blob.size !== part.length) {
        throw new DownloadError('下载分片长度不一致', 'retry')
      }
      return blob
    } catch (err) {
      if (signal.aborted || err instanceof DownloadCancelledError) throw new DownloadCancelledError()
      if (err instanceof DownloadStaleError) throw err
      const retryable = err?.code === 'retry' || err?.code === 'network' || err?.name === 'TypeError'
      if (!retryable || attempt >= maxRetries) throw err
      hooks.onRetry?.({ part, attempt: attempt + 1, maxRetries })
      await wait(Math.min(2000, 250 * (2 ** attempt)), signal)
    }
  }
  throw new DownloadError('下载分片失败', 'network')
}

function defaultNativeDownload(itemId) {
  if (typeof window !== 'undefined' && window.location) {
    window.location.href = downloadUrl(itemId)
  }
}

function startNativeDownload(itemId, options) {
  const downloader = typeof options.nativeDownload === 'function'
    ? options.nativeDownload
    : defaultNativeDownload
  downloader(itemId)
}

async function triggerBlobDownload(item, ranges, task, hooks) {
  const chunks = []
  for (const part of ranges) {
    const chunk = await getChunk(task.key, part.index)
    if (!chunk) throw new DownloadError('下载分片缓存缺失，请重试', 'storage')
    chunks.push(chunk)
  }
  const blob = new Blob(chunks, { type: item.mimeType || 'application/octet-stream' })
  if (typeof document === 'undefined') return blob
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = basename(item.name)
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  setTimeout(() => URL.revokeObjectURL(url), 30_000)
  hooks.onState?.({
    phase: 'done',
    mode: 'chunked',
    parallelism: task.parallelism || 1,
    doneBytes: task.size,
    totalBytes: task.size,
  })
  return blob
}

async function runDownload(item, hooks, options, signal) {
  const maxSize = options.maxSize ?? DOWNLOAD_MAX_CHUNKED_SIZE
  const chunkSize = options.chunkSize ?? DOWNLOAD_CHUNK_SIZE
  const concurrency = Math.max(1, Math.min(options.concurrency ?? DOWNLOAD_MAX_CONCURRENCY, 8))
  const metadata = await fetchMetadata(item, signal)

  if (!shouldUseChunkedDownload(metadata.size, {
    maxSize,
    indexedDBAvailable: typeof indexedDB !== 'undefined',
  })) {
    startNativeDownload(item.id, options)
    hooks.onState?.({ phase: 'native', mode: 'native', doneBytes: 0, totalBytes: metadata.size })
    return { mode: 'native', size: metadata.size }
  }

  const ranges = planRanges(metadata.size, chunkSize)
  const key = taskKeyForItem(item.id, options.scope)
  let task = await getTask(key)
  const expectedParts = ranges.length
  if (
    task &&
    (task.size !== metadata.size || task.etag !== metadata.etag || task.chunkSize !== chunkSize || task.parts !== expectedParts)
  ) {
    await deleteTaskRecord(task)
    task = null
  }
  if (!task) {
    task = {
      key,
      itemId: item.id,
      name: item.name,
      size: metadata.size,
      etag: metadata.etag,
      chunkSize,
      parts: expectedParts,
      completed: [],
      updatedAt: Date.now(),
    }
  }

  const completed = new Set((task.completed || []).filter(index => Number.isInteger(index) && index >= 0 && index < expectedParts))
  const remainingBytes = metadata.size - completedBytes(ranges, completed)
  if (!(await storageHasRoom(remainingBytes))) {
    await deleteTaskRecord(task)
    startNativeDownload(item.id, options)
    hooks.onState?.({ phase: 'native', mode: 'native', doneBytes: 0, totalBytes: metadata.size })
    return { mode: 'native', size: metadata.size }
  }

  task.completed = Array.from(completed).sort((a, b) => a - b)
  task.parallelism = Math.min(concurrency, ranges.length)
  task.updatedAt = Date.now()
  await putTask(task)
  hooks.onState?.({
    phase: 'running',
    mode: 'chunked',
    parallelism: task.parallelism,
    doneBytes: completedBytes(ranges, completed),
    totalBytes: metadata.size,
    completedParts: completed.size,
    totalParts: ranges.length,
  })

	let nextIndex = 0
	let persistQueue = Promise.resolve()
	const persistPart = (part, blob) => {
		const snapshot = {
			...task,
			completed: Array.from(completed).sort((a, b) => a - b),
			updatedAt: Date.now(),
		}
		const operation = persistQueue.then(() => putChunkAndTask(snapshot, part, blob))
		persistQueue = operation.catch(() => {})
		return operation
	}
	const worker = async () => {
    while (true) {
      const index = nextIndex
      nextIndex += 1
      if (index >= ranges.length) return
      if (completed.has(index)) continue
      const part = ranges[index]
      const blob = await fetchPart(item, metadata, part, signal, options, hooks)
      completed.add(index)
      task.completed = Array.from(completed).sort((a, b) => a - b)
      task.updatedAt = Date.now()
      await persistPart(part, blob)
      hooks.onState?.({
        phase: 'running',
        mode: 'chunked',
        parallelism: task.parallelism,
        doneBytes: completedBytes(ranges, completed),
        totalBytes: metadata.size,
        completedParts: completed.size,
        totalParts: ranges.length,
      })
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, ranges.length) }, worker))
  if (signal.aborted) throw new DownloadCancelledError()
  await triggerBlobDownload(item, ranges, task, hooks)
  await deleteTaskRecord(task)
  return { mode: 'chunked', size: metadata.size }
}

export function startDownload(item, hooks = {}, options = {}) {
  const existing = activeDownloads.get(item.id)
  if (existing) return existing

  const controller = new AbortController()
  const promise = runDownload(item, hooks, options, controller.signal)
    .catch(async (err) => {
      if (err?.code === 'storage') {
        const task = await getTask(taskKeyForItem(item.id, options.scope)).catch(() => null)
        await deleteTaskRecord(task).catch(() => {})
        startNativeDownload(item.id, options)
        hooks.onState?.({ phase: 'native', mode: 'native', doneBytes: 0, totalBytes: item.size })
        return { mode: 'native', size: item.size }
      }
      hooks.onState?.({ phase: err?.code === 'cancelled' ? 'paused' : 'error', error: err?.message || '下载失败' })
      throw err
    })
    .finally(() => activeDownloads.delete(item.id))
  const handle = {
    promise,
    cancel: () => controller.abort(),
    controller,
  }
  activeDownloads.set(item.id, handle)
  return handle
}

export async function listDownloadTasks() {
  return getTasks()
}

export async function discardDownload(itemId, scope) {
  const task = await getTask(taskKeyForItem(itemId, scope))
  await deleteTaskRecord(task)
}

export function nativeDownload(itemId) {
  defaultNativeDownload(itemId)
}
