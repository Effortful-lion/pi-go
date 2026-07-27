package tui

import (
	"strings"
)

// Highlight 对代码进行语法高亮，返回带 ANSI 着色码的文本。
// lang 支持: go, python, js, shell, json, yaml
func Highlight(code string, lang string) string {
	switch lang {
	case "go":
		return highlightGo(code)
	case "python", "py":
		return highlightPython(code)
	case "javascript", "js", "ts", "typescript":
		return highlightJS(code)
	case "shell", "sh", "bash", "zsh":
		return highlightShell(code)
	case "json":
		return highlightJSON(code)
	case "yaml", "yml":
		return highlightYAML(code)
	default:
		// 未知语言：尝试通用高亮
		return highlightGeneric(code)
	}
}

// highlightGo Go 代码高亮。
func highlightGo(code string) string {
	keywords := setOf(
		"break", "case", "chan", "const", "continue", "default", "defer",
		"else", "fallthrough", "for", "func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return", "select", "struct",
		"switch", "type", "var",
	)
	builtins := setOf(
		"nil", "true", "false", "iota", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64",
		"complex64", "complex128", "byte", "rune", "string", "bool", "error",
		"make", "len", "cap", "new", "append", "copy", "close", "delete",
		"panic", "recover", "print", "println",
	)
	return tokenBasedHL(code, keywords, builtins)
}

// highlightPython Python 代码高亮。
func highlightPython(code string) string {
	keywords := setOf(
		"and", "as", "assert", "async", "await", "break", "class", "continue",
		"def", "del", "elif", "else", "except", "finally", "for", "from",
		"global", "if", "import", "in", "is", "lambda", "nonlocal", "not",
		"or", "pass", "raise", "return", "try", "while", "with", "yield",
	)
	builtins := setOf(
		"None", "True", "False", "self", "cls", "int", "float", "str", "bool",
		"list", "dict", "tuple", "set", "type", "range", "len", "print",
		"isinstance", "hasattr", "getattr", "super",
	)
	return tokenBasedHL(code, keywords, builtins)
}

// highlightJS JavaScript/TypeScript 代码高亮。
func highlightJS(code string) string {
	keywords := setOf(
		"async", "await", "break", "case", "catch", "class", "const", "continue",
		"debugger", "default", "delete", "do", "else", "export", "extends",
		"finally", "for", "function", "if", "import", "in", "instanceof",
		"let", "new", "of", "return", "super", "switch", "this", "throw",
		"try", "typeof", "var", "void", "while", "with", "yield",
		"interface", "type", "enum", "implements", "namespace", "module",
	)
	builtins := setOf(
		"null", "undefined", "true", "false", "NaN", "Infinity",
		"console", "document", "window", "Array", "Object", "String",
		"Number", "Boolean", "Map", "Set", "Promise", "JSON", "Error",
	)
	return tokenBasedHL(code, keywords, builtins)
}

// highlightShell Shell 代码高亮。
func highlightShell(code string) string {
	keywords := setOf(
		"if", "then", "else", "elif", "fi", "case", "esac", "for", "while",
		"until", "do", "done", "in", "function", "return", "exit", "export",
		"local", "source", "alias", "unset", "readonly",
	)
	builtins := setOf("echo", "cd", "pwd", "ls", "cat", "grep", "sed", "awk",
		"mkdir", "rm", "cp", "mv", "chmod", "chown", "find", "xargs",
		"curl", "wget", "git", "docker", "ssh",
	)
	result := tokenBasedHL(code, keywords, builtins)

	// 额外：注释高亮（行首 #）
	result = highlightShellComments(result)
	return result
}

// highlightJSON JSON 高亮（键名/字符串/数字/布尔/null）。
func highlightJSON(code string) string {
	var b strings.Builder
	b.Grow(len(code) * 2)
	i := 0
	for i < len(code) {
		ch := code[i]
		switch {
		case ch == '"':
			b.WriteByte('"')
			i++
			j := i
			for j < len(code) {
				if code[j] == '\\' {
					j += 2
					continue
				}
				if code[j] == '"' {
					break
				}
				j++
			}
			content := code[i:j]
			// 判断是否是键名（后面跟冒号）
			rest := strings.TrimLeft(code[j+1:], " \t\r\n")
			if strings.HasPrefix(rest, ":") {
				b.WriteString(content)
			} else {
				b.WriteString(Green(`"` + content + `"`))
			}
			b.WriteByte('"')
			i = j + 1
		case ch >= '0' && ch <= '9' || ch == '-':
			j := i
			for j < len(code) && (code[j] >= '0' && code[j] <= '9' || code[j] == '.' || code[j] == 'e' || code[j] == 'E' || code[j] == '-' || code[j] == '+') {
				j++
			}
			b.WriteString(Cyan(code[i:j]))
			i = j
		case strings.HasPrefix(code[i:], "true"), strings.HasPrefix(code[i:], "false"), strings.HasPrefix(code[i:], "null"):
			b.WriteString(Magenta(code[i:i+4]))
			i += 4
			if code[i-1] == 'e' {
				i++
			}
		default:
			b.WriteByte(ch)
			i++
		}
	}
	return b.String()
}

