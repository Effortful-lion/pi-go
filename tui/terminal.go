// Package tui 提供终端 UI 能力——原始终端控制、颜色样式、行编辑与交互式对话。
//
// 本文件提供终端底层能力：原始模式、光标控制、ANSI 颜色/样式、单行编辑。
package tui

import (
	"fmt"
	"os"
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
func Bold(text string) string    { return StyleText(text, Style{Bold: true}) }
func Dim(text string) string     { return StyleText(text, Style{Dim: true}) }
func Red(text string) string     { return StyleText(text, Style{Fg: ColorRed}) }
func Green(text string) string   { return StyleText(text, Style{Fg: ColorGreen}) }
func Blue(text string) string    { return StyleText(text, Style{Fg: ColorBlue}) }
func Cyan(text string) string    { return StyleText(text, Style{Fg: ColorCyan}) }
func Gray(text string) string    { return StyleText(text, Style{Fg: ColorGray}) }
func Yellow(text string) string  { return StyleText(text, Style{Fg: ColorYellow}) }

// ============================================================================
// 行编辑
// ============================================================================

const (
	keyEnter     = 13  // \r
	keyCtrlC     = 3   // Ctrl+C
	keyCtrlD     = 4   // Ctrl+D
	keyBackspace = 127 // \x7f
	keyDelete    = '\033' // 转义序列前缀
	keyTab       = 9
)

// EditLine 单行文本编辑器（原始模式）。
//
// 支持按键：← → Home End Backspace Delete Enter Ctrl+C Ctrl+D
// prompt 参数是显示在输入行前的提示符。
// 返回用户输入的文本，cancelled 为 true 表示用户取消了输入。
func EditLine(prompt string) (string, bool) {
	// 临时显示光标
	CursorShow()
	defer CursorHide()

	var buf []rune
	cursor := 0

	// 初始渲染
	fmt.Print(prompt)
	renderLine(buf, cursor)

	for {
		seq := readKey()

		switch {
		case len(seq) == 1:
			switch seq[0] {
			case keyEnter:
				// 确认输入
				fmt.Print("\r\n")
				return string(buf), false

			case keyCtrlC:
				// 取消
				if len(buf) > 0 {
					// 有内容时清空而不是退出——让调用方决定
					buf = buf[:0]
					cursor = 0
					fmt.Print("\r")
				fmt.Print(prompt)
				ClearLineFromCursor()
				} else {
					fmt.Print("\r\n")
					return "", true
				}

			case keyCtrlD:
				// 空行时退出
				if len(buf) == 0 {
					fmt.Print("\r\n")
					return "", true
				}
				// 删除光标后字符
				if cursor < len(buf) {
					buf = append(buf[:cursor], buf[cursor+1:]...)
				}

			case keyBackspace:
				if cursor > 0 {
					buf = append(buf[:cursor-1], buf[cursor:]...)
					cursor--
				}

			case keyTab:
				// Tab 不做处理
				continue

			default:
				// 普通文本插入
				if seq[0] >= 32 && seq[0] < 127 {
					// 在 cursor 位置插入
					buf = append(buf, 0)
					copy(buf[cursor+1:], buf[cursor:])
					buf[cursor] = rune(seq[0])
					cursor++
				}
			}

		case len(seq) == 2 && seq[0] == '[':
			// 双字符转义序列
			switch seq[1] {
			case 'D': // ← 左箭头
				if cursor > 0 {
					cursor--
				}
			case 'C': // → 右箭头
				if cursor < len(buf) {
					cursor++
				}
			case 'H': // Home
				cursor = 0
			case 'F': // End
				cursor = len(buf)
			case '3': // Delete (三字符序列，实际再读一个)
				// \033[3~ → Delete 键
				// 需要在外面再读一个 byte，这里简单处理
			}

		case len(seq) == 3 && seq[0] == '[' && seq[2] == '~':
			switch seq[1] {
			case '3': // Delete
				if cursor < len(buf) {
					buf = append(buf[:cursor], buf[cursor+1:]...)
				}
			case '1': // Home (xterm: \033[1~)
				cursor = 0
			case '4': // End (xterm: \033[4~)
				cursor = len(buf)
			}

		case len(seq) == 4 && seq[0] == '[' && seq[1] == '1' && seq[2] == ';':
			// \033[1;5D → Ctrl+←   \033[1;5C → Ctrl+→
			switch seq[3] {
			case 'D': // Ctrl+←
				// 跳到前一个单词开头
				for cursor > 0 && buf[cursor-1] == ' ' {
					cursor--
				}
				for cursor > 0 && buf[cursor-1] != ' ' {
					cursor--
				}
			case 'C': // Ctrl+→
				// 跳到下一个单词末尾
				for cursor < len(buf) && buf[cursor] != ' ' {
					cursor++
				}
				for cursor < len(buf) && buf[cursor] == ' ' {
					cursor++
				}
			}
		}

		// 重绘制当前行
	fmt.Print("\r")
	fmt.Print(prompt)
	ClearLineFromCursor()
		renderLine(buf, cursor)
	}
}

type keySeq []byte

// readKey 从 stdin 读取一个按键序列。
// 单字符按键如 'a', Enter, Ctrl+C → 返回 [byte]
// ANSI 转义序列如 ← → → 返回完整序列
func readKey() keySeq {
	var buf [6]byte
	n, _ := os.Stdin.Read(buf[:1])
	if n == 0 {
		return keySeq{}
	}

	first := buf[0]

	// 不是转义字符 → 单字符按键
	if first != '\033' {
		return keySeq{first}
	}

	// 是转义序列，继续读取
	n2, _ := os.Stdin.Read(buf[1:])
	if n2 == 0 {
		return keySeq{first} // 孤立的 ESC
	}

	seq := keySeq{buf[0]}

	// \033[ → 检查是否为 CSI 序列
	if buf[1] == '[' {
		seq = append(seq, '[')
		// 读取直到非数字/分号字符
		for i := 2; i < n2+1; i++ {
			b := buf[i]
			if (b >= '0' && b <= '9') || b == ';' {
				seq = append(seq, b)
			} else {
				seq = append(seq, b)
				// 如果终止符是 ~ 且我们读到了 3 字符 CSI 序列
				if b == '~' {
					// \033[3~ 风格的序列已完成
				} else if b >= 0x40 && b <= 0x7E {
					// 标准 CSI 终止符
				}
				return seq
			}
		}
		return seq
	}

	// 其他转义序列，按原样返回
	if n2 > 0 {
		seq = append(seq, buf[1:n2+1]...)
	}
	return seq
}

// renderLine 重绘制当前输入行。
func renderLine(buf []rune, cursor int) {
	for i, r := range buf {
		if i == cursor {
			// 光标位置：用反向视频显示
			fmt.Printf("%s[7m%c%s[0m", esc, r, esc)
		} else {
			fmt.Printf("%c", r)
		}
	}
	if cursor == len(buf) {
		// 光标在末尾，蓝色方块提示
		fmt.Print(Blue("█"))
	}
}
