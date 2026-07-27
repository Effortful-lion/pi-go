// Package tui 提供终端 UI 能力——原始终端控制、颜色样式、行编辑与交互式对话。
//
// 本文件提供终端底层能力：原始模式、光标控制、ANSI 颜色/样式、单行编辑。
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// ============================================================================
// 原始模式 (Raw Mode)
// ============================================================================

// EnterRawMode 进入原始终端模式，返回恢复函数。
//
// 原始模式下终端逐字符输入，不回显，不用等 Enter。
// 必须通过 defer 调用返回的 restore 函数来恢复终端设置。
//
// 用法：
//
//	restore, err := tui.EnterRawMode()
//	if err != nil {
//	    // handle
//	}
//	defer restore()
func EnterRawMode() (restore func(), err error) {
	fd := int(os.Stdin.Fd())

	// 保存当前 termios
	var oldState syscall.Termios
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(fd),
		ioctlReadTermios, uintptr(unsafe.Pointer(&oldState)),
		0, 0, 0,
	); errno != 0 {
		return func() {}, fmt.Errorf("tcgetattr: %v", errno)
	}

	// 复制并修改
	newState := oldState
	// 关闭回显和规范模式
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	// 关闭输入处理
	newState.Iflag &^= syscall.ICRNL | syscall.INLCR | syscall.IGNCR
	// 关闭输出处理
	newState.Oflag &^= syscall.OPOST
	// 设置最小读取字节和超时
	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0

	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL, uintptr(fd),
		ioctlWriteTermios, uintptr(unsafe.Pointer(&newState)),
		0, 0, 0,
	); errno != 0 {
		return func() {}, fmt.Errorf("tcsetattr: %v", errno)
	}

	restore = func() {
		syscall.Syscall6(
			syscall.SYS_IOCTL, uintptr(fd),
			ioctlWriteTermios, uintptr(unsafe.Pointer(&oldState)),
			0, 0, 0,
		)
	}
	return restore, nil
}

const (
	ioctlReadTermios  = 0x40487413 // TIOCGETA on macOS
	ioctlWriteTermios = 0x80487414 // TIOCSETA on macOS
)

// ============================================================================
// ANSI Escape 基础
// ============================================================================

// ANSI escape 前缀。
const esc = "\033"

// ============================================================================
// 光标控制
// ============================================================================

// CursorUp 光标上移 n 行。
func CursorUp(n int) {
	fmt.Printf("%s[%dA", esc, n)
}

// CursorDown 光标下移 n 行。
func CursorDown(n int) {
	fmt.Printf("%s[%dB", esc, n)
}

// CursorForward 光标右移 n 列。
func CursorForward(n int) {
	fmt.Printf("%s[%dC", esc, n)
}

// CursorBack 光标左移 n 列。
func CursorBack(n int) {
	fmt.Printf("%s[%dD", esc, n)
}

// CursorMoveTo 将光标移动到指定行/列（1-based）。
func CursorMoveTo(row, col int) {
	fmt.Printf("%s[%d;%dH", esc, row, col)
}

// CursorHide 隐藏光标。
func CursorHide() {
	fmt.Printf("%s[?25l", esc)
}

// CursorShow 显示光标。
func CursorShow() {
	fmt.Printf("%s[?25h", esc)
}

// ClearLine 清除当前行（光标之后的内容）。
func ClearLine() {
	fmt.Printf("%s[2K", esc)
}

// ClearLineFromCursor 清除当前行从光标到行尾。
func ClearLineFromCursor() {
	fmt.Printf("%s[0K", esc)
}

// ClearScreen 清屏并将光标移到左上角。
func ClearScreen() {
	fmt.Printf("%s[2J", esc)
}

// LineStart 将光标移到行首。
func LineStart() {
	fmt.Printf("\r")
}

// ============================================================================
// 颜色与样式
// ============================================================================

// Color ANSI 标准色号。
type Color int

const (
	ColorDefault Color = 39
	ColorBlack   Color = 30
	ColorRed     Color = 31
	ColorGreen   Color = 32
	ColorYellow  Color = 33
	ColorBlue    Color = 34
	ColorMagenta Color = 35
	ColorCyan    Color = 36
	ColorWhite   Color = 37
	ColorGray    Color = 90
)

