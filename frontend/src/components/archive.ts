export function isArchiveName(name: string): boolean {
  return /\.(zip|7z|rar|tar|tgz|tbz2|txz|tzst|tar\.(gz|bz2|xz|zst|lz4))$/i.test(name)
}

export function archiveFolderName(name: string): string {
  return name.replace(/\.(tar\.(gz|bz2|xz|zst|lz4)|zip|7z|rar|tar|tgz|tbz2|txz|tzst)$/i, '') || '解压文件'
}

export function archiveParent(path: string): string {
  const index = path.lastIndexOf('/')
  return index < 0 ? '' : path.slice(0, index)
}
