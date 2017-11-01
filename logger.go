package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RFC5424 log message levels.
const (
	LevelEmergency = iota + 1
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInformational
	LevelDebug
)

// Legacy log level constants to ensure backwards compatibility.
const (
	LevelRaw   = 0
	LevelInfo  = LevelInformational
	LevelTrace = LevelDebug
	LevelWarn  = LevelWarning
)

// LoggerOption logger options
type LoggerOption struct {
	Access  bool   `label:"enable access log status"`
	Runtime bool   `label:"enable runtime log status"`
	Level   string `label:"runtime log level"`
	Path    string `label:"file log save path"`
}

// Logger log service
type Logger struct {
	level   int
	config  *Config
	backend *os.File
}

// Init init logger
func (l *Logger) Init() error {
	var mapper = map[string]int{"emergency": LevelEmergency, "alert": LevelAlert, "critical": LevelCritical, "error": LevelError, "warning": LevelWarning, "notice": LevelNotice, "informational": LevelInformational, "debug": LevelDebug, "info": LevelInformational, "trace": LevelDebug, "warn": LevelWarning}
	if "" == l.config.Logger.Level {
		l.config.Logger.Level = "error"
	}
	l.config.Logger.Level = strings.ToLower(l.config.Logger.Level)
	if _, ok := mapper[l.config.Logger.Level]; !ok {
		return errors.New("proxy: not support log message level " + l.config.Logger.Level)
	}
	l.level = mapper[l.config.Logger.Level]

	// only enable logger check save path
	if l.config.Logger.Access || l.config.Logger.Runtime {
		if "" == l.config.Logger.Path {
			target, err := exec.LookPath(os.Args[0])
			if nil != err {
				return err
			}
			target, err = filepath.Abs(target + "/../../log")
			if nil != err {
				return err
			}
			l.config.Logger.Path = target
		}

		l.config.Logger.Path = strings.TrimRight(l.config.Logger.Path, "/") + "/"
		if i, err := os.Stat(l.config.Logger.Path); err == nil {
			if !i.IsDir() {
				return errors.New("proxy: path " + l.config.Logger.Path + " must be direct")
			}
		} else {
			return err
		}

		f, err := os.OpenFile(l.config.Logger.Path+"runtime.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModeTemporary)
		if nil != err {
			return err
		}

		l.backend = f
	} else {
		l.backend = os.Stderr
	}

	return nil
}

// Write log to putput
func (l *Logger) Write(level int, format string, msg ...interface{}) {
	if level <= l.level {
		var msg = time.Now().Format("2006-01-02 15:04:05") + fmt.Sprintf(format, msg...)
		if _, err := l.backend.WriteString(msg); nil != err {
			fmt.Println("proxy: write log message error, ", err)
		}
	}
}
