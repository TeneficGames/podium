// podium
// https://github.com/TeneficGames/podium
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2026 Tenefic Games
// Forked from
// https://github.com/topfreegames/podium
// Copyright © 2016 Top Free Games

package log

import (
	"bytes"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogHelpers(t *testing.T) {
	var output bytes.Buffer
	logger := CreateLoggerWithLevel(zap.DebugLevel, LoggerOptions{
		WriteSyncer:     zapcore.AddSync(&output),
		RemoveTimestamp: true,
	})

	D(logger, "debug")
	I(logger, "info", func(message CM) {
		message.Write(zap.String("source", "test"))
	})
	W(logger, "warn")
	E(logger, "error")

	logs := output.String()
	for _, expected := range []string{
		`"level":"debug","msg":"debug"`,
		`"level":"info","msg":"info","source":"test"`,
		`"level":"warn","msg":"warn"`,
		`"level":"error","msg":"error"`,
	} {
		if !strings.Contains(logs, expected) {
			t.Fatalf("expected log output to contain %q, got %s", expected, logs)
		}
	}
	if strings.Contains(logs, `"ts":`) {
		t.Fatalf("expected timestamps to be removed, got %s", logs)
	}
}

func TestLogSkipsDisabledLevel(t *testing.T) {
	var output bytes.Buffer
	logger := CreateLoggerWithLevel(zap.InfoLevel, LoggerOptions{
		WriteSyncer: zapcore.AddSync(&output),
	})
	callbackCalled := false

	D(logger, "debug", func(message CM) {
		callbackCalled = true
		message.Write()
	})

	if callbackCalled {
		t.Fatal("expected callback not to run for a disabled log level")
	}
	if output.Len() != 0 {
		t.Fatalf("expected no output, got %s", output.String())
	}
}

func TestPanicLog(t *testing.T) {
	var output bytes.Buffer
	logger := CreateLoggerWithLevel(zap.PanicLevel, LoggerOptions{
		WriteSyncer: zapcore.AddSync(&output),
	})

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic log to panic")
			}
		}()
		P(logger, "panic")
	}()

	if !strings.Contains(output.String(), `"level":"panic"`) ||
		!strings.Contains(output.String(), `"msg":"panic"`) {
		t.Fatalf("expected panic message, got %s", output.String())
	}
}