// Style 文本样式。
type Style struct {
	Fg        Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
}

// StyleText 用指定样式包裹文本，返回包含 ANSI escape 的字符串。
// 用法：fmt.Print(tui.StyleText("hello", tui.Style{Fg: tui.ColorRed, Bold: true}))
func StyleText(text string, s Style) string {
	codes := make([]string, 0, 4)

	if s.Bold {
		codes = append(codes, "1")
	}
	if s.Dim {
		codes = append(codes, "2")
	}
	if s.Italic {
		codes = append(codes, "3")
	}
	if s.Underline {
		codes = append(codes, "4")
	}
	if s.Fg != 0 && s.Fg != ColorDefault {
		codes = append(codes, fmt.Sprintf("%d", s.Fg))
	}

	if len(codes) == 0 {
		return text
	}

	ansi := esc + "["
	for i, c := range codes {
		if i > 0 {
			ansi += ";"
		}
		ansi += c
	}
	ansi += "m"

	return ansi + text + esc + "[0m"
}

// 便捷样式函数
func Bold(text string) string      { return StyleText(text, Style{Bold: true}) }
func Dim(text string) string       { return StyleText(text, Style{Dim: true}) }
func Italic(text string) string    { return StyleText(text, Style{Italic: true}) }
func Underline(text string) string { return StyleText(text, Style{Underline: true}) }
func Red(text string) string       { return StyleText(text, Style{Fg: ColorRed}) }
func Green(text string) string     { return StyleText(text, Style{Fg: ColorGreen}) }
func Blue(text string) string      { return StyleText(text, Style{Fg: ColorBlue}) }
func Cyan(text string) string      { return StyleText(text, Style{Fg: ColorCyan}) }
func Gray(text string) string      { return StyleText(text, Style{Fg: ColorGray}) }
func Yellow(text string) string    { return StyleText(text, Style{Fg: ColorYellow}) }
func Magenta(text string) string   { return StyleText(text, Style{Fg: ColorMagenta}) }

// ============================================================================
// 行编辑
// ============================================================================

const (
	keyEnter     = 13  // \r
	keyCtrlC     = 3   // Ctrl+C
	keyCtrlD     = 4   // Ctrl+D
	keyCtrlL     = 12  // Ctrl+L 清屏
	keyBackspace = 127 // \x7f
	keyDelete    = '\033' // 转义序列前缀
	keyTab       = 9
	maxHistory   = 1000
)

// LineEditor 多行文本编辑器，支持历史记录、多行编辑、补全、清屏。
type LineEditor struct {
	prompt  string
	history []string // 历史记录，去重，最新的在前
}

// NewLineEditor 创建行编辑器。
func NewLineEditor(prompt string) *LineEditor {
	return &LineEditor{
		prompt:  prompt,
		history: make([]string, 0, 64),
	}
}

// AddHistory 添加一条历史记录（自动去重、裁剪）。
func (le *LineEditor) AddHistory(entry string) {
	if entry == "" {
		return
	}
	// 去重：删除已存在的同一条
	for i, h := range le.history {
		if h == entry {
			le.history = append(le.history[:i], le.history[i+1:]...)
			break
		}
	}
	le.history = append(le.history, entry)
	if len(le.history) > maxHistory {
		le.history = le.history[1:]
	}
}

