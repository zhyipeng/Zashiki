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

export function mediaAssetUrl(localPath: string): string {
  return `/__media__/${encodeAssetPath(localPath)}`
}
