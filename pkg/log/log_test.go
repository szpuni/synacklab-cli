package log

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLevel_ValidValues(t *testing.T) {
	cases := map[string]Level{
		"error":   LevelError,
		"ERROR":   LevelError,
		"warn":    LevelWarn,
		"warning": LevelWarn,
		"info":    LevelInfo,
		"":        LevelInfo,
		"  info ": LevelInfo,
	}
	for input, want := range cases {
		got, err := ParseLevel(input)
		require.NoError(t, err, "input=%q", input)
		assert.Equal(t, want, got, "input=%q", input)
	}
}

func TestParseLevel_InvalidValueIsError(t *testing.T) {
	_, err := ParseLevel("verbose")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verbose")
}

func TestLogger_InfoLevelShowsAllThreeLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, LevelInfo)

	logger.Error("err %d", 1)
	logger.Warn("warn %d", 2)
	logger.Info("info %d", 3)

	out := buf.String()
	assert.Contains(t, out, "ERROR")
	assert.Contains(t, out, "err 1")
	assert.Contains(t, out, "WARN")
	assert.Contains(t, out, "warn 2")
	assert.Contains(t, out, "INFO")
	assert.Contains(t, out, "info 3")
}

func TestLogger_WarnLevelHidesInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, LevelWarn)

	logger.Warn("visible warning")
	logger.Info("hidden info")

	out := buf.String()
	assert.Contains(t, out, "visible warning")
	assert.NotContains(t, out, "hidden info")
}

func TestLogger_ErrorLevelHidesWarnAndInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, LevelError)

	logger.Error("visible error")
	logger.Warn("hidden warning")
	logger.Info("hidden info")

	out := buf.String()
	assert.Contains(t, out, "visible error")
	assert.NotContains(t, out, "hidden warning")
	assert.NotContains(t, out, "hidden info")
}

func TestLogger_EachLineHasATimestamp(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, LevelInfo)

	logger.Info("hello")

	line := strings.TrimSpace(buf.String())
	fields := strings.Fields(line)
	require.NotEmpty(t, fields)
	assert.Contains(t, fields[0], "T", "first field should look like an RFC3339-ish timestamp")
}

func TestDiscard_NeverPanicsAndProducesNoOutput(t *testing.T) {
	assert.NotPanics(t, func() {
		logger := Discard()
		logger.Error("e")
		logger.Warn("w")
		logger.Info("i")
	})
}
