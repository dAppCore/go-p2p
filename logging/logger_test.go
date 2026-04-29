package logging

import (
	"testing"

	core "dappco.re/go"
)

type loggerTestBuffer interface {
	core.Writer
	Len() int
	Reset()
	String() string
}

func axLoggerBuffer(level Level) (*Logger, loggerTestBuffer) {
	buf := core.NewBuffer()
	logger := New(Config{Output: buf, Level: level, Component: "test"})
	return logger, buf
}

func TestLoggerLevels(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output: buf,
		Level:  LevelInfo,
	})

	// Debug should not appear at Info level
	logger.Debug("debug message")
	if buf.Len() > 0 {
		t.Error("Debug message should not appear at Info level")
	}

	// Info should appear
	logger.Info("info message")
	if !core.Contains(buf.String(), "[INFO]") {
		t.Error("Info message should appear")
	}
	if !core.Contains(buf.String(), "info message") {
		t.Error("Info message content should appear")
	}
	buf.Reset()

	// Warn should appear
	logger.Warn("warn message")
	if !core.Contains(buf.String(), "[WARN]") {
		t.Error("Warn message should appear")
	}
	buf.Reset()

	// Error should appear
	logger.Error("error message")
	if !core.Contains(buf.String(), "[ERROR]") {
		t.Error("Error message should appear")
	}
}

func TestLogger_Level_String_Good(t *testing.T) {
	got := LevelInfo.String()
	if got != "INFO" {
		t.Fatalf("level: got %q", got)
	}
	if LevelWarn.String() != "WARN" {
		t.Fatal("warn level string mismatch")
	}
}

func TestLogger_Level_String_Bad(t *testing.T) {
	got := Level(99).String()
	if got != "UNKNOWN" {
		t.Fatalf("level: got %q", got)
	}
	if Level(-1).String() != "UNKNOWN" {
		t.Fatal("negative level should be unknown")
	}
}

func TestLogger_Level_String_Ugly(t *testing.T) {
	got := LevelDebug.String()
	if got != "DEBUG" {
		t.Fatalf("level: got %q", got)
	}
	if LevelError.String() != "ERROR" {
		t.Fatal("error level string mismatch")
	}
}

func TestLogger_DefaultConfig_Good(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Output == nil {
		t.Fatal("expected default output")
	}
	if cfg.Level != LevelInfo {
		t.Fatalf("level: got %v", cfg.Level)
	}
}

func TestLogger_DefaultConfig_Bad(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Component != "" {
		t.Fatalf("component: got %q", cfg.Component)
	}
	if cfg.Level == LevelDebug {
		t.Fatal("default should not be debug")
	}
}

func TestLogger_DefaultConfig_Ugly(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = nil
	logger := New(cfg)
	if logger.output == nil {
		t.Fatal("New should repair nil output")
	}
}

func TestLogger_New_Good(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{Output: buf, Level: LevelDebug, Component: "core"})
	if logger == nil {
		t.Fatal("expected logger")
	}
	if logger.component != "core" {
		t.Fatalf("component: got %q", logger.component)
	}
}

func TestLogger_New_Bad(t *testing.T) {
	logger := New(Config{})
	if logger.output == nil {
		t.Fatal("expected fallback output")
	}
	if logger.level != LevelDebug {
		t.Fatalf("zero config level: got %v", logger.level)
	}
}

func TestLogger_New_Ugly(t *testing.T) {
	logger := New(Config{Level: Level(99), Component: ""})
	if logger.GetLevel() != Level(99) {
		t.Fatalf("level: got %v", logger.GetLevel())
	}
	if logger.component != "" {
		t.Fatal("expected empty component")
	}
}

func TestLogger_Logger_WithComponent_Good(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelDebug)
	child := logger.WithComponent("child")
	if child.component != "child" {
		t.Fatalf("component: got %q", child.component)
	}
	if child.level != logger.level {
		t.Fatal("child should inherit level")
	}
}

func TestLogger_Logger_WithComponent_Bad(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelInfo)
	child := logger.WithComponent("")
	if child.component != "" {
		t.Fatalf("component: got %q", child.component)
	}
	if child.output != logger.output {
		t.Fatal("child should inherit output")
	}
}

func TestLogger_Logger_WithComponent_Ugly(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelInfo)
	child := logger.WithComponent("worker/a")
	child.Info("ready")
	if logger.component != "test" {
		t.Fatalf("parent component changed: %q", logger.component)
	}
}

