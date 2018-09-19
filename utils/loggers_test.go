package utils

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"log"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func line() int {
	_, _, line, _ := runtime.Caller(1)
	return line
}

func TestLoggerFilenames(t *testing.T) {
	var buffer bytes.Buffer
	// Setup logrus
	logrus.AddHook(NewFilenameLoggerHook())
	logrus.SetOutput(&buffer)
	// Use logrus for standard log output
	log.SetFlags(log.Lshortfile)
	log.SetOutput(logrus.StandardLogger().Writer())

	log.Printf("This is a test from stdlog %d", 42); line1 := line()
	logrus.Warnf("This is logrus warning %d", 55); line2 := line()

	// Standard logging is asynchronous. Wait for it to be flushed.
	// PS: I feel dirty.
	// TODO: whack that part of Logrus with a stick.
	time.Sleep(100*time.Millisecond)

	split := strings.Split(buffer.String(), "\n")
	prefix1 := "source=\"loggers_test.go:" + strconv.Itoa(line1) + "\""
	prefix2 := "source=\"utils/loggers_test.go:" + strconv.Itoa(line2) + "\""

	assert.True(t, strings.HasSuffix(split[0], prefix1) || strings.HasSuffix(split[1], prefix1))
	assert.True(t, strings.HasSuffix(split[0], prefix2) || strings.HasSuffix(split[1], prefix2))
}

func TestContextLogging(t *testing.T) {
	var buffer bytes.Buffer
	// Setup logrus
	logrus.AddHook(NewFilenameLoggerHook())
	logrus.SetOutput(&buffer)

	ctx := SaveLoggerToContext(context.Background(), logrus.StandardLogger())
	ctx = SaveReqIdToContext(ctx, "req1")
	assert.Equal(t, "req1", GetReqIdFromContext(ctx))

	AddLoggerFields(ctx, logrus.Fields{"tag": "test"})
	CL(ctx).Warnf("This is a test")

	split := strings.Split(buffer.String(), "\n")
	assert.True(t, strings.Contains(split[0], "This is a test"))
}

func TestBadContext(t *testing.T) {
	assert.Panics(t, func() {
		AddLoggerFields(context.Background(), logrus.Fields{"tag": "test"})
	})

	assert.Panics(t, func() {
		GetReqIdFromContext(context.Background())
	})
}
