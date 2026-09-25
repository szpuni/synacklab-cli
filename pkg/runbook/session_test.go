package runbook

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionStore_SetVarLastWriteWins(t *testing.T) {
	s := NewSessionStore("sess-1", "/tmp/runbook.md", "/tmp")

	s.SetVar("PUBLIC_IP", "1.2.3.4")
	s.SetVar("PUBLIC_IP", "5.6.7.8")

	assert.Equal(t, "5.6.7.8", s.Get().Vars["PUBLIC_IP"])
}

func TestSessionStore_SetCwd(t *testing.T) {
	s := NewSessionStore("sess-1", "/tmp/runbook.md", "/tmp")

	assert.Equal(t, "/tmp", s.Get().Cwd)
	s.SetCwd("/tmp/subdir")
	assert.Equal(t, "/tmp/subdir", s.Get().Cwd)
}

func TestSessionStore_AppendHistoryPreservesOrder(t *testing.T) {
	s := NewSessionStore("sess-1", "/tmp/runbook.md", "/tmp")

	s.AppendHistory(Execution{StepName: "first", Started: time.Now()})
	s.AppendHistory(Execution{StepName: "second", Started: time.Now()})

	hist := s.Get().History
	if assert.Len(t, hist, 2) {
		assert.Equal(t, "first", hist[0].StepName)
		assert.Equal(t, "second", hist[1].StepName)
	}
}

func TestSessionStore_ResetClearsVarsAndCwdButKeepsHistory(t *testing.T) {
	s := NewSessionStore("sess-1", "/tmp/runbook.md", "/tmp")
	s.SetVar("A", "1")
	s.SetCwd("/tmp/subdir")
	s.AppendHistory(Execution{StepName: "first"})

	s.Reset()

	got := s.Get()
	assert.Empty(t, got.Vars)
	assert.Equal(t, "/tmp", got.Cwd)
	assert.Len(t, got.History, 1, "reset should not clear execution history")
}

func TestSessionStore_GetReturnsIndependentSnapshot(t *testing.T) {
	s := NewSessionStore("sess-1", "/tmp/runbook.md", "/tmp")
	s.SetVar("A", "1")

	snapshot := s.Get()
	snapshot.Vars["A"] = "mutated"

	assert.Equal(t, "1", s.Get().Vars["A"], "mutating a Get() snapshot must not affect stored state")
}

func TestSessionStore_ConcurrentSetVarIsRaceFree(t *testing.T) {
	s := NewSessionStore("sess-1", "/tmp/runbook.md", "/tmp")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.SetVar("K", "v")
			_ = s.Get()
		}()
	}
	wg.Wait()

	assert.Equal(t, "v", s.Get().Vars["K"])
}