// ReadLine 读取一行输入（可能包含多行，通过 Alt+Enter 换行）。
// 返回输入文本和取消标志。
func (le *LineEditor) ReadLine() (string, bool) {
	CursorShow()
	defer CursorHide()

	// 编辑状态
	lines := [][]rune{{}}
	curLine := 0
	curCol := 0
	histIdx := -1 // -1=新输入，≥0=浏览历史
	var savedLines [][]rune // 进入历史浏览前的编辑内容

	// 初始渲染
	le.renderAll(lines, curLine, curCol)

	for {
		seq := readKey()

		// --- 单字符按键 ---
		if len(seq) == 1 {
			switch seq[0] {
			case keyEnter:
				if histIdx >= 0 {
					// 从历史中确认选择
					text := le.history[histIdx]
					fmt.Print("\r\n")
					return text, false
				}
				text := joinLines(lines)
				fmt.Print("\r\n")
				return text, false

			case keyCtrlC:
				lines = [][]rune{{}}
				curLine, curCol = 0, 0
				histIdx = -1
				if len(joinLines(lines)) == 0 {
					fmt.Print("\r\n")
					return "", true
				}
				le.renderAll(lines, curLine, curCol)
				continue

			case keyCtrlD:
				if histIdx >= 0 {
					continue
				}
				if len(lines) == 1 && len(lines[0]) == 0 {
					fmt.Print("\r\n")
					return "", true
				}
				if curCol < len(lines[curLine]) {
					lines[curLine] = append(lines[curLine][:curCol], lines[curLine][curCol+1:]...)
				}

			case keyCtrlL:
				ClearScreen()
				le.renderAll(lines, curLine, curCol)
				continue

			case keyBackspace:
				if histIdx >= 0 {
					continue
				}
				if curCol > 0 {
					lines[curLine] = append(lines[curLine][:curCol-1], lines[curLine][curCol:]...)
					curCol--
				} else if curLine > 0 {
					// 合并到上一行
					prevLen := len(lines[curLine-1])
					lines[curLine-1] = append(lines[curLine-1], lines[curLine]...)
					lines = append(lines[:curLine], lines[curLine+1:]...)
					curLine--
					curCol = prevLen
				}

			case keyTab:
				if histIdx >= 0 {
					continue
				}
				// Tab 路径补全
				le.handleTab(lines, &curLine, &curCol)

			default:
				// 普通文本插入
				if seq[0] >= 32 && seq[0] < 127 {
					if histIdx >= 0 {
						// 退出历史浏览，恢复之前编辑的内容
						if savedLines != nil {
							lines = savedLines
						} else {
							lines = [][]rune{{}}
						}
						curLine = 0
						curCol = 0
						histIdx = -1
						savedLines = nil
					}
					lines[curLine] = append(lines[curLine], 0)
					copy(lines[curLine][curCol+1:], lines[curLine][curCol:])
					lines[curLine][curCol] = rune(seq[0])
					curCol++
				}
			}

			le.renderAll(lines, curLine, curCol)
			continue
		}

		// --- 双字符 CSI 序列 (\033[ + 字母) ---
		if len(seq) == 2 && seq[0] == '[' {
			switch seq[1] {
			case 'D': // ←
				if curCol > 0 {
					curCol--
				}
			case 'C': // →
				if curCol < len(lines[curLine]) {
					curCol++
				}
			case 'A': // ↑
				if histIdx >= 0 {
					histIdx = le.historyPrev(histIdx)
					if histIdx >= 0 && histIdx < len(le.history) {
						lines = splitLines(le.history[histIdx])
						curLine = len(lines) - 1
						curCol = len(lines[curLine])
					}
				} else if curLine > 0 {
					curLine--
					if curCol > len(lines[curLine]) {
						curCol = len(lines[curLine])
					}
				} else {
					// 浏览历史
					if len(le.history) > 0 {
						histIdx = le.historyPrev(histIdx)
						if histIdx >= 0 && histIdx < len(le.history) {
							savedLines = copyLines(lines)
							lines = splitLines(le.history[histIdx])
							curLine = len(lines) - 1
							curCol = len(lines[curLine])
						}
					}
				}
			case 'B': // ↓
				if histIdx >= 0 {
					histIdx = le.historyNext(histIdx)
					if histIdx < 0 {
						// 退出历史浏览，恢复编辑内容
						lines = savedLines
						curLine = len(lines) - 1
						curCol = len(lines[curLine])
						savedLines = nil
					} else if histIdx < len(le.history) {
						lines = splitLines(le.history[histIdx])
						curLine = len(lines) - 1
						curCol = len(lines[curLine])
					}
				} else if curLine < len(lines)-1 {
					curLine++
					if curCol > len(lines[curLine]) {
						curCol = len(lines[curLine])
					}
				}
			case 'H': // Home
				curCol = 0
			case 'F': // End
				curCol = len(lines[curLine])
			}

			le.renderAll(lines, curLine, curCol)
			continue
		}

		// --- 三字符 CSI 序列 (\033[ + 数字 + ~) ---
		if len(seq) == 3 && seq[0] == '[' && seq[2] == '~' {
			switch seq[1] {
			case '3': // Delete
				if curCol < len(lines[curLine]) {
					lines[curLine] = append(lines[curLine][:curCol], lines[curLine][curCol+1:]...)
				}
			case '1': // Home (xterm)
				curCol = 0
			case '4': // End (xterm)
				curCol = len(lines[curLine])
			}
			le.renderAll(lines, curLine, curCol)
			continue
		}

		// --- Ctrl+← → (\033[1;5D / \033[1;5C) ---
		if len(seq) == 4 && seq[0] == '[' && seq[1] == '1' && seq[2] == ';' {
			switch seq[3] {
			case 'D':
				for curCol > 0 && lines[curLine][curCol-1] == ' ' {
					curCol--
				}
				for curCol > 0 && lines[curLine][curCol-1] != ' ' {
					curCol--
				}
			case 'C':
				for curCol < len(lines[curLine]) && lines[curLine][curCol] != ' ' {
					curCol++
				}
				for curCol < len(lines[curLine]) && lines[curLine][curCol] == ' ' {
					curCol++
				}
			}
			le.renderAll(lines, curLine, curCol)
			continue
		}

		// --- Alt+Enter (\033\r 或 \033\n) ---
		if len(seq) >= 2 && seq[0] == '\033' && (seq[1] == '\r' || seq[1] == '\n') {
			if histIdx >= 0 {
				continue
			}
			// 在光标处拆分为两行
			right := make([]rune, len(lines[curLine])-curCol)
			copy(right, lines[curLine][curCol:])
			lines[curLine] = lines[curLine][:curCol]
			// 插入新行
			lines = append(lines, nil)
			copy(lines[curLine+2:], lines[curLine+1:])
			lines[curLine+1] = right
			curLine++
			curCol = 0
			le.renderAll(lines, curLine, curCol)
			continue
		}
	}
}

