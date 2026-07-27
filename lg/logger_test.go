package lg

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"debug", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"WARN", LevelWarn, false},
		{"warning", LevelWarn, false},
		{"error", LevelError, false},
		{"FATAL", LevelFatal, false},
		{"unknown", LevelInfo, true},
	}

	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if err == nil && got != tt.expected {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestEntryFormat(t *testing.T) {
	entry := &Entry{
		Level:   LevelInfo,
		Module:  "user",
		File:    "service.go:42",
		Message: "用户登录成功",
		Fields:  Fields{"uid": 123, "ip": "10.0.0.1"},
	}
	formatted := entry.Format()
	if !strings.Contains(formatted, "INFO") {
		t.Error("missing level")
	}
	if !strings.Contains(formatted, "[user]") {
		t.Error("missing module")
	}
	if !strings.Contains(formatted, "service.go:42") {
		t.Error("missing file location")
	}
	if !strings.Contains(formatted, "用户登录成功") {
		t.Error("missing message")
	}
	if !strings.Contains(formatted, "uid=123") {
		t.Error("missing uid field")
	}
}

func TestConsoleWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)

	err := w.Write(&Entry{Level: LevelInfo, Message: "hello"})
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hello") {
		t.Errorf("expected 'hello' in output, got: %s", output)
	}
}

func TestConsoleWriterLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelWarn)

	_ = w.Write(&Entry{Level: LevelDebug, Message: "debug"})
	if buf.Len() > 0 {
		t.Error("debug message should be filtered")
	}

	_ = w.Write(&Entry{Level: LevelError, Message: "error"})
	if !strings.Contains(buf.String(), "error") {
		t.Error("error message should be written")
	}
}

func TestFileWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	w, err := NewFileWriter(path, LevelInfo)
	if err != nil {
		t.Fatalf("NewFileWriter failed: %v", err)
	}
	defer w.Close()

	_ = w.Write(&Entry{Level: LevelInfo, Message: "file log test"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if !strings.Contains(string(data), "file log test") {
		t.Errorf("expected log in file, got: %s", string(data))
	}
}

func TestFileWriterAutoCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "logs")
	path := filepath.Join(dir, "app.log")

	w, err := NewFileWriter(path, LevelInfo)
	if err != nil {
		t.Fatalf("NewFileWriter should auto-create dir: %v", err)
	}
	defer w.Close()
	defer os.RemoveAll(dir)

	_ = w.Write(&Entry{Level: LevelInfo, Message: "auto dir test"})
}

func TestMultiWriter(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	w1 := NewConsoleWriter(&buf1, LevelInfo)
	w2 := NewConsoleWriter(&buf2, LevelDebug)
	mw := NewMultiWriter(w1, w2)

	_ = mw.Write(&Entry{Level: LevelInfo, Message: "shared"})

	if !strings.Contains(buf1.String(), "shared") {
		t.Error("buf1 should contain log")
	}
	if !strings.Contains(buf2.String(), "shared") {
		t.Error("buf2 should contain log")
	}
}

func TestRouter(t *testing.T) {
	var defaultBuf, userBuf, shopBuf bytes.Buffer

	router := NewRouter(NewConsoleWriter(&defaultBuf, LevelInfo))
	router.Route("user", NewConsoleWriter(&userBuf, LevelDebug))
	router.Route("shop", NewConsoleWriter(&shopBuf, LevelInfo))

	logger := New(router)

	logger.Module("user").Info("用户登录")
	if !strings.Contains(userBuf.String(), "用户登录") {
		t.Error("user log should go to userBuf")
	}
	if defaultBuf.Len() > 0 {
		t.Error("user log should NOT go to defaultBuf")
	}

	logger.Module("shop").Warn("库存不足")
	if !strings.Contains(shopBuf.String(), "库存不足") {
		t.Error("shop log should go to shopBuf")
	}

	logger.Module("unknown").Error("未知错误")
	if !strings.Contains(defaultBuf.String(), "未知错误") {
		t.Error("unknown module should go to defaultBuf")
	}
}

func TestRouterUnroute(t *testing.T) {
	var defaultBuf, userBuf bytes.Buffer
	router := NewRouter(NewConsoleWriter(&defaultBuf, LevelInfo))
	router.Route("user", NewConsoleWriter(&userBuf, LevelInfo))

	logger := New(router)

	logger.Module("user").Info("before unroute")
	if userBuf.Len() == 0 {
		t.Error("should go to userBuf before unroute")
	}

	router.Unroute("user")

	logger.Module("user").Info("after unroute")
	if !strings.Contains(defaultBuf.String(), "after unroute") {
		t.Error("after unroute, should go to defaultBuf")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)
	logger := New(w).With(Fields{"app": "myapp", "env": "test"})

	logger.Info("服务启动", Fields{"port": 8080})

	output := buf.String()
	if !strings.Contains(output, "app=myapp") {
		t.Error("missing fixed field 'app'")
	}
	if !strings.Contains(output, "env=test") {
		t.Error("missing fixed field 'env'")
	}
	if !strings.Contains(output, "port=8080") {
		t.Error("missing per-log field 'port'")
	}
}

func TestLoggerLevelFilter(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelWarn)
	logger := New(w)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")

	output := buf.String()
	if strings.Contains(output, "debug") || strings.Contains(output, "info") {
		t.Error("debug/info should be filtered")
	}
	if !strings.Contains(output, "warn") {
		t.Error("warn should be output")
	}
}

func TestLoggerModuleWithInherit(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)
	logger := New(w).With(Fields{"service": "api"})

	userLog := logger.Module("user")
	userLog.Info("登录", Fields{"uid": 1})

	output := buf.String()
	if !strings.Contains(output, "[user]") {
		t.Error("missing module prefix")
	}
	if !strings.Contains(output, "service=api") {
		t.Error("missing inherited field")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	SetDefault(New(NewConsoleWriter(&buf, LevelDebug)))

	Info("包级别日志", Fields{"key": "val"})
	output := buf.String()
	if !strings.Contains(output, "包级别日志") {
		t.Error("missing message")
	}
	if !strings.Contains(output, "key=val") {
		t.Error("missing field")
	}
}

func TestFormatFunctions(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelInfo)
	logger := New(w)

	logger.Infof("用户 %d 登录, 来自 %s", 123, "10.0.0.1")
	output := buf.String()
	if !strings.Contains(output, "用户 123 登录, 来自 10.0.0.1") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestCallerLocation(t *testing.T) {
	var buf bytes.Buffer
	w := NewConsoleWriter(&buf, LevelDebug)
	logger := New(w)

	logger.Debug("caller test")
	output := buf.String()
	if !strings.Contains(output, "logger_test.go") {
		t.Errorf("missing caller file, got: %s", output)
	}
}
