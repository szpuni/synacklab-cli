package runbook

import (
	"strings"
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

func TestFormatDocument_CanonicalizesSensitiveAfterCapture(t *testing.T) {
	src := "```bash {sensitive=A, input=A,B, timeout=30s, name=x}\necho hi\n```\n"

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, "```bash {name=x, input=A,B, sensitive=A, timeout=30s}\necho hi\n```\n", string(out))
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

// TestFormatDocument_RewritesExactlyTheFencesParseRuns guards the rule fmt
// and the parser share: only non-empty top-level bash/python fences are
// runnable steps, and only those get their attributes canonicalized.
func TestFormatDocument_RewritesExactlyTheFencesParseRuns(t *testing.T) {
	src := "```bash {capture=A, name=one}\nexport A=1\n```\n\n" +
		"```sh {capture=B, name=shell}\necho sh\n```\n\n" +
		"```bash {capture=C, name=empty}\n```\n\n" +
		"- item\n\n  ```python {capture=D, name=nested}\n  print(1)\n  ```\n\n" +
		"```python {capture=E, name=two}\nprint(2)\n```\n"

	doc, err := Parse([]byte(src), "/tmp/runbook.md")
	require.NoError(t, err)
	assert.Len(t, doc.Steps, 2)
	assert.Contains(t, doc.Steps, "one")
	assert.Contains(t, doc.Steps, "two")

	out, err := FormatDocument([]byte(src))
	require.NoError(t, err)
	want := strings.NewReplacer(
		"{capture=A, name=one}", "{name=one, capture=A}",
		"{capture=E, name=two}", "{name=two, capture=E}",
	).Replace(src)
	assert.Equal(t, want, string(out))
}
