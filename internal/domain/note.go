package domain

import (
	"strings"
	"unicode"
)

// 笔记：工作空间里的一棵树。文件夹可以装文件夹和笔记，笔记永远是叶子；
// 根级只放文件夹。一篇笔记有一个标题和一串从上到下的块：标题块（h1-h5）、
// 文本块、图片块、分隔线块。标题不是块：它存在节点行上，树里也用它当名字。
//
// 块的顺序由 sort_order 决定，0 在最上面。图片的尺寸如实存下来：原图宽高、
// 用户拖出来的缩放比例；显示宽度以正文宽度为上限，超过时按比例缩到正文宽。

// NoteKind 是一个树节点的两种形态。
const (
	NoteKindFolder = "folder"
	NoteKindNote   = "note"
)

// NoteBlockKind 是一种块的形态。
const (
	NoteBlockText    = "text"
	NoteBlockHeading = "heading"
	NoteBlockImage   = "image"
	// NoteBlockDivider 是一条通栏的横线。它不装文字：行还留着（块和行一一
	// 对应），但正文不长字，光标也从不落在它上面。
	NoteBlockDivider = "divider"
)

const (
	// NoteHeadingMinLevel 和 NoteHeadingMaxLevel 是标题块允许的级别。
	NoteHeadingMinLevel = 1
	NoteHeadingMaxLevel = 5
	// NoteTitleMaxRunes 是标题与文件夹名保留的码点数；更长的截断，避免树被
	// 一篇标题撑破。
	NoteTitleMaxRunes = 200
	// NoteDefaultTitle 是空标题在树与正文顶部的显示文字。
	NoteDefaultTitle = "未命名笔记"
	// NoteImageDefaultScale 是图片刚插入时的显示比例：原始像素大小。
	NoteImageDefaultScale = 1
	// NoteImageMinScale 防止一次拖拽把图片缩成看不见的一条。
	NoteImageMinScale = 0.05
	// NoteImageMaxScale 是拖拽能放大到的上限；再宽也会被正文宽度截住。
	NoteImageMaxScale = 4
)

// NoteNode 是笔记树的一个节点。ParentID 为空表示根级，根级只允许文件夹。
type NoteNode struct {
	ID        string
	ParentID  string
	Kind      string // folder / note
	Title     string
	SortOrder int
	Expanded  bool // 文件夹的折叠状态；笔记恒为 false
	CreatedAt int64
	UpdatedAt int64
}

// NoteBlock 是一篇笔记里的一个块。标题块的 Level 是 1..5，文本块为 0；
// 图片块用 ImagePath（相对工作空间的 data/assets）、原图宽高和缩放比例。
type NoteBlock struct {
	ID          string
	NoteID      string
	Kind        string // text / heading / image / divider
	Level       int    // 标题块 1..5；其它为 0
	Text        string // 标题块与文本块的文字
	ImagePath   string
	ImageWidth  int     // 原图像素宽
	ImageHeight int     // 原图像素高
	ImageScale  float64 // 显示比例，1 = 原始大小
	SortOrder   int
	CreatedAt   int64
	UpdatedAt   int64
}

// Note 是一篇笔记连同它的块，块按 SortOrder 从小到大。
type Note struct {
	Node   NoteNode
	Blocks []NoteBlock
}

// NoteDeleted 是一次删除带走的笔记与图片。NoteIDs 是被删掉的笔记节点，
// ImagePaths 是它们引用的图片（相对 data/assets），由调用方清理文件。
type NoteDeleted struct {
	NoteIDs    []string
	ImagePaths []string
}

// NoteTitle 把用户输入整理成存储用的标题：去空白、按码点截断。空标题保留空串，
// 显示时交给 NoteDisplayTitle 换成默认名。
func NoteTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	runes := []rune(title)
	if len(runes) > NoteTitleMaxRunes {
		runes = runes[:NoteTitleMaxRunes]
	}
	return string(runes)
}

// NoteDisplayTitle 是标题为空时树与正文顶部显示的名字。
func NoteDisplayTitle(title string) string {
	if strings.TrimSpace(title) == "" {
		return NoteDefaultTitle
	}
	return strings.TrimSpace(title)
}

// NoteHeadingLevel 把级别收进 1..5；0 或越界时按 1 处理，以便一块标题块永远
// 有可画的字号。
func NoteHeadingLevel(level int) int {
	if level < NoteHeadingMinLevel {
		return NoteHeadingMinLevel
	}
	if level > NoteHeadingMaxLevel {
		return NoteHeadingMaxLevel
	}
	return level
}

// NoteWordCount 数一段笔记正文里的字：一个汉字（以及假名、谚文）算一个字，一串
// 连续的字母或数字算一个词，空格、标点与符号都不算。页脚的「字数」用它，和把空格
// 标点都数进去的「字符数」是两个数。
func NoteWordCount(text string) int {
	words := 0
	inWord := false
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			inWord = false
		case noteCJK(r):
			words++
			inWord = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if !inWord {
				words++
				inWord = true
			}
		default:
			// 标点与符号分开两个词，自己不算一个字。
			inWord = false
		}
	}
	return words
}

// noteCJK 判断一个字符是不是来自不空格分词的文字，那种文字一个字就是一个词。
func noteCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul, unicode.Bopomofo)
}

// NoteImageScale 把拖拽比例收进可显示的范围；0 或负数当作默认比例。
func NoteImageScale(scale float64) float64 {
	if scale <= 0 {
		return NoteImageDefaultScale
	}
	if scale < NoteImageMinScale {
		return NoteImageMinScale
	}
	if scale > NoteImageMaxScale {
		return NoteImageMaxScale
	}
	return scale
}

// NoteImageSize 是图片在正文里的显示尺寸：原图乘缩放比例，宽度超过正文宽度
// （maxWidth）时按比例缩到正文宽。原图宽高未知（0）时返回 0，调用方先解码。
func NoteImageSize(width, height int, scale, maxWidth float64) (float64, float64) {
	if width <= 0 || height <= 0 || maxWidth <= 0 {
		return 0, 0
	}
	scale = NoteImageScale(scale)
	w := float64(width) * scale
	h := float64(height) * scale
	if w > maxWidth {
		h *= maxWidth / w
		w = maxWidth
	}
	return w, h
}
