package utils

import (
	"context"
	"github.com/sirupsen/logrus"
	"log"
	"os"
)

type ContextLogger struct {
	logger *logrus.Logger
	fields logrus.Fields
}

type loggerKey struct{}

var (
	loggerValueKey = &loggerKey{}
)

// Obtain the contextualized logger entry from the request context.
// Will use the default one if there's none available.
func CL(ctx context.Context) *logrus.Entry {
	l, ok := ctx.Value(loggerValueKey).(*ContextLogger)
	if !ok || l == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}

	fields := logrus.Fields{}
	for k, v := range l.fields {
		fields[k] = v
	}

	return l.logger.WithFields(fields)
}

// Add fields to a context logger, will panic if there's no logger associated
// with the context
func AddLoggerFields(ctx context.Context, fields logrus.Fields) {
	l, ok := ctx.Value(loggerValueKey).(*ContextLogger)
	if !ok || l == nil {
		panic("Trying to add fields to a context without a logger")
	}
	for k, v := range fields {
		l.fields[k] = v
	}
}

// ToContext sets a logrus logger on the context, which can then obtained by CL.
func SaveLoggerToContext(ctx context.Context, logger *logrus.Logger) context.Context {
	l := &ContextLogger{
		logger: logger,
		fields: logrus.Fields{},
	}
	return context.WithValue(ctx, loggerValueKey, l)
}

type reqIdContextKey struct {}

func GetReqIdFromContext(ctx context.Context) string {
	val, ok := ctx.Value(reqIdContextKey{}).(string)
	if !ok {
		panic("Can't find the RequestId in the context")
	}
	return val
}

func SaveReqIdToContext(ctx context.Context, requestId string) context.Context {
	return context.WithValue(ctx, reqIdContextKey{}, requestId)
}

func SetupClientLogging(verbose bool) {
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})
	// Add filename to output
	logrus.AddHook(NewFilenameLoggerHook())
	// Add filename to output
	logrus.SetOutput(os.Stderr)

	// Only log the warning severity or above.
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	log.SetFlags(log.Lshortfile)
	log.SetOutput(logrus.StandardLogger().Writer())
}
