package runbook

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrontmatter_PresentWithAllFields(t *testing.T) {
	src := []byte("---\ndefault_timeout: 60s\ndanger_patterns:\n  - \"rm -rf\"\n  - \"terraform destroy\"\n---\n# Title\n\nbody\n")

	fm, rest, err := parseFrontmatter(src)
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, fm.DefaultTimeout)
	assert.Equal(t, []string{"rm -rf", "terraform destroy"}, fm.DangerPatterns)
	assert.Equal(t, "# Title\n\nbody\n", string(rest))
}

func TestParseFrontmatter_AbsentReturnsDefaults(t *testing.T) {
	src := []byte("# Title\n\nbody\n")

	fm, rest, err := parseFrontmatter(src)
	require.NoError(t, err)
	assert.Equal(t, time.Duration(0), fm.DefaultTimeout)
	assert.Nil(t, fm.DangerPatterns)
	assert.Equal(t, string(src), string(rest))
}

func TestParseFrontmatter_PartialFieldsLeaveOthersZero(t *testing.T) {
	src := []byte("---\ndefault_timeout: 5m\n---\nbody\n")

	fm, _, err := parseFrontmatter(src)
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, fm.DefaultTimeout)
	assert.Nil(t, fm.DangerPatterns)
}

func TestParseFrontmatter_UnclosedBlockIsError(t *testing.T) {
	src := []byte("---\ndefault_timeout: 5m\nbody without closing delimiter\n")

	_, _, err := parseFrontmatter(src)
	assert.Error(t, err)
}

func TestParseFrontmatter_MalformedYAMLIsError(t *testing.T) {
	src := []byte("---\ndefault_timeout: [unterminated\n---\nbody\n")

	_, _, err := parseFrontmatter(src)
	assert.Error(t, err)
}

func TestParseFrontmatter_InvalidDurationIsError(t *testing.T) {
	src := []byte("---\ndefault_timeout: not-a-duration\n---\nbody\n")

	_, _, err := parseFrontmatter(src)
	assert.Error(t, err)
}
