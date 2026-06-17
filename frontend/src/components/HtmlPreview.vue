<script setup lang="ts">
import { computed } from 'vue'
import type { FilePreview } from '../../bindings/zashiki/internal/filemanager'

const props = defineProps<{
  preview: FilePreview
}>()

/**
 * Convert a relative path to an absolute path based on the HTML file's directory.
 */
function resolvePath(baseDir: string, refPath: string): string {
  // Already absolute
  if (refPath.startsWith('/') || refPath.match(/^[A-Za-z]:/)) {
    return refPath
  }
  // Handle protocol-relative or absolute URLs - don't transform
  if (refPath.startsWith('//') || refPath.includes('://')) {
    return refPath
  }
  // Handle data: URLs
  if (refPath.startsWith('data:')) {
    return refPath
  }

  const parts = baseDir.split('/')
  const refParts = refPath.split('/')

  // Remove trailing empty string if baseDir ends with /
  if (parts.length > 0 && parts[parts.length - 1] === '') {
    parts.pop()
  }

  for (const part of refParts) {
    if (part === '..') {
      if (parts.length > 1) parts.pop()
    } else if (part !== '.' && part !== '') {
      parts.push(part)
    }
  }

  return parts.join('/')
}

/**
 * Encode a local file path for the asset proxy URL.
 * Uses base64 URL encoding compatible with Go's base64.URLEncoding.
 */
function encodeAssetUrl(localPath: string): string {
  // Encode the path as UTF-8 bytes, then base64 URL-encode (with padding)
  const utf8Bytes = new TextEncoder().encode(localPath)
  // Convert bytes to binary string for btoa
  let binaryString = ''
  for (const byte of utf8Bytes) {
    binaryString += String.fromCharCode(byte)
  }
  // base64 encode with URL-safe alphabet (replace + with -, / with _)
  // Keep padding (=) since Go's URLEncoding expects it
  let encoded = btoa(binaryString)
  encoded = encoded.replace(/\+/g, '-').replace(/\//g, '_')
  return `/__html_assets__/${encoded}`
}

/**
 * Rewrite HTML content to proxy local asset references through the asset middleware.
 */
const processedHtml = computed(() => {
  const html = props.preview.content
  if (!html) return ''

  const filePath = props.preview.path
  // Get the directory of the HTML file
  const lastSlash = filePath.lastIndexOf('/')
  const baseDir = lastSlash >= 0 ? filePath.substring(0, lastSlash + 1) : '/'

  // Replace src/href attributes with relative or absolute local paths
  let result = html

  // Match src="..." and href="..." attributes
  result = result.replace(/(src|href)\s*=\s*["']([^"']+)["']/gi, (_match, attr: string, value: string) => {
    // Skip data: URLs, http/https URLs, # anchors, and protocol-relative URLs
    if (value.startsWith('data:') || value.startsWith('http://') || value.startsWith('https://') || value.startsWith('//') || value.startsWith('#') || value.startsWith('javascript:')) {
      return _match
    }

    const absolutePath = resolvePath(baseDir, value)
    const proxiedUrl = encodeAssetUrl(absolutePath)
    return `${attr}="${proxiedUrl}"`
  })

  // Also handle url() in inline styles
  result = result.replace(/url\(\s*["']?([^"')]+)["']?\s*\)/gi, (_match, value: string) => {
    if (value.startsWith('data:') || value.startsWith('http://') || value.startsWith('https://') || value.startsWith('//') || value.startsWith('#')) {
      return _match
    }

    const absolutePath = resolvePath(baseDir, value)
    const proxiedUrl = encodeAssetUrl(absolutePath)
    return `url("${proxiedUrl}")`
  })

  return result
})
</script>

<template>
  <iframe
    class="html-preview-frame"
    :srcdoc="processedHtml"
    sandbox="allow-scripts allow-same-origin"
    referrerpolicy="no-referrer"
  />
</template>

<style scoped>
.html-preview-frame {
  width: 100%;
  height: 100%;
  border: none;
  background: white;
}
</style>