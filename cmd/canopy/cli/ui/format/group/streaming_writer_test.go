package group

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamingGroupWriter_GitHub(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamingGroupWriter(&buf, "Test Group", GitHub)

	// initially not started
	assert.False(t, sw.Started())

	// start the group
	err := sw.Start()
	require.NoError(t, err)
	assert.True(t, sw.Started())

	// check header was written
	assert.Equal(t, "::group::Test Group\n", buf.String())

	// write content
	n, err := sw.Write([]byte("line 1\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	n, err = sw.Write([]byte("line 2\n"))
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	// content is written immediately (no buffering)
	assert.Equal(t, "::group::Test Group\nline 1\nline 2\n", buf.String())

	// end the group
	err = sw.End()
	require.NoError(t, err)
	assert.False(t, sw.Started())

	// check footer was written
	assert.Equal(t, "::group::Test Group\nline 1\nline 2\n::endgroup::\n", buf.String())
}

func TestStreamingGroupWriter_Azure(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamingGroupWriter(&buf, "Azure Group", Azure)

	err := sw.Start()
	require.NoError(t, err)
	assert.Equal(t, "##[group]Azure Group\n", buf.String())

	_, _ = sw.Write([]byte("content\n"))

	err = sw.End()
	require.NoError(t, err)
	assert.Equal(t, "##[group]Azure Group\ncontent\n##[endgroup]\n", buf.String())
}

func TestStreamingGroupWriter_NilFormatter(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamingGroupWriter(&buf, "No Group", nil)

	err := sw.Start()
	require.NoError(t, err)
	assert.True(t, sw.Started()) // still marked as started

	// no header written
	assert.Empty(t, buf.String())

	_, _ = sw.Write([]byte("content\n"))
	assert.Equal(t, "content\n", buf.String())

	err = sw.End()
	require.NoError(t, err)

	// no footer written
	assert.Equal(t, "content\n", buf.String())
}

func TestStreamingGroupWriter_StartIdempotent(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamingGroupWriter(&buf, "Test", GitHub)

	// multiple starts should only write header once
	_ = sw.Start()
	_ = sw.Start()
	_ = sw.Start()

	assert.Equal(t, "::group::Test\n", buf.String())
}

func TestStreamingGroupWriter_EndWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamingGroupWriter(&buf, "Test", GitHub)

	// end without start should be a no-op
	err := sw.End()
	require.NoError(t, err)

	assert.Empty(t, buf.String())
	assert.False(t, sw.Started())
}

func TestStreamingGroupWriter_ReusableAfterEnd(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamingGroupWriter(&buf, "Test", GitHub)

	// first usage
	_ = sw.Start()
	_, _ = sw.Write([]byte("first\n"))
	_ = sw.End()

	expected := "::group::Test\nfirst\n::endgroup::\n"
	assert.Equal(t, expected, buf.String())

	// can be started again after end
	_ = sw.Start()
	_, _ = sw.Write([]byte("second\n"))
	_ = sw.End()

	expected += "::group::Test\nsecond\n::endgroup::\n"
	assert.Equal(t, expected, buf.String())
}
