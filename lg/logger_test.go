package lg

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSetPath_NoLevelDir(t *testing.T) {
	dir := t.TempDir()
	pattern := NewLogNamePattern().Date("2006-01-02")

	err := SetPath(dir, LevelInfo, pattern)
	if err != nil {
		t.Fatal(err)
	}

	Info("hello no dirs")

	// 检查日志文件在 dir 根目录下
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no log files created")
	}
}

func TestSetPath_WithLevelDir(t *testing.T) {
	dir := t.TempDir()
	pattern := NewLogNamePattern().Date("2006-01-02")

	err := SetPath(dir, LevelInfo, pattern,
		WithLevelDir(LevelError, "error"),
		WithLevelDir(LevelInfo, "info"),
	)
	if err != nil {
		t.Fatal(err)
	}

	// 写一条 error 日志
	Error("something wrong")
	// 写一条 info 日志
	Info("all good")

	// error 日志应该在 error 子目录
	errorEntries, _ := os.ReadDir(dir + "/error")
	if len(errorEntries) == 0 {
		t.Error("expected log file in error/ directory")
	}

	// info 日志应该在 info 子目录
	infoEntries, _ := os.ReadDir(dir + "/info")
	if len(infoEntries) == 0 {
		t.Error("expected log file in info/ directory")
	}
}

func TestSetPath_LevelFilter(t *testing.T) {
	dir := t.TempDir()
	pattern := NewLogNamePattern().Date("2006-01-02")

	// error 子目录只收 LevelError 及以上
	err := SetPath(dir, LevelDebug, pattern,
		WithLevelDir(LevelError, "error"),
	)
	if err != nil {
		t.Fatal(err)
	}

	Debug("debug msg") // 级别 < LevelError，不应进 error 目录
	Error("error msg") // 级别 >= LevelError，进 error 目录

	// error 子目录应该只有 error 日志
	errorEntries, _ := os.ReadDir(dir + "/error")
	if len(errorEntries) == 0 {
		t.Fatal("expected log file in error/")
	}
	data, _ := os.ReadFile(dir + "/error/" + errorEntries[0].Name())
	if strings.Contains(string(data), "debug msg") {
		t.Error("debug msg should NOT be in error directory")
	}
	if !strings.Contains(string(data), "error msg") {
		t.Error("error msg should be in error directory")
	}
}

func TestRotateByInterval_Hourly(t *testing.T) {
	dir := t.TempDir()

	err := SetPath(dir, LevelInfo,
		NewLogNamePattern().Module(),
		WithRotateByInterval(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	Info("test hourly")

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no log file created")
	}
	name := entries[0].Name()
	// 文件名应包含日期和分钟精度: pg_2026-08-01_12-00.log
	if !strings.Contains(name, "_") {
		t.Errorf("expected datetime suffix in filename, got %q", name)
	}
	if !strings.HasSuffix(name, ".log") {
		t.Errorf("expected .log suffix, got %q", name)
	}
}

func TestRotateByInterval_Daily(t *testing.T) {
	dir := t.TempDir()

	err := SetPath(dir, LevelInfo,
		NewLogNamePattern().Module(),
		WithRotateByInterval(24*time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}

	Info("test daily")

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no log file created")
	}
	name := entries[0].Name()
	// 文件名应包含日期: pg_2026-08-01.log
	if !strings.Contains(name, "2026") {
		t.Errorf("expected date in filename, got %q", name)
	}
	if !strings.HasSuffix(name, ".log") {
		t.Errorf("expected .log suffix, got %q", name)
	}
}

func TestRotateByInterval_Minutely(t *testing.T) {
	dir := t.TempDir()

	err := SetPath(dir, LevelInfo,
		NewLogNamePattern().Module(),
		WithRotateByInterval(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}

	Info("test minutely")

	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Fatal("no log file created")
	}
	name := entries[0].Name()
	// 文件名应包含日期和秒精度: pg_2026-08-01_12-00-00.log
	parts := strings.SplitN(strings.TrimSuffix(name, ".log"), "_", 3)
	if len(parts) < 3 {
		t.Errorf("expected module_date_time format, got %q", name)
	}
}

func TestRotateBySize_Switch(t *testing.T) {
	dir := t.TempDir()

	err := SetPath(dir, LevelInfo,
		NewLogNamePattern().Module(),
		WithRotateBySize(50), // 很小，一条日志就超
	)
	if err != nil {
		t.Fatal(err)
	}

	// 写入多条，触发大小切割
	Info("line 1")
	Info("line 2")
	Info("line 3")

	entries, _ := os.ReadDir(dir)
	if len(entries) < 2 {
		t.Fatalf("expected at least 2 files from size rotation, got %d", len(entries))
	}

	// 检查有序号的文件
	hasSeq := false
	for _, e := range entries {
		if strings.Contains(e.Name(), ".001.") || strings.Contains(e.Name(), ".002.") {
			hasSeq = true
		}
	}
	if !hasSeq {
		t.Error("expected sequenced files like .001.log")
	}
}

func TestRotateBySize_SeqFormat(t *testing.T) {
	dir := t.TempDir()

	err := SetPath(dir, LevelInfo,
		NewLogNamePattern().Module(),
		WithRotateBySize(30),
	)
	if err != nil {
		t.Fatal(err)
	}

	Info("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") // 长消息触发切割
	Info("bb")

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if strings.Contains(name, ".") && !strings.HasSuffix(name, ".log") {
			if !strings.Contains(name, ".00") {
				t.Errorf("unexpected seq format: %q", name)
			}
		}
	}
}

func TestRetention_CleanOldFiles(t *testing.T) {
	dir := t.TempDir()

	// 创建一个"旧"日志文件（修改时间设为 2 天前）
	oldPath := filepath.Join(dir, "old.log")
	os.WriteFile(oldPath, []byte("old"), 0644)
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)

	// 创建一个"新"日志文件
	newPath := filepath.Join(dir, "new.log")
	os.WriteFile(newPath, []byte("new"), 0644)

	// 保留 24 小时，应删除 old.log
	cleanDir(dir, 24*time.Hour)

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old.log should be deleted")
	}
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("new.log should be kept")
	}
}

func TestRetention_OnlyLogFiles(t *testing.T) {
	dir := t.TempDir()

	// 创建 .txt 文件（不应被删除）
	txtPath := filepath.Join(dir, "readme.txt")
	os.WriteFile(txtPath, []byte("readme"), 0644)
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(txtPath, oldTime, oldTime)

	cleanDir(dir, 24*time.Hour)

	if _, err := os.Stat(txtPath); os.IsNotExist(err) {
		t.Error("non-log files should not be deleted")
	}
}

func TestRetention_SubDirs(t *testing.T) {
	dir := t.TempDir()

	// 创建子目录下的旧日志
	subDir := filepath.Join(dir, "error")
	os.MkdirAll(subDir, 0755)
	oldPath := filepath.Join(subDir, "old.log")
	os.WriteFile(oldPath, []byte("old"), 0644)
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)

	cleanDir(dir, 24*time.Hour)

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("old log in subdir should be deleted")
	}
}
