export const findModeLabelAlphabet = 'asdfghjklqwertyuiopzxcvbnm'

export interface FindModeSource {
  path: string
  name: string
}

export interface FindModeTarget<T extends FindModeSource = FindModeSource> {
  entry: T
  path: string
  label: string
  index: number
  searchTokens: string[]
}

export type FindModePinyinResolver = (text: string, pattern: 'pinyin' | 'first') => string

export function createFindModeTargets<T extends FindModeSource>(
  entries: readonly T[],
  pinyinResolver?: FindModePinyinResolver,
  alphabet = findModeLabelAlphabet,
): FindModeTarget<T>[] {
  const labelLength = findModeLabelLength(entries.length, alphabet.length)
  return entries.map((entry, index) => ({
    entry,
    path: entry.path,
    label: findModeLabelForIndex(index, labelLength, alphabet),
    index,
    searchTokens: findModeSearchTokens(entry.name, pinyinResolver),
  }))
}

export function findModeLabelLength(count: number, alphabetSize: number): number {
  if (count <= 0) return 0
  if (alphabetSize <= 1) return count

  let length = 1
  let capacity = alphabetSize
  while (capacity < count) {
    length++
    capacity *= alphabetSize
  }
  return length
}

export function findModeLabelForIndex(index: number, labelLength: number, alphabet = findModeLabelAlphabet): string {
  if (labelLength <= 0 || alphabet.length === 0) return ''
  if (alphabet.length === 1) return alphabet[0].repeat(index + 1)

  const base = alphabet.length
  let value = index
  const chars = Array.from({ length: labelLength }, () => alphabet[0])
  for (let i = labelLength - 1; i >= 0; i--) {
    chars[i] = alphabet[value % base]
    value = Math.floor(value / base)
  }
  return chars.join('')
}

export function normalizeFindModeText(text: string): string {
  return text.toLowerCase().replace(/\s+/g, '')
}

export function findModeSearchTokens(name: string, pinyinResolver?: FindModePinyinResolver): string[] {
  const tokens = [normalizeFindModeText(name)]
  if (pinyinResolver) {
    tokens.push(normalizeFindModeText(pinyinResolver(name, 'pinyin')))
    tokens.push(normalizeFindModeText(pinyinResolver(name, 'first')))
  }
  return Array.from(new Set(tokens.filter(Boolean)))
}

export function findModeTargetMatches(target: FindModeTarget, query: string): boolean {
  const normalizedQuery = normalizeFindModeText(query)
  if (!normalizedQuery) return true
  return target.label.startsWith(normalizedQuery) || target.searchTokens.some(token => token.includes(normalizedQuery))
}

export function findModeMatchedTargets<T extends FindModeSource>(
  targets: readonly FindModeTarget<T>[],
  query: string,
): FindModeTarget<T>[] {
  return targets.filter(target => findModeTargetMatches(target, query))
}

export function exactFindModeLabelMatch<T extends FindModeSource>(
  targets: readonly FindModeTarget<T>[],
  query: string,
): FindModeTarget<T> | null {
  const normalizedQuery = normalizeFindModeText(query)
  if (!normalizedQuery) return null
  return targets.find(target => target.label === normalizedQuery) || null
}