// historyPrev 上一个历史条目（自动 clamp）。
func (le *LineEditor) historyPrev(current int) int {
	if len(le.history) == 0 {
		return -1
	}
	if current < 0 {
		// 从未浏览→浏览最新的
		return len(le.history) - 1
	}
	if current > 0 {
		return current - 1
	}
	return 0
}

// historyNext 下一个历史条目，返回 -1 表示退出浏览。
func (le *LineEditor) historyNext(current int) int {
	if current < 0 {
		return -1
	}
	if current+1 >= len(le.history) {
		return -1
	}
	return current + 1
}

// handleTab 执行路径补全。
func (le *LineEditor) handleTab(lines [][]rune, curLine *int, curCol *int) {
	prefix := pathPrefixAt(lines[*curLine], *curCol)
	if prefix == "" {
		return
	}
	completions := listCompletions(prefix)
	if len(completions) == 0 {
		return
	}
	if len(completions) == 1 {
		// 单个补全：替换前缀
		suffix := completions[0][len(prefix):]
		for _, r := range suffix {
			lines[*curLine] = append(lines[*curLine], 0)
			copy(lines[*curLine][*curCol+1:], lines[*curLine][*curCol:])
			lines[*curLine][*curCol] = r
			*curCol++
		}
		return
	}
	// 多个候选：找出公共前缀
	common := commonPrefix(completions)
	if len(common) > len(prefix) {
		suffix := common[len(prefix):]
		for _, r := range suffix {
			lines[*curLine] = append(lines[*curLine], 0)
			copy(lines[*curLine][*curCol+1:], lines[*curLine][*curCol:])
			lines[*curLine][*curCol] = r
			*curCol++
		}
		return
	}
	// 显示候选列表。简单实现：不做显示，让调用方处理。
}

// pathPrefixAt 获取光标前的路径前缀。
func pathPrefixAt(line []rune, col int) string {
	if col < 1 {
		return ""
	}
	// 向前扫描到空格或行首
	start := col - 1
	for start >= 0 && line[start] != ' ' && line[start] != '\t' {
		start--
	}
	start++
	return string(line[start:col])
}

