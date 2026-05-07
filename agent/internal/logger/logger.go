package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// Level represents log severity.
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[Level]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

// Logger provides structured leveled logging.
type Logger struct {
	level  Level
	logger *log.Logger
}

// New creates a new Logger writing to stdout.
func New(level Level) *Logger {
	return &Logger{
		level:  level,
		logger: log.New(os.Stdout, "", 0),
	}
}

func (l *Logger) log(lvl Level, msg string, fields ...interface{}) {
	if lvl < l.level {
		return
	}
	ts := time.Now().Format("2006-01-02T15:04:05.000Z07:00")
	line := fmt.Sprintf("[%s] [%s] %s", ts, levelNames[lvl], msg)
	if len(fields) > 0 && len(fields)%2 == 0 {
		for i := 0; i < len(fields); i += 2 {
			line += fmt.Sprintf(" %v=%v", fields[i], fields[i+1])
		}
	}
	l.logger.Println(line)
}

func (l *Logger) Debug(msg string, fields ...interface{}) { l.log(DEBUG, msg, fields...) }
func (l *Logger) Info(msg string, fields ...interface{})  { l.log(INFO, msg, fields...) }
func (l *Logger) Warn(msg string, fields ...interface{})  { l.log(WARN, msg, fields...) }
func (l *Logger) Error(msg string, fields ...interface{}) { l.log(ERROR, msg, fields...) }
