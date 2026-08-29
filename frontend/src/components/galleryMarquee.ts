/**
 * 看图模式鼠标框选（marquee）的纯几何/选中逻辑。
 * 坐标一律使用网格内容坐标（含滚动偏移），与 DOM 解耦便于测试。
 */

export interface MarqueePoint {
  x: number
  y: number
}

export interface MarqueeRect {
  left: number
  top: number
  right: number
  bottom: number
}

export interface MarqueeItemRect {
  path: string
  left: number
  top: number
  width: number
  height: number
}

/** 位移超过该阈值才算框选，否则视为空白处单击。 */
export const marqueeDragThreshold = 3

export function isMarqueeDrag(start: MarqueePoint, current: MarqueePoint, threshold = marqueeDragThreshold): boolean {
  return Math.abs(current.x - start.x) > threshold || Math.abs(current.y - start.y) > threshold
}

export function marqueeRect(start: MarqueePoint, current: MarqueePoint): MarqueeRect {
  return {
    left: Math.min(start.x, current.x),
    top: Math.min(start.y, current.y),
    right: Math.max(start.x, current.x),
    bottom: Math.max(start.y, current.y),
  }
}

export function rectsIntersect(rect: MarqueeRect, item: MarqueeItemRect): boolean {
  return item.left < rect.right
    && item.left + item.width > rect.left
    && item.top < rect.bottom
    && item.top + item.height > rect.top
}

/**
 * 依据框选矩形计算选中集合。
 * - 非 additive：选中集 = 与矩形相交的条目（替换原选中）。
 * - additive（按住 Ctrl）：相交条目相对拖拽开始时的选中态做切换，其余保持原选中。
 */
export function applyMarqueeSelection(
  baseSelected: string[],
  items: MarqueeItemRect[],
  rect: MarqueeRect,
  additive: boolean,
): string[] {
  if (!additive) {
    return items.filter(item => rectsIntersect(rect, item)).map(item => item.path)
  }
  const result = new Set(baseSelected)
  for (const item of items) {
    if (!rectsIntersect(rect, item)) continue
    if (result.has(item.path)) {
      result.delete(item.path)
    } else {
      result.add(item.path)
    }
  }
  return Array.from(result)
}
