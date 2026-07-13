package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
)

func newTestDBStore(t *testing.T) dbStore {
	t.Helper()
	d, err := db.New(":memory:")
	require.NoError(t, err)
	return dbStore{Store: d}
}

func TestGetOrCreateSession(t *testing.T) {
	s := newTestDBStore(t)

	// same name resolves to the same session on repeat calls (idempotent)
	first, err := s.getOrCreateSession("work")
	require.NoError(t, err)
	require.NotNil(t, first)

	again, err := s.getOrCreateSession("work")
	require.NoError(t, err)
	require.NotNil(t, again)
	require.Equal(t, first.uuid, again.uuid)

	// a distinct name resolves to a distinct session
	other, err := s.getOrCreateSession("debug")
	require.NoError(t, err)
	require.NotNil(t, other)
	require.NotEqual(t, first.uuid, other.uuid)
}
