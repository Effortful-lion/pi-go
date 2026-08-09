package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Effortful-lion/pi-go/ai"
	lg "github.com/Effortful-lion/unibase/logx"
)

var slog = lg.Module("[session]")

// SessionInfo Session 元信息。
type SessionInfo struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	MsgCount  int       `json:"msg_count"`
}

// sessionDir 返回 Session 存储目录。
func sessionDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pi-go/sessions"
	}
	return filepath.Join(home, ".pi-go", "sessions")
}

// SaveSession 将 Agent 当前对话历史保存为 Session 文件（JSONL 格式）。
func (a *Agent) SaveSession(sessionID string) error {
	dir := sessionDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	// 文件名使用 sessionID，清理非法字符
	safeID := sanitizeFilename(sessionID)
	path := filepath.Join(dir, safeID+".jsonl")

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create session file: %w", err)
	}
	defer f.Close()

	messages := a.Messages()
	for _, msg := range messages {
		line, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal message: %w", err)
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("write message: %w", err)
		}
	}

	slog.Info("session saved", lg.Fields{"id": sessionID, "messages": len(messages)})
	return nil
}

// LoadSession 从 JSONL 文件加载对话历史。
// 返回消息列表，可注入到新 Agent 的 Config 中。
func LoadSession(sessionID string) ([]ai.Message, error) {
	dir := sessionDir()
	safeID := sanitizeFilename(sessionID)
	path := filepath.Join(dir, safeID+".jsonl")

	f, err := os.Open(path)
	if err != nil {
		// 也尝试完整路径
		if filepath.IsAbs(sessionID) {
			f2, err2 := os.Open(sessionID)
			if err2 != nil {
				return nil, fmt.Errorf("open session: %w", err)
			}
			f = f2
		} else {
			return nil, fmt.Errorf("open session %q: %w", sessionID, err)
		}
	}
	defer f.Close()

	var messages []ai.Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg ai.Message
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Warn("skip invalid session line", lg.Fields{"error": err.Error()})
			continue
		}
		messages = append(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return messages, fmt.Errorf("scan session: %w", err)
	}

	slog.Info("session loaded", lg.Fields{"id": sessionID, "messages": len(messages)})
	return messages, nil
}

// ListSessions 列出所有已保存的 Session。
func ListSessions() ([]SessionInfo, error) {
	dir := sessionDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var sessions []SessionInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".jsonl" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		sessionID := e.Name()[:len(e.Name())-6] // 去掉 .jsonl 后缀

		// 计算消息数量
		msgCount := countLines(path)

		sessions = append(sessions, SessionInfo{
			ID:        sessionID,
			Path:      path,
			CreatedAt: fi.ModTime(), // JSONL 追加模式，用 mtime 近似
			UpdatedAt: fi.ModTime(),
			MsgCount:  msgCount,
		})
	}

	// 按创建时间倒序
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// DeleteSession 删除指定的 Session。
func DeleteSession(sessionID string) error {
	dir := sessionDir()
	safeID := sanitizeFilename(sessionID)
	path := filepath.Join(dir, safeID+".jsonl")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// countLines 粗略计算文件行数。
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		n++
	}
	return n
}

// sanitizeFilename 清理文件名中的非法字符。
func sanitizeFilename(name string) string {
	repl := func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}
	var out []rune
	for _, r := range name {
		out = append(out, repl(r))
	}
	if len(out) == 0 {
		return "session"
	}
	return string(out)
}
