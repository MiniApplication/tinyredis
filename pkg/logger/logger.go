package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/hsn/tiny-redis/pkg/config"
)

type LogConfig struct {
	Path  string
	Name  string
	Level LogLevel
}

var (
	logFile            *os.File
	logger             *log.Logger
	logMu              sync.Mutex
	levelLabels        = []string{"debug", "info", "warning", "error", "panic"}
	logcfg             *LogConfig
	defaultCallerDepth = 2
	logPrefix          = ""
)

func SetUp(cfg *config.Config) (err error) {
	logcfg = &LogConfig{
		Path:  cfg.LogDir,
		Name:  "redis.log",
		Level: INFO,
	}
	for i, v := range levelLabels {
		if v == cfg.LogLevel {
			logcfg.Level = LogLevel(i)
			break
		}
	}
	if _, err = os.Stat(logcfg.Path); err != nil {
		mkErr := os.Mkdir(logcfg.Path, 0755)
		if mkErr != nil {
			return mkErr
		}
	}
	logfile := path.Join(logcfg.Path, logcfg.Name)
	logFile, err = os.OpenFile(logfile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	writer := io.MultiWriter(os.Stdout, logFile)
	logger = log.New(writer, "", log.LstdFlags)

	return nil
}
func Disable() {
	logger.SetOutput(io.Discard)
}
func setPrefix(level LogLevel) {
	_, file, line, ok := runtime.Caller(defaultCallerDepth)
	if ok {
		logPrefix = fmt.Sprintf("[%s][%s:%d] ", levelLabels[level], filepath.Base(file), line)
	} else {
		logPrefix = fmt.Sprintf("[%s] ", levelLabels[level])
	}
	logger.SetPrefix(logPrefix)
}
