# Zashiki App Icon — 设计方案

## 设计理念

| 概念 | 说明 |
|------|------|
| **文件夹** | 文件管理器最直观的隐喻 |
| **Z 字形折线** | Zashiki 的首字母，同时象征文件路径/目录树的折叠展开 |
| **和室美学** | Zashiki = 座敷（日式房间），底板采用低饱和深色，传递"有序、安静、整理"的感觉 |
| **蓝色渐变** | 科技感 + 信任感，与文件管理器的精确、可靠特质一致 |

---

## 方案 A：SVG（已写入仓库）

文件路径：`build/appicon.icon/Assets/zashiki_icon.svg`

- 深色圆角底板
- 文件夹造型（梯形顶 + 矩形身）作为主体
- 内部的横线条代表文件列表
- 白色 "Z" 字形折线压在前方（Zashiki + 路径折线双重隐喻）

### 如何使用

1. 将 SVG 替换到 `build/appicon.icon/icon.json` 的 `image-name` 字段：
2. 运行 `wails3 generate icons`（或通过 Taskfile 中的构建命令）自动输出各平台格式
3. 替换 `build/appicon.png`（1024x1024 主图标）

---

## 方案 B：AI 绘图 Prompt（生成高精度位图）

### Midjourney Prompt

```
App icon design for "Zashiki" file explorer, minimal flat style, square app icon with rounded corners, dark navy blue background (#1a1a2e), a folder shape in gradient blue-purple (#6C63FF to #3F8CFF), inside the folder subtle horizontal lines representing file list, a bold white "Z" folded shape crossing the folder as a path/metaphor, clean vector style, no text, modern tech aesthetic, high contrast, 8k, --ar 1:1 --style raw --v 6
```

### DALL-E Prompt

```
A modern app icon for a file explorer application called "Zashiki". Clean minimal design, square with rounded corners on dark navy background. A blue-purple gradient folder shape dominates the center. Inside the folder, subtle horizontal lines suggest a file listing. A bold white folded "Z"-shaped line crosses the folder, representing both the letter Z and a folder path. Vector style, no text, sleek, professional, tech aesthetic.
```

---

## 方案 C：自定义建议

如果你有自己偏好的风格（扁平 / 拟物 / 渐变 / 纯符号），或者想换颜色（比如品牌色），告诉我就好，我可以更新 SVG 和 prompt。
