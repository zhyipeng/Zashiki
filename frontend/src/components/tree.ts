export interface LeafNode {
  kind: 'leaf'
  id: number
  path: string
}

export interface SplitNode {
  kind: 'split'
  id: number
  direction: 'horizontal' | 'vertical'
  children: [TreeNode, TreeNode]
}

export type TreeNode = LeafNode | SplitNode

export function isLeaf(node: TreeNode): node is LeafNode {
  return node.kind === 'leaf'
}

export function createLeaf(id: number, path: string): LeafNode {
  return { kind: 'leaf', id, path }
}

export function splitLeaf(
  root: TreeNode,
  leafId: number,
  newLeafId: number,
  newSplitId: number,
  direction: 'horizontal' | 'vertical',
): TreeNode {
  if (isLeaf(root)) {
    if (root.id === leafId) {
      return {
        kind: 'split',
        id: newSplitId,
        direction,
        children: [root, createLeaf(newLeafId, root.path)],
      }
    }
    return root
  }
  return {
    ...root,
    children: root.children.map(c =>
      splitLeaf(c, leafId, newLeafId, newSplitId, direction),
    ) as [TreeNode, TreeNode],
  }
}

export function closeLeaf(root: TreeNode, leafId: number): TreeNode | null {
  if (isLeaf(root)) {
    return root.id === leafId ? null : root
  }
  const surviving = root.children
    .map(c => closeLeaf(c, leafId))
    .filter((c): c is TreeNode => c !== null)
  if (surviving.length === 0) return null
  if (surviving.length === 1) return surviving[0]
  return { ...root, children: surviving as [TreeNode, TreeNode] }
}

export function navigateLeaf(
  root: TreeNode,
  leafId: number,
  path: string,
): TreeNode {
  if (isLeaf(root)) {
    return root.id === leafId ? { ...root, path } : root
  }
  return {
    ...root,
    children: root.children.map(c => navigateLeaf(c, leafId, path)) as [
      TreeNode,
      TreeNode,
    ],
  }
}

export function getFirstLeafId(node: TreeNode): number {
  if (isLeaf(node)) return node.id
  return getFirstLeafId(node.children[0])
}

export function getLeafIds(node: TreeNode): number[] {
  if (isLeaf(node)) return [node.id]
  return [...getLeafIds(node.children[0]), ...getLeafIds(node.children[1])]
}

export function findLeafById(
  node: TreeNode,
  id: number,
): LeafNode | null {
  if (isLeaf(node)) return node.id === id ? node : null
  for (const child of node.children) {
    const found = findLeafById(child, id)
    if (found) return found
  }
  return null
}
