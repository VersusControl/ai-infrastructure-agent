package logging

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
}

func NewLogger(level, format string) *Logger {
	logger := logrus.New()

	// Set log level
	switch level {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
	}

	// Set formatter
	if format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	return &Logger{Logger: logger}
}

// WithContext adds context information to log entries
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.WithFields(logrus.Fields{})

	if requestID := ctx.Value("request_id"); requestID != nil {
		entry = entry.WithField("request_id", requestID)
	}

	if userID := ctx.Value("user_id"); userID != nil {
		entry = entry.WithField("user_id", userID)
	}

	return entry
}

// LogMCPRequest logs incoming MCP requests
func (l *Logger) LogMCPRequest(method string, duration time.Duration, err error) {
	fields := logrus.Fields{
		"type":     "mcp_request",
		"method":   method,
		"duration": duration.Milliseconds(),
	}

	if err != nil {
		fields["error"] = err.Error()
		l.WithFields(fields).Error("MCP request failed")
	} else {
		l.WithFields(fields).Info("MCP request completed")
	}
}

func (l *Logger) LogMCPCallTool(name string, arguments map[string]interface{}) {
	l.WithFields(logrus.Fields{
		"tool":      name,
		"arguments": arguments,
	}).Info("Processing MCP tool call")
}
