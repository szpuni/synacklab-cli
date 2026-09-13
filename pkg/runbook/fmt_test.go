package runbook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatDocument_CanonicalizesKeyOrder(t *testing.T) {
	src := "```bash {timeout=30s, name=x, capture=A,B}\necho hi\n```\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, "```bash {name=x, capture=A,B, timeout=30s}\necho hi\n```\n", string(out))
}

func TestFormatDocument_LeavesProseCodeAndValuesUnchanged(t *testing.T) {
	src := "# Title\n\nSome   prose   with  odd   spacing.\n\n```bash {capture=B,A}\n" +
		"echo   'weird   spacing'\n```\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)

	got := string(out)
	assert.Contains(t, got, "Some   prose   with  odd   spacing.")
	assert.Contains(t, got, "echo   'weird   spacing'\n")
	assert.Contains(t, got, "capture=B,A", "attribute VALUES must not be reordered/reformatted, only key order/spacing")
}

func TestFormatDocument_NoAttributesStaysBare(t *testing.T) {
	src := "```bash\necho hi\n```\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, src, string(out))
}

func TestFormatDocument_NonRunnableFenceUntouched(t *testing.T) {
	src := "```yaml   {weird=spacing}\nkey: value\n```\n\n```bash {name=x}\necho hi\n```\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	assert.Contains(t, string(out), "```yaml   {weird=spacing}\nkey: value\n```\n")
}

func TestFormatDocument_PreservesFrontmatterVerbatim(t *testing.T) {
	src := "---\ndefault_timeout:   60s\n---\n```bash {timeout=1s, name=x}\necho hi\n```\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	assert.Contains(t, string(out), "---\ndefault_timeout:   60s\n---\n")
}

func TestFormatDocument_MalformedFenceIsErrorAndReturnsNil(t *testing.T) {
	src := "```bash {name=x\necho hi\n```\n"

	out, err := FormatDocument([]byte(src))
	require.Error(t, err)
	assert.Nil(t, out)
}

func TestFormatDocument_Idempotent(t *testing.T) {
	src := "```bash {timeout=30s, name=x, capture=A}\necho hi\n```\n"

	once, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	twice, err := FormatDocument(once)
	require.NoError(t, err)

	assert.Equal(t, string(once), string(twice))
}

func TestFormatDocument_PreservesFenceMarkerLength(t *testing.T) {
	src := "````bash {name=x}\necho hi\n````\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, "````bash {name=x}\necho hi\n````\n", string(out))
}
