package lg

import (
	"fmt"
	"time"
)

// Entry 是一条日志记录，包含完整的上下文信息。
type Entry struct {
	Time    time.Time // 日志时间
	Level   Level     // 日志级别
	Module  string    // 模块/子系统名称，如 "user", "shop", "order"
	File    string    // 调用位置，如 "service.go:42"
	Message string    // 日志内容
	Fields  Fields    // 结构化字段
}

// Fields 是结构化日志字段，用于携带业务上下文。
type Fields map[string]any

// Format 返回 Entry 的默认字符串表示。
func (e *Entry) Format() string {
	loc := ""
	if e.File != "" {
		loc = " " + e.File
	}
	mod := ""
	if e.Module != "" {
		mod = "[" + e.Module + "] "
	}
	msg := fmt.Sprintf("[%s] %s%s%s", e.Level.String(), mod, e.Message, loc)
	if len(e.Fields) > 0 {
		msg += " " + e.Fields.format()
	}
	return msg
}

// format 格式化 Fields 为键值对字符串。
func (f Fields) format() string {
	if len(f) == 0 {
		return ""
	}
	s := ""
	for k, v := range f {
		s += fmt.Sprintf(" %s=%v", k, v)
	}
	return s[1:] // 去掉前导空格
}
