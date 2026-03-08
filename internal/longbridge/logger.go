package longbridge

import (
	"fmt"
	"log"
	"strings"

	protocol "github.com/longportapp/openapi-protocol/go"
)

type sdkLogger struct {
	level protocol.LogLevel
}

func newSDKLogger(level string) *sdkLogger {
	logger := &sdkLogger{level: protocol.LevelInfo}
	logger.SetLevel(level)
	return logger
}

func (l *sdkLogger) SetLevel(level string) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		l.level = protocol.LevelDebug
	case "warn":
		l.level = protocol.LevelWarn
	case "error":
		l.level = protocol.LevelError
	default:
		l.level = protocol.LevelInfo
	}
}

func (l *sdkLogger) Info(msg string) {
	if l.level <= protocol.LevelInfo && !shouldSuppressSDKInfo(msg) {
		log.Println("[INFO]", msg)
	}
}

func (l *sdkLogger) Error(msg string) {
	if l.level <= protocol.LevelError && !shouldSuppressSDKError(msg) {
		log.Println("[ERR]", msg)
	}
}

func (l *sdkLogger) Warn(msg string) {
	if l.level <= protocol.LevelWarn {
		log.Println("[WARN]", msg)
	}
}

func (l *sdkLogger) Debug(msg string) {
	if l.level <= protocol.LevelDebug {
		log.Println("[DEBUG]", msg)
	}
}

func (l *sdkLogger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l *sdkLogger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l *sdkLogger) Warnf(format string, args ...interface{}) {
	if l.level <= protocol.LevelWarn {
		log.Printf("[WARN] "+format, args...)
	}
}

func (l *sdkLogger) Debugf(format string, args ...interface{}) {
	if l.level <= protocol.LevelDebug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func shouldSuppressSDKInfo(msg string) bool {
	switch strings.TrimSpace(msg) {
	case "start reconnecting.", "reconnect success":
		return true
	default:
		return false
	}
}

func shouldSuppressSDKError(msg string) bool {
	return strings.Contains(msg, "close conn, err: websocket: close 1006") ||
		(strings.Contains(msg, "close conn, err:") && strings.Contains(msg, "unexpected EOF"))
}
