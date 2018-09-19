package utils

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type Hook struct {
	Field     string
	Skip      int
	levels    []logrus.Level
	Formatter func(file, function string, line int) string
}

func (hook *Hook) Levels() []logrus.Level {
	return hook.levels
}

func (hook *Hook) Fire(entry *logrus.Entry) error {
	file, function, line := findCaller(hook.Skip)

	if strings.HasSuffix(file,".s") {
		// The stack trace ends up in the runtime.
		// This means we're running in stdlog compat mode.
		// Try to extract the filename by parsing the message
		globalFlags := log.Flags()
		if globalFlags & log.Lshortfile == 0 &&  globalFlags & log.Llongfile == 0 {
			//No chances, the file name is lost to the mists of time
			return nil
		}
		// Try to parse the log message to extract the line and file
		var source string
		source, entry.Message = hook.parseStdLogMessage(entry.Message)
		if source != "" {
			entry.Data[hook.Field] = source
		}
		return nil
	} else {
		entry.Data[hook.Field] = hook.Formatter(file, function, line)
		return nil
	}
}

func (hook *Hook) parseStdLogMessage(logMsg string) (string, string) {
	prefixLen := len(log.Prefix())
	dePrefixed := logMsg[prefixLen:]

	// The line looks like "[PREFIX ][datetimestamp ]server.go:112: ...."
	locationEndPos := strings.Index(dePrefixed, ": ")
	if locationEndPos == -1 {
		return "", logMsg
	}
	locationStartPos := strings.LastIndex(logMsg[0:locationEndPos], " ")
	if locationStartPos == -1 {
		locationStartPos = 0
	}

	location := logMsg[locationStartPos:locationEndPos]
	locationParts := strings.Split(location, ":")
	line, _ := strconv.Atoi(locationParts[1])

	// And now remove the filename from the log message (to avoid duplication)
	logMsg = logMsg[0:prefixLen] + logMsg[prefixLen:locationStartPos+prefixLen] +
		logMsg[locationEndPos+prefixLen+2:]

	return hook.Formatter(locationParts[0], "", line), logMsg
}

func NewFilenameLoggerHook(levels ...logrus.Level) *Hook {
	hook := Hook{
		Field:  "source",
		Skip:   5,
		levels: levels,
		Formatter: func(file, function string, line int) string {
			return fmt.Sprintf("%s:%d", file, line)
		},
	}
	if len(hook.levels) == 0 {
		hook.levels = logrus.AllLevels
	}

	return &hook
}

func findCaller(skip int) (string, string, int) {
	var (
		pc       uintptr
		file     string
		function string
		line     int
	)
	for i := 0; i < 10; i++ {
		pc, file, line = getCaller(skip + i)
		if !strings.HasPrefix(file, "logrus/") {
			break
		}
	}
	if pc != 0 {
		frames := runtime.CallersFrames([]uintptr{pc})
		frame, _ := frames.Next()
		function = frame.Function
	}

	return file, function, line
}

func getCaller(skip int) (uintptr, string, int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return 0, "", 0
	}

	n := 0
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			n += 1
			if n >= 2 {
				file = file[i+1:]
				break
			}
		}
	}

	return pc, file, line
}