func TestLogger_Logger_SetLevel_Good(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelInfo)
	logger.SetLevel(LevelDebug)
	if logger.GetLevel() != LevelDebug {
		t.Fatalf("level: got %v", logger.GetLevel())
	}
}

func TestLogger_Logger_SetLevel_Bad(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelInfo)
	logger.SetLevel(Level(99))
	if logger.GetLevel() != Level(99) {
		t.Fatalf("level: got %v", logger.GetLevel())
	}
}

func TestLogger_Logger_SetLevel_Ugly(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelError)
	logger.SetLevel(LevelDebug)
	logger.SetLevel(LevelError)
	if logger.GetLevel() != LevelError {
		t.Fatalf("level: got %v", logger.GetLevel())
	}
}

func TestLogger_Logger_GetLevel_Good(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelWarn)
	got := logger.GetLevel()
	if got != LevelWarn {
		t.Fatalf("level: got %v", got)
	}
}

func TestLogger_Logger_GetLevel_Bad(t *testing.T) {
	logger, _ := axLoggerBuffer(LevelDebug)
	logger.SetLevel(LevelInfo)
	if logger.GetLevel() == LevelDebug {
		t.Fatal("expected changed level")
	}
}

func TestLogger_Logger_GetLevel_Ugly(t *testing.T) {
	logger, _ := axLoggerBuffer(Level(42))
	got := logger.GetLevel()
	if got != Level(42) {
		t.Fatalf("level: got %v", got)
	}
}

