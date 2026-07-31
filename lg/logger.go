package lg

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Logger 是面向用户使用的日志记录器。
//
// 用法示例：
//
//	// 直接使用
//	lg.Info("服务启动", lg.Fields{"port": 8080})
//	lg.Error("连接失败", lg.Fields{"db": "mysql"})
//
//	// 模块化使用
//	lg.Module("user").Info("用户登录", lg.Fields{"uid": 123})
//	lg.Module("shop").Warn("库存不足", lg.Fields{"sku": "A001"})
//
//	// 使用具体注册的日志器
//	userLog := lg.New(userWriter).Module("user")
//	userLog.Info("用户登录成功")
type Logger struct {
	module     string // 所属模块
	writer     Writer // 输出目标（通常是 Router）
	fields     Fields // 预置的固定字段
	callerSkip int    // 调用栈跳过层数
}

// New 创建一个日志记录器。
func New(writer Writer) *Logger {
	if writer == nil {
		writer = NewConsoleWriter(os.Stdout, LevelInfo)
	}
	return &Logger{
		writer:     writer,
		callerSkip: 2, // New → Debug/Info... → caller
	}
}

// Module 创建一个绑定到指定模块的子 Logger。
// 该 Logger 的所有日志都会带上模块名，Router 会根据模块名分流。
func (l *Logger) Module(module string) *Logger {
	return &Logger{
		module:     module,
		writer:     l.writer,
		fields:     l.fields, // 继承父 Logger 的固定字段
		callerSkip: l.callerSkip,
	}
}

// With 创建一个带固定字段的子 Logger。
// 固定字段会自动附加到每条日志中。
func (l *Logger) With(fields Fields) *Logger {
	merged := Fields{}
	for k, v := range l.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &Logger{
		module:     l.module,
		writer:     l.writer,
		fields:     merged,
		callerSkip: l.callerSkip,
	}
}

// Debug 输出调试日志。
func (l *Logger) Debug(msg string, fields ...Fields) {
	l.log(LevelDebug, msg, fields...)
}

// Info 输出信息日志。
func (l *Logger) Info(msg string, fields ...Fields) {
	l.log(LevelInfo, msg, fields...)
}

// Warn 输出警告日志。
func (l *Logger) Warn(msg string, fields ...Fields) {
	l.log(LevelWarn, msg, fields...)
}

// Error 输出错误日志。
func (l *Logger) Error(msg string, fields ...Fields) {
	l.log(LevelError, msg, fields...)
}

// Fatal 输出致命日志并退出程序。
func (l *Logger) Fatal(msg string, fields ...Fields) {
	l.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

// Debugf 格式化调试日志。
func (l *Logger) Debugf(format string, args ...any) {
	l.log(LevelDebug, fmt.Sprintf(format, args...))
}

// Infof 格式化信息日志。
func (l *Logger) Infof(format string, args ...any) {
	l.log(LevelInfo, fmt.Sprintf(format, args...))
}

// Warnf 格式化警告日志。
func (l *Logger) Warnf(format string, args ...any) {
	l.log(LevelWarn, fmt.Sprintf(format, args...))
}

// Errorf 格式化错误日志。
func (l *Logger) Errorf(format string, args ...any) {
	l.log(LevelError, fmt.Sprintf(format, args...))
}

// Fatalf 格式化致命日志并退出程序。
func (l *Logger) Fatalf(format string, args ...any) {
	l.log(LevelFatal, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// log 是内部统一的日志写入方法。
func (l *Logger) log(level Level, msg string, fields ...Fields) {
	if level < l.writer.Level() {
		return
	}

	// 合并字段
	merged := Fields{}
	for k, v := range l.fields {
		merged[k] = v
	}
	for _, f := range fields {
		for k, v := range f {
			merged[k] = v
		}
	}

	entry := &Entry{
		Time:    time.Now(),
		Level:   level,
		Module:  l.module,
		File:    caller(l.callerSkip),
		Message: msg,
		Fields:  merged,
	}

	_ = l.writer.Write(entry)
}

// caller 获取调用者的文件名和行号。
func caller(skip int) string {
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return ""
	}
	// 只保留文件名，不保留完整路径
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' || file[i] == '\\' {
			file = file[i+1:]
			break
		}
	}
	return file + ":" + strconv.Itoa(line)
}

