package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Lichas/maxclaw/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreEnsureFiles(t *testing.T) {
	workspace := t.TempDir()
	store := NewStore(workspace)

	require.NoError(t, store.EnsureFiles())
	assert.FileExists(t, filepath.Join(workspace, "memory", "MEMORY.md"))
	assert.FileExists(t, filepath.Join(workspace, "memory", "HISTORY.md"))
}

func TestStoreAppendHistory(t *testing.T) {
	workspace := t.TempDir()
	store := NewStore(workspace)

	require.NoError(t, store.AppendHistory("entry one"))
	require.NoError(t, store.AppendHistory("entry two"))

	body, err := os.ReadFile(store.HistoryPath())
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "entry one")
	assert.Contains(t, text, "entry two")
}

func TestConsolidateSessionUpdatesIndexAndIdempotent(t *testing.T) {
	workspace := t.TempDir()
	sess := &session.Session{
		Key: "cli:default",
		Messages: []session.Message{
			{Role: "user", Content: "First request", Timestamp: time.Now()},
			{Role: "assistant", Content: "First response", Timestamp: time.Now()},
			{Role: "user", Content: "Second request", Timestamp: time.Now()},
		},
	}

	changed, err := ConsolidateSession(workspace, sess, 1)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, 2, sess.LastConsolidated)

	changed, err = ConsolidateSession(workspace, sess, 1)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 2, sess.LastConsolidated)

	store := NewStore(workspace)
	body, err := os.ReadFile(store.HistoryPath())
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "session: cli:default")
	assert.Equal(t, 1, strings.Count(text, "session: cli:default"))
}

func TestArchiveSessionAll(t *testing.T) {
	workspace := t.TempDir()
	sess := &session.Session{
		Key: "telegram:chat-1",
		Messages: []session.Message{
			{Role: "user", Content: "Please prepare release notes", Timestamp: time.Now()},
			{Role: "assistant", Content: "Done", Timestamp: time.Now()},
		},
	}

	changed, err := ArchiveSessionAll(workspace, sess)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, len(sess.Messages), sess.LastConsolidated)

	store := NewStore(workspace)
	body, err := os.ReadFile(store.HistoryPath())
	require.NoError(t, err)
	assert.Contains(t, string(body), "Please prepare release notes")
}