func TestLogger_Logger_Debug_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelDebug)
	logger.Debug("debug", Fields{"k": "v"})
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Debug_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelInfo)
	logger.Debug("debug")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Debug_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelDebug)
	logger.Debug("")
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Info_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelInfo)
	logger.Info("info")
	if !core.Contains(buf.String(), "[INFO]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Info_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelWarn)
	logger.Info("info")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Info_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelInfo)
	logger.Info("info", nil)
	if !core.Contains(buf.String(), "info") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Warn_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelWarn)
	logger.Warn("warn")
	if !core.Contains(buf.String(), "[WARN]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Warn_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelError)
	logger.Warn("warn")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Warn_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelWarn)
	logger.Warn("", Fields{"empty": true})
	if !core.Contains(buf.String(), "empty=true") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Error_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelError)
	logger.Error("error")
	if !core.Contains(buf.String(), "[ERROR]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Error_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(Level(99))
	logger.Error("error")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Error_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelError)
	logger.Error("", Fields{"code": 500})
	if !core.Contains(buf.String(), "code=500") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Debugf_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelDebug)
	logger.Debugf("debug %d", 1)
	if !core.Contains(buf.String(), "debug 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Debugf_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelInfo)
	logger.Debugf("debug %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Debugf_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelDebug)
	logger.Debugf("%s", "")
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Infof_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelInfo)
	logger.Infof("info %d", 1)
	if !core.Contains(buf.String(), "info 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Infof_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelWarn)
	logger.Infof("info %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Infof_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelInfo)
	logger.Infof("%s", "")
	if !core.Contains(buf.String(), "[INFO]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Warnf_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelWarn)
	logger.Warnf("warn %d", 1)
	if !core.Contains(buf.String(), "warn 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Warnf_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelError)
	logger.Warnf("warn %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Warnf_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelWarn)
	logger.Warnf("%s", "")
	if !core.Contains(buf.String(), "[WARN]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Errorf_Good(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelError)
	logger.Errorf("error %d", 1)
	if !core.Contains(buf.String(), "error 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Errorf_Bad(t *testing.T) {
	logger, buf := axLoggerBuffer(Level(99))
	logger.Errorf("error %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Logger_Errorf_Ugly(t *testing.T) {
	logger, buf := axLoggerBuffer(LevelError)
	logger.Errorf("%s", "")
	if !core.Contains(buf.String(), "[ERROR]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_SetGlobal_Good(t *testing.T) {
	previous := GetGlobal()
	logger, _ := axLoggerBuffer(LevelDebug)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	if GetGlobal() != logger {
		t.Fatal("global logger not set")
	}
}

func TestLogger_SetGlobal_Bad(t *testing.T) {
	previous := GetGlobal()
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(previous) })
	if GetGlobal() != nil {
		t.Fatal("expected nil global logger")
	}
}

func TestLogger_SetGlobal_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, _ := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	SetGlobal(previous)
	if GetGlobal() != previous {
		t.Fatal("global logger not restored")
	}
}

func TestLogger_GetGlobal_Good(t *testing.T) {
	logger := GetGlobal()
	if logger == nil {
		t.Fatal("expected global logger")
	}
	if logger.GetLevel() < LevelDebug {
		t.Fatal("unexpected level")
	}
}

func TestLogger_GetGlobal_Bad(t *testing.T) {
	previous := GetGlobal()
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(previous) })
	if GetGlobal() != nil {
		t.Fatal("expected nil global logger")
	}
}

func TestLogger_GetGlobal_Ugly(t *testing.T) {
	previous := GetGlobal()
	SetGlobal(previous)
	if GetGlobal() != previous {
		t.Fatal("global logger pointer changed")
	}
}

func TestLogger_SetGlobalLevel_Good(t *testing.T) {
	previous := GetGlobal()
	logger, _ := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	SetGlobalLevel(LevelDebug)
	if GetGlobal().GetLevel() != LevelDebug {
		t.Fatalf("level: got %v", GetGlobal().GetLevel())
	}
}

func TestLogger_SetGlobalLevel_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, _ := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	SetGlobalLevel(Level(99))
	if GetGlobal().GetLevel() != Level(99) {
		t.Fatalf("level: got %v", GetGlobal().GetLevel())
	}
}

func TestLogger_SetGlobalLevel_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, _ := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	SetGlobalLevel(LevelError)
	if GetGlobal().GetLevel() != LevelError {
		t.Fatalf("level: got %v", GetGlobal().GetLevel())
	}
}

func TestLogger_Debug_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelDebug)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Debug("debug")
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Debug_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Debug("debug")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Debug_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelDebug)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Debug("", nil)
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Info_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Info("info")
	if !core.Contains(buf.String(), "[INFO]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Info_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelWarn)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Info("info")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Info_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Info("", Fields{"k": "v"})
	if !core.Contains(buf.String(), "k=v") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Warn_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelWarn)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Warn("warn")
	if !core.Contains(buf.String(), "[WARN]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Warn_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Warn("warn")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Warn_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelWarn)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Warn("", Fields{"empty": true})
	if !core.Contains(buf.String(), "empty=true") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Error_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Error("error")
	if !core.Contains(buf.String(), "[ERROR]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Error_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(Level(99))
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Error("error")
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Error_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Error("", Fields{"code": 500})
	if !core.Contains(buf.String(), "code=500") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Debugf_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelDebug)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Debugf("debug %d", 1)
	if !core.Contains(buf.String(), "debug 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Debugf_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Debugf("debug %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Debugf_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelDebug)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Debugf("%s", "")
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Infof_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Infof("info %d", 1)
	if !core.Contains(buf.String(), "info 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Infof_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelWarn)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Infof("info %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Infof_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelInfo)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Infof("%s", "")
	if !core.Contains(buf.String(), "[INFO]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Warnf_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelWarn)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Warnf("warn %d", 1)
	if !core.Contains(buf.String(), "warn 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Warnf_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Warnf("warn %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Warnf_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelWarn)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Warnf("%s", "")
	if !core.Contains(buf.String(), "[WARN]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Errorf_Good(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Errorf("error %d", 1)
	if !core.Contains(buf.String(), "error 1") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Errorf_Bad(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(Level(99))
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Errorf("error %d", 1)
	if buf.Len() != 0 {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_Errorf_Ugly(t *testing.T) {
	previous := GetGlobal()
	logger, buf := axLoggerBuffer(LevelError)
	SetGlobal(logger)
	t.Cleanup(func() { SetGlobal(previous) })
	Errorf("%s", "")
	if !core.Contains(buf.String(), "[ERROR]") {
		t.Fatalf("log: %q", buf.String())
	}
}

func TestLogger_ParseLevel_Good(t *testing.T) {
	level, err := ParseLevel("debug")
	if err != nil {
		t.Fatalf("ParseLevel: %v", err)
	}
	if level != LevelDebug {
		t.Fatalf("level: got %v", level)
	}
}

func TestLogger_ParseLevel_Bad(t *testing.T) {
	level, err := ParseLevel("trace")
	if err == nil {
		t.Fatal("expected parse error")
	}
	if level != LevelInfo {
		t.Fatalf("fallback level: got %v", level)
	}
}

func TestLogger_ParseLevel_Ugly(t *testing.T) {
	level, err := ParseLevel("WARNING")
	if err != nil {
		t.Fatalf("ParseLevel: %v", err)
	}
	if level != LevelWarn {
		t.Fatalf("level: got %v", level)
	}
}

func TestLoggerDebugLevel(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output: buf,
		Level:  LevelDebug,
	})

	logger.Debug("debug message")
	if !core.Contains(buf.String(), "[DEBUG]") {
		t.Error("Debug message should appear at Debug level")
	}
}

func TestLoggerWithFields(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output: buf,
		Level:  LevelInfo,
	})

	logger.Info("test message", Fields{"key": "value", "num": 42})
	output := buf.String()

	if !core.Contains(output, "key=value") {
		t.Error("Field key=value should appear")
	}
	if !core.Contains(output, "num=42") {
		t.Error("Field num=42 should appear")
	}
}

func TestLoggerWithComponent(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output:    buf,
		Level:     LevelInfo,
		Component: "TestComponent",
	})

	logger.Info("test message")
	output := buf.String()

	if !core.Contains(output, "[TestComponent]") {
		t.Error("Component name should appear in log")
	}
}