// ============================================================================
// 包级别便捷函数 — 使用默认 Logger
// ============================================================================

var defaultLogger = New(NewConsoleWriter(os.Stdout, LevelInfo))

// SetDefault 设置默认 Logger。
func SetDefault(l *Logger) {
	defaultLogger = l
}

// Default 返回默认 Logger。
func Default() *Logger {
	return defaultLogger
}

// Module 使用默认 Logger 创建模块子 Logger。
func Module(module string) *Logger {
	return defaultLogger.Module(module)
}

// ============================================================================
// LogOption — 日志配置选项
// ============================================================================

// LogOption 日志配置选项函数，传入 SetPath 定制日志行为。
type LogOption func(*logConfig)

type logConfig struct {
	levelDirs      []levelDir
	rotateInterval time.Duration
	rotateSize     int64
	retention      time.Duration
}

type levelDir struct {
	level Level
	dir   string
}

// WithLevelDir 将指定级别的日志输出到独立子目录。
func WithLevelDir(level Level, dir string) LogOption {
	return func(c *logConfig) {
		c.levelDirs = append(c.levelDirs, levelDir{level: level, dir: dir})
	}
}

// WithRotateByInterval 按时间间隔切割日志文件。
// 文件名自动追加日期+时钟后缀，精度由间隔决定：
//   - < 1小时 → pg_2026-08-01_12-00-00.log
//   - >= 1小时 → pg_2026-08-01_12-00.log
//   - >= 1天 → pg_2026-08-01.log
func WithRotateByInterval(d time.Duration) LogOption {
	return func(c *logConfig) {
		c.rotateInterval = d
	}
}

// WithRotateBySize 按文件大小切割日志文件。
// 超过 size 字节时自动切换新文件，文件名追加 .001 等序号。
func WithRotateBySize(size int64) LogOption {
	return func(c *logConfig) {
		c.rotateSize = size
	}
}

// WithRetention 定时清理过期日志文件。
// maxAge: 日志保留时长，超过此时间的 .log 文件将被删除。
// 检查间隔 = min(maxAge, 1小时)。
func WithRetention(maxAge time.Duration) LogOption {
	return func(c *logConfig) {
		c.retention = maxAge
	}
}

// SetPath 将默认 Logger 和 Frame Logger 的输出重定向到指定目录下的日志文件。
// dir: 日志目录，如 "logs"
// level: 最低输出级别
// pattern: 文件名模式构建器，Build() 自动追加 .log 后缀
// opts: 可选配置，如 WithLevelDir 分级目录等
//
// 使用示例:
//
//	// 不分级: logs/pg_2026-08-01.log
//	lg.SetPath("logs", lg.LevelInfo,
//	    lg.NewLogNamePattern().Module().Char("_").Date("2006-01-02"))
//
//	// 按级别分目录:
//	//   logs/info/pg_2026-08-01.log
//	//   logs/error/pg_2026-08-01.log
//	lg.SetPath("logs", lg.LevelInfo,
//	    lg.NewLogNamePattern().Module().Char("_").Date("2006-01-02"),
//	    lg.WithLevelDir(lg.LevelInfo, "info"),
//	    lg.WithLevelDir(lg.LevelError, "error"),
//	)
func SetPath(dir string, level Level, pattern *LogNamePattern, opts ...LogOption) error {
	cfg := &logConfig{}
	for _, o := range opts {
		o(cfg)
	}

	// 定时切割：在 pattern 末尾追加日期+时钟
	if cfg.rotateInterval > 0 {
		dateLayout, clockLayout := intervalLayout(cfg.rotateInterval)
		pattern.Char("_").Date(dateLayout)
		if clockLayout != "" {
			pattern.Char("_").Clock(clockLayout)
		}
	}

	logName := pattern.Build()

	// 没有分级配置：单个 FileWriter
	if len(cfg.levelDirs) == 0 {
		fw, err := NewFileWriterWithLogName(dir, level, logName)
		if err != nil {
			return err
		}
		fw.rotateSize = cfg.rotateSize
		SetDefault(New(fw))
		SetFrameWriter(fw)
		startRetention(dir, cfg.retention)
		return nil
	}

	// 有分级配置：默认 dir + 各 LevelDir 子目录，MultiWriter 组合
	var writers []Writer

	defaultFW, err := NewFileWriterWithLogName(dir, level, logName)
	if err != nil {
		return err
	}
	defaultFW.rotateSize = cfg.rotateSize
	writers = append(writers, defaultFW)

	for _, ld := range cfg.levelDirs {
		subDir := dir + "/" + ld.dir
		fw, err := NewFileWriterWithLogName(subDir, ld.level, logName)
		if err != nil {
			return err
		}
		fw.rotateSize = cfg.rotateSize
		writers = append(writers, fw)
	}

	mw := NewMultiWriter(writers...)
	SetDefault(New(mw))
	SetFrameWriter(mw)
	startRetention(dir, cfg.retention)
	return nil
}