// listCompletions 获取路径补全候选。
func listCompletions(prefix string) []string {
	dir, filePrefix := splitDirPrefix(prefix)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(filePrefix)) {
			match := filepath.Join(dir, name)
			if e.IsDir() {
				match += string(os.PathSeparator)
			}
			// 保持原始前缀的路径分隔符风格
			result = append(result, match)
		}
	}
	return result
}

// splitDirPrefix 将路径拆分为目录部分和文件名前缀。
func splitDirPrefix(path string) (string, string) {
	dir, file := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	return dir, file
}

// commonPrefix 找出一组字符串的公共前缀。
func commonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	p := strs[0]
	for _, s := range strs[1:] {
		for len(p) > 0 && !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
		}
	}
	return p
}

// renderAll 重绘制全部编辑行。
func (le *LineEditor) renderAll(lines [][]rune, curLine, curCol int) {
	fmt.Print("\r")
	// 清除所有行，然后重绘
	for i, line := range lines {
		ClearLine()
		if i == 0 {
			fmt.Print(le.prompt)
		} else {
			fmt.Print(strings.Repeat(" ", len(le.prompt)))
		}
		le.renderLine(line, i == curLine, curCol)
		if i < len(lines)-1 {
			fmt.Print("\r\n")
		}
	}
	// 清除多余行
	ClearLineFromCursor()
	// 光标归位到编辑行
	if curLine > 0 {
		// 需要上移 curLine 行
		CursorUp(curLine)
	}
	// 移动到当前行开头 + prompt 宽度 + curCol
	fmt.Print("\r")
	if curLine == 0 {
		fmt.Print(le.prompt)
	} else {
		fmt.Print(strings.Repeat(" ", len(le.prompt)))
	}
	CursorForward(curCol)
}

// renderLine 渲染单行内容。
func (le *LineEditor) renderLine(line []rune, isCurrentLine bool, curCol int) {
	for i, r := range line {
		if isCurrentLine && i == curCol {
			fmt.Printf("%s[7m%c%s[0m", esc, r, esc)
		} else {
			fmt.Printf("%c", r)
		}
	}
	if isCurrentLine && curCol == len(line) {
		fmt.Print(Blue("█"))
	}
}

// --- 兼容旧版 ---

// EditLine 单行文本编辑器（兼容旧 API）。
// 实际委托给 LineEditor，不支持历史记录。
func EditLine(prompt string) (string, bool) {
	le := NewLineEditor(prompt)
	return le.ReadLine()
}

// --- 内部工具 ---

func joinLines(lines [][]rune) string {
	strs := make([]string, len(lines))
	for i, line := range lines {
		strs[i] = string(line)
	}
	return strings.Join(strs, "\n")
}

func splitLines(text string) [][]rune {
	parts := strings.Split(text, "\n")
	lines := make([][]rune, len(parts))
	for i, p := range parts {
		lines[i] = []rune(p)
	}
	return lines
}

func copyLines(src [][]rune) [][]rune {
	dst := make([][]rune, len(src))
	for i, line := range src {
		dst[i] = make([]rune, len(line))
		copy(dst[i], line)
	}
	return dst
}

type keySeq []byte

// readKey 从 stdin 读取一个按键序列。
func readKey() keySeq {
	var buf [6]byte
	n, _ := os.Stdin.Read(buf[:1])
	if n == 0 {
		return keySeq{}
	}

	first := buf[0]
	if first != '\033' {
		return keySeq{first}
	}

	n2, _ := os.Stdin.Read(buf[1:])
	if n2 == 0 {
		return keySeq{first}
	}

	seq := keySeq{buf[0]}

	if buf[1] == '[' {
		seq = append(seq, '[')
		for i := 2; i < n2+1; i++ {
			b := buf[i]
			if (b >= '0' && b <= '9') || b == ';' {
				seq = append(seq, b)
			} else {
				seq = append(seq, b)
				return seq
			}
		}
		return seq
	}

	if n2 > 0 {
		seq = append(seq, buf[1:n2+1]...)
	}
	return seq
}