// highlightYAML YAML 高亮。
func highlightYAML(code string) string {
	lines := strings.Split(code, "\n")
	var b strings.Builder
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		indent := line[:len(line)-len(trimmed)]

		if strings.HasPrefix(trimmed, "#") {
			b.WriteString(indent)
			b.WriteString(Dim(trimmed))
		} else if colonIndex := strings.Index(trimmed, ":"); colonIndex >= 0 {
			key := trimmed[:colonIndex]
			hasComment := strings.Contains(trimmed[colonIndex+1:], "#")
			if hasComment {
				commentIdx := strings.Index(trimmed[colonIndex+1:], "#") + colonIndex + 1
				val := trimmed[colonIndex+1 : commentIdx]
				comment := trimmed[commentIdx:]
				b.WriteString(indent)
				b.WriteString(Yellow(key))
				b.WriteByte(':')
				b.WriteString(Cyan(strings.TrimSpace(val)))
				if strings.TrimSpace(val) != "" {
					b.WriteByte(' ')
				}
				b.WriteString(Dim(comment))
			} else {
				val := trimmed[colonIndex+1:]
				b.WriteString(indent)
				b.WriteString(Yellow(key))
				b.WriteByte(':')
				b.WriteString(Cyan(val))
			}
		} else {
			b.WriteString(line)
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// highlightGeneric 通用高亮（字符串和注释）。
func highlightGeneric(code string) string {
	return tokenBasedHL(code, nil, nil)
}

// tokenBasedHL 基于空格分隔的 token 进行高亮着色。
func tokenBasedHL(code string, keywords, builtins map[string]bool) string {
	// 状态：0=普通，1=单行注释，2=字符串(")，3=字符串(')，4=字符串(`)
	lines := strings.Split(code, "\n")
	var b strings.Builder
	for li, line := range lines {
		if li > 0 {
			b.WriteByte('\n')
		}
		if line == "" {
			continue
		}

		// 检查是否整行注释
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
			b.WriteString(Dim(line))
			continue
		}

		// 简单 token 化: 按空格分割，每段独立着色
		parts := splitPreserve(line)
		for _, part := range parts {
			switch {
			case isString(part):
				b.WriteString(Green(part))
			case keywords != nil && keywords[part]:
				b.WriteString(Magenta(part))
			case builtins != nil && builtins[part]:
				b.WriteString(Cyan(part))
			default:
				// 内嵌注释
				if commentIdx := findLineComment(part); commentIdx >= 0 {
					b.WriteString(part[:commentIdx])
					b.WriteString(Dim(part[commentIdx:]))
				} else {
					b.WriteString(part)
				}
			}
		}
	}
	return b.String()
}

// setOf 将字符串列表转换为集合。
func setOf(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

// splitPreserve 分割 token，保留引号内内容不分割。
func splitPreserve(line string) []string {
	var tokens []string
	inString := false
	quoteChar := byte(0)
	start := 0

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if inString {
			if ch == '\\' {
				i++ // 跳过转义
				continue
			}
			if ch == quoteChar {
				inString = false
				quoteChar = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			inString = true
			quoteChar = ch
			continue
		}
		if ch == ' ' {
			if i > start {
				tokens = append(tokens, line[start:i])
			}
			tokens = append(tokens, " ")
			start = i + 1
		}
	}
	if start < len(line) {
		tokens = append(tokens, line[start:])
	}
	return tokens
}

// isString 判断 token 是否是字符串字面量。
func isString(s string) bool {
	if len(s) < 2 {
		return false
	}
	return (s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '\'' && s[len(s)-1] == '\'') ||
		(s[0] == '`' && s[len(s)-1] == '`')
}

// findLineComment 查找内嵌注释位置。
func findLineComment(s string) int {
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			return i
		}
	}
	return -1
}

// highlightShellComments 额外处理 shell 注释。
func highlightShellComments(code string) string {
	lines := strings.Split(code, "\n")
	var b strings.Builder
	for i, line := range lines {
		hashIdx := strings.Index(line, "#")
		if hashIdx >= 0 {
			// 确保 # 不是引号内
			before := line[:hashIdx]
			if strings.Count(before, "\"")%2 == 0 && strings.Count(before, "'")%2 == 0 {
				b.WriteString(line[:hashIdx])
				b.WriteString(Dim(line[hashIdx:]))
			} else {
				b.WriteString(line)
			}
		} else {
			b.WriteString(line)
		}
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