func TestLoggerDerivedComponent(t *testing.T) {
	buf := core.NewBuffer()
	parent := New(Config{
		Output: buf,
		Level:  LevelInfo,
	})

	child := parent.WithComponent("ChildComponent")
	child.Info("child message")
	output := buf.String()

	if !core.Contains(output, "[ChildComponent]") {
		t.Error("Derived component name should appear")
	}
}

func TestLoggerFormatted(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output: buf,
		Level:  LevelInfo,
	})

	logger.Infof("formatted %s %d", "string", 123)
	output := buf.String()

	if !core.Contains(output, "formatted string 123") {
		t.Errorf("Formatted message should appear, got: %s", output)
	}
}

func TestSetLevel(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output: buf,
		Level:  LevelError,
	})

	// Info should not appear at Error level
	logger.Info("should not appear")
	if buf.Len() > 0 {
		t.Error("Info should not appear at Error level")
	}

	// Change to Info level
	logger.SetLevel(LevelInfo)
	logger.Info("should appear now")
	if !core.Contains(buf.String(), "should appear now") {
		t.Error("Info should appear after level change")
	}

	// Verify GetLevel
	if logger.GetLevel() != LevelInfo {
		t.Error("GetLevel should return LevelInfo")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"DEBUG", LevelDebug, false},
		{"debug", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"info", LevelInfo, false},
		{"WARN", LevelWarn, false},
		{"WARNING", LevelWarn, false},
		{"ERROR", LevelError, false},
		{"error", LevelError, false},
		{"invalid", LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)
			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.wantErr && level != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestGlobalLogger(t *testing.T) {
	buf := core.NewBuffer()
	logger := New(Config{
		Output: buf,
		Level:  LevelInfo,
	})

	SetGlobal(logger)

	Info("global test")
	if !core.Contains(buf.String(), "global test") {
		t.Error("Global logger should write message")
	}

	buf.Reset()
	SetGlobalLevel(LevelError)
	Info("should not appear")
	if buf.Len() > 0 {
		t.Error("Info should not appear at Error level")
	}

	// Reset to default for other tests
	SetGlobal(New(DefaultConfig()))
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("Level(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestMergeFields(t *testing.T) {
	// Empty fields
	result := mergeFields(nil)
	if result != nil {
		t.Error("nil input should return nil")
	}

	result = mergeFields([]Fields{})
	if result != nil {
		t.Error("empty input should return nil")
	}

	// Single fields
	result = mergeFields([]Fields{{"key": "value"}})
	if result["key"] != "value" {
		t.Error("Single field should be preserved")
	}

	// Multiple fields
	result = mergeFields([]Fields{
		{"key1": "value1"},
		{"key2": "value2"},
	})
	if result["key1"] != "value1" || result["key2"] != "value2" {
		t.Error("Multiple fields should be merged")
	}

	// Override
	result = mergeFields([]Fields{
		{"key": "value1"},
		{"key": "value2"},
	})
	if result["key"] != "value2" {
		t.Error("Later fields should override earlier ones")
	}
}