// startRetention 启动后台清理 goroutine，定期删除过期日志文件。
func startRetention(dir string, maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	interval := maxAge
	if interval > time.Hour {
		interval = time.Hour
	}
	go func() {
		for {
			time.Sleep(interval)
			cleanDir(dir, maxAge)
		}
	}()
}

// cleanDir 递归清理目录中超过 maxAge 的 .log 文件。
func cleanDir(dir string, maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".log") {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
		return nil
	})
}

// intervalLayout 根据时间间隔返回日期和时钟的 format layout。
func intervalLayout(d time.Duration) (dateLayout, clockLayout string) {
	switch {
	case d >= 24*time.Hour:
		return "2006-01-02", ""
	case d >= time.Hour:
		return "2006-01-02", "15-04"
	default:
		return "2006-01-02", "15-04-05"
	}
}

// ============================================================================
// 框架内置日志器
// ============================================================================

// Frame 是框架内部日志器，用于记录 llmLib 库自身的运行时信息。
// 所有库内部错误、警告都通过 Frame 输出，模块名为 "frame"。
var Frame = New(NewConsoleWriter(os.Stderr, LevelWarn)).Module("frame")

// SetFrameWriter 替换 Frame 日志器的输出目标。
func SetFrameWriter(w Writer) {
	Frame = New(w).Module("frame")
}

// ============================================================================
// 包级别便捷函数
// ============================================================================

// Debug 包级别 Debug。
func Debug(msg string, fields ...Fields) { defaultLogger.Debug(msg, fields...) }

// Info 包级别 Info。
func Info(msg string, fields ...Fields) { defaultLogger.Info(msg, fields...) }

// Warn 包级别 Warn。
func Warn(msg string, fields ...Fields) { defaultLogger.Warn(msg, fields...) }

// Error 包级别 Error。
func Error(msg string, fields ...Fields) { defaultLogger.Error(msg, fields...) }

// Fatal 包级别 Fatal。
func Fatal(msg string, fields ...Fields) { defaultLogger.Fatal(msg, fields...) }

// Debugf 包级别 Debugf。
func Debugf(format string, args ...any) { defaultLogger.Debugf(format, args...) }

// Infof 包级别 Infof。
func Infof(format string, args ...any) { defaultLogger.Infof(format, args...) }

// Warnf 包级别 Warnf。
func Warnf(format string, args ...any) { defaultLogger.Warnf(format, args...) }

// Errorf 包级别 Errorf。
func Errorf(format string, args ...any) { defaultLogger.Errorf(format, args...) }

// Fatalf 包级别 Fatalf。
func Fatalf(format string, args ...any) { defaultLogger.Fatalf(format, args...) }
