/**
 * Encode a local file path for the asset proxy URL.
 * Uses base64 URL encoding compatible with Go's base64.URLEncoding.
 */
export function encodeAssetPath(localPath: string): string {
  // Encode the path as UTF-8 bytes, then base64 URL-encode (with padding)
  const utf8Bytes = new TextEncoder().encode(localPath)
  // Convert bytes to binary string for btoa
  let binaryString = ''
  for (const byte of utf8Bytes) {
    binaryString += String.fromCharCode(byte)
  }
  // base64 encode with URL-safe alphabet (replace + with -, / with _)
  // Keep padding (=) since Go's URLEncoding expects it
  return btoa(binaryString).replace(/\+/g, '-').replace(/\//g, '_')
}

export function htmlAssetUrl(localPath: string): string {
  return `/__html_assets__/${encodeAssetPath(localPath)}`
}

export function thumbnailUrl(localPath: string, size = 256): string {
  return `/__thumbnails__/${encodeAssetPath(localPath)}?s=${size}`
}

// mediastream 流式服务器的根地址（含随机令牌），应用启动时由 main.ts 注入。
// 大文件（音视频/Office/PDF）必须走这条通道——Windows 上 Wails 资源管线
// 会把整个响应体缓冲进内存，无法流式传输。
let mediaStreamBase = ''

export function setMediaStreamBase(base: string) {
  mediaStreamBase = base.replace(/\/+$/, '')
}

/** 构建流式资源 URL；服务器未就绪时返回空串。 */
export function mediaStreamUrl(localPath: string): string {
  if (!mediaStreamBase) return ''
  return `${mediaStreamBase}/${encodeAssetPath(localPath)}`
}
