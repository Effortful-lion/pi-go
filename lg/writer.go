package lg

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Writer 是日志输出接口。实现此接口即可将日志输出到任意目标。
//
// 典型实现：
//   - os.Stdout / os.Stderr
//   - 文件
//   - 网络（UDP/TCP）
//   - 消息队列
//   - 第三方日志服务
type Writer interface {
	// Write 写入一条日志记录。
	Write(entry *Entry) error

	// Level 返回该 Writer 接受的最低日志级别。
	// 低于此级别的日志不会被传递给 Write。
	Level() Level

	// Close 关闭 Writer，释放资源。
	Close() error
}

// ============================================================================
// ConsoleWriter — 控制台输出
// ============================================================================

// ConsoleWriter 将日志写入 io.Writer（通常为 os.Stdout 或 os.Stderr）。
type ConsoleWriter struct {
	out   io.Writer
	level Level
	mu    sync.Mutex
}

// NewConsoleWriter 创建一个控制台 Writer。
// out: 输出目标，如 os.Stdout, os.Stderr
// level: 最低输出级别
func NewConsoleWriter(out io.Writer, level Level) *ConsoleWriter {
	if out == nil {
		out = os.Stdout
	}
	return &ConsoleWriter{out: out, level: level}
}

func (w *ConsoleWriter) Level() Level { return w.level }

func (w *ConsoleWriter) Write(entry *Entry) error {
	if entry.Level < w.level {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := fmt.Fprintln(w.out, entry.Format())
	return err
}

func (w *ConsoleWriter) Close() error { return nil }

// ============================================================================
// FileWriter — 文件输出
// ============================================================================

// FileWriter 将日志写入文件，支持自动创建目录。
type FileWriter struct {
	file  *os.File
	level Level
	mu    sync.Mutex
}

// NewFileWriter 创建一个文件 Writer。
// path: 日志文件路径，父目录不存在时自动创建
// level: 最低输出级别
func NewFileWriter(path string, level Level) (*FileWriter, error) {
	dir := pathDir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("lg: create dir %s: %w", dir, err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lg: open file %s: %w", path, err)
	}
	return &FileWriter{file: f, level: level}, nil
}

// pathDir 提取文件路径中的目录部分。
func pathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}

func (w *FileWriter) Level() Level { return w.level }

func (w *FileWriter) Write(entry *Entry) error {
	if entry.Level < w.level {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := fmt.Fprintln(w.file, entry.Format())
	return err
}

func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// ============================================================================
// MultiWriter — 多路输出
// ============================================================================

// MultiWriter 将一条日志同时写入多个 Writer。
type MultiWriter struct {
	writers []Writer
}

// NewMultiWriter 创建一个多路 Writer。
func NewMultiWriter(writers ...Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (w *MultiWriter) Level() Level {
	// 取所有子 Writer 中的最低级别（最宽松）
	minLevel := LevelFatal
	for _, wr := range w.writers {
		if wr.Level() < minLevel {
			minLevel = wr.Level()
		}
	}
	return minLevel
}

func (w *MultiWriter) Write(entry *Entry) error {
	for _, wr := range w.writers {
		if entry.Level < wr.Level() {
			continue
		}
		_ = wr.Write(entry) // 忽略单个 writer 的错误
	}
	return nil
}

func (w *MultiWriter) Close() error {
	for _, wr := range w.writers {
		_ = wr.Close()
	}
	return nil
}
