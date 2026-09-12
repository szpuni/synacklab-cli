package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"synacklab/pkg/config"
)

func TestBuildLogger_DefaultsToInfoWhenUnset(t *testing.T) {
	logger, err := buildLogger(&config.Config{})
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestBuildLogger_UsesConfiguredLevel(t *testing.T) {
	logger, err := buildLogger(&config.Config{LogLevel: "warn"})
	require.NoError(t, err)
	require.NotNil(t, logger)
}

func TestBuildLogger_InvalidLevelIsError(t *testing.T) {
	_, err := buildLogger(&config.Config{LogLevel: "verbose"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verbose")
}
