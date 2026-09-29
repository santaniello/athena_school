package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestGetSourceDocument_returnsErrSourceNotFound_whenItemDoesNotExist(t *testing.T) {
	// Given an item id nothing owns
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := newTestService(nil, nil, items, nil, nil, nil, nil, nil)

	// When getting its document
	_, err := service.GetSourceDocument(ctx, testSessionID, "missing")

	// Then it reports ErrSourceNotFound
	assert.ErrorIs(t, err, ErrSourceNotFound)
}

func TestGetSourceDocument_returnsErrSourceNotFound_whenItemBelongsToAnotherSession(t *testing.T) {
	// Given an item owned by a different session
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: "other-session"}, nil).Once()
	service := newTestService(nil, nil, items, nil, nil, nil, nil, nil)

	// When session-1 tries to get its document
	_, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then it is rejected the same way as a non-existent item
	assert.ErrorIs(t, err, ErrSourceNotFound)
}

func TestGetSourceDocument_returnsAGenericItemLookupError_wrapped(t *testing.T) {
	// Given a repository error unrelated to "not found"
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	boom := errors.New("database unavailable")
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{}, boom).Once()
	service := newTestService(nil, nil, items, nil, nil, nil, nil, nil)

	// When getting its document
	_, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then the underlying error is preserved
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestGetSourceDocument_returnsErrSourceTextUnavailable_whenNoDocumentIsStored(t *testing.T) {
	// Given an item that exists but has no stored document text (e.g.
	// imported before spec 2.18)
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(ctx, testSessionID, "item-1").Return("", domainknowledge.ErrDocumentNotFound).Once()
	service := newTestService(nil, nil, items, documents, nil, nil, nil, nil)

	// When getting its document
	_, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then it reports ErrSourceTextUnavailable
	assert.ErrorIs(t, err, ErrSourceTextUnavailable)
}

func TestGetSourceDocument_returnsAGenericDocumentLookupError_wrapped(t *testing.T) {
	// Given a document-repository error unrelated to "not found"
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	boom := errors.New("database unavailable")
	documents.EXPECT().Get(ctx, testSessionID, "item-1").Return("", boom).Once()
	service := newTestService(nil, nil, items, documents, nil, nil, nil, nil)

	// When getting its document
	_, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then the underlying error is preserved
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestGetSourceDocument_returnsTheChunkListingError_wrapped(t *testing.T) {
	// Given chunk listing that fails after ownership and text both check out
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(ctx, testSessionID, "item-1").Return("content", nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	boom := errors.New("database unavailable")
	chunks.EXPECT().ListByItemID(ctx, "item-1").Return(nil, boom).Once()
	service := newTestService(chunks, nil, items, documents, nil, nil, nil, nil)

	// When getting its document
	_, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then the underlying error is preserved
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestGetSourceDocument_returnsTheDocumentWithOrderedSegmentsAndTitleAndPath(t *testing.T) {
	// Given an item, its stored text and its chunks
	ctx := context.Background()
	content := "Intro text. Chunk one here. Middle gap. Chunk two here. Trailing text."
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").
		Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID, Concept: "Distributed Systems"}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(ctx, testSessionID, "item-1").Return(content, nil).Once()
	start1, end1 := 12, 27
	start2, end2 := 41, 56
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListByItemID(ctx, "item-1").Return([]domainknowledge.Chunk{
		{ID: "chunk-1", FilePath: "notes/ds.md", StartOffset: &start1, EndOffset: &end1},
		{ID: "chunk-2", FilePath: "notes/ds.md", StartOffset: &start2, EndOffset: &end2},
	}, nil).Once()
	service := newTestService(chunks, nil, items, documents, nil, nil, nil, nil)

	// When getting its document
	got, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then the title/path come from the item/chunks, and the segments cut
	// the content at the chunk boundaries with the gaps in between
	require.NoError(t, err)
	assert.Equal(t, "item-1", got.ItemID)
	assert.Equal(t, "Distributed Systems", got.Title)
	assert.Equal(t, "notes/ds.md", got.Path)
	require.Len(t, got.Segments, 5)
	assert.Equal(t, domainknowledge.DocumentSegment{Text: content[:12]}, got.Segments[0])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: content[12:27], ChunkID: "chunk-1"}, got.Segments[1])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: content[27:41]}, got.Segments[2])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: content[41:56], ChunkID: "chunk-2"}, got.Segments[3])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: content[56:]}, got.Segments[4])

	// And the segments reassemble to the stored text exactly
	var reassembled string
	for _, s := range got.Segments {
		reassembled += s.Text
	}
	assert.Equal(t, content, reassembled)
}

func TestGetSourceDocument_returnsAnEmptyPath_whenTheItemOwnsNoChunks(t *testing.T) {
	// Given an item with stored text but (unusually) no chunks at all
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	documents := knowledgemocks.NewMockDocumentRepository(t)
	documents.EXPECT().Get(ctx, testSessionID, "item-1").Return("content", nil).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListByItemID(ctx, "item-1").Return(nil, nil).Once()
	service := newTestService(chunks, nil, items, documents, nil, nil, nil, nil)

	// When getting its document
	got, err := service.GetSourceDocument(ctx, testSessionID, "item-1")

	// Then it degrades to an empty path and one whole-content gap segment,
	// rather than failing
	require.NoError(t, err)
	assert.Empty(t, got.Path)
	assert.Equal(t, []domainknowledge.DocumentSegment{{Text: "content"}}, got.Segments)
}

func TestBuildSegments_reassemblesToContentExactly_whenChunksCoverTheWholeContent(t *testing.T) {
	// Given two adjacent chunks covering all of content, no gaps
	content := "firstsecond"
	start1, end1 := 0, 5
	start2, end2 := 5, 11
	chunks := []domainknowledge.Chunk{
		{ID: "c1", StartOffset: &start1, EndOffset: &end1},
		{ID: "c2", StartOffset: &start2, EndOffset: &end2},
	}

	// When building segments
	segments := buildSegments(content, chunks)

	// Then there are exactly two segments, no gap segments, and they
	// reassemble to content exactly
	require.Len(t, segments, 2)
	assert.Equal(t, domainknowledge.DocumentSegment{Text: "first", ChunkID: "c1"}, segments[0])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: "second", ChunkID: "c2"}, segments[1])
}

func TestBuildSegments_skipsAChunkWithNoOffset_ratherThanFailing(t *testing.T) {
	// Given one chunk with offsets and one pre-2.18 chunk with none
	content := "hello world"
	start, end := 0, 5
	chunks := []domainknowledge.Chunk{
		{ID: "c1", StartOffset: &start, EndOffset: &end},
		{ID: "c2-no-offset"},
	}

	// When building segments
	segments := buildSegments(content, chunks)

	// Then the offset-less chunk is skipped, not turned into a broken
	// segment, and the rest of the content still comes back as a gap
	require.Len(t, segments, 2)
	assert.Equal(t, domainknowledge.DocumentSegment{Text: "hello", ChunkID: "c1"}, segments[0])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: " world"}, segments[1])
}

func TestBuildSegments_skipsAChunkWhoseOffsetsDoNotFitContent(t *testing.T) {
	// Given a chunk whose End is past the end of content (corrupt/stale data)
	content := "short"
	start, end := 0, 999
	chunks := []domainknowledge.Chunk{
		{ID: "c1", StartOffset: &start, EndOffset: &end},
	}

	// When building segments
	segments := buildSegments(content, chunks)

	// Then the out-of-range chunk is skipped and the whole content comes
	// back as one gap segment, instead of panicking on a bad slice
	assert.Equal(t, []domainknowledge.DocumentSegment{{Text: "short"}}, segments)
}

func TestBuildSegments_returnsOneGapSegment_whenThereAreNoChunksAtAll(t *testing.T) {
	// Given content with no chunks
	segments := buildSegments("just some text", nil)

	// Then the whole content comes back as a single gap segment
	assert.Equal(t, []domainknowledge.DocumentSegment{{Text: "just some text"}}, segments)
}

func TestBuildSegments_keepsAZeroLengthChunk_asItsOwnEmptySegment(t *testing.T) {
	// Given a chunk whose Start equals its End (a degenerate, but not
	// invalid, zero-length span) sitting between two gaps
	content := "before after"
	start, end := 6, 6
	chunks := []domainknowledge.Chunk{
		{ID: "c1", StartOffset: &start, EndOffset: &end},
	}

	// When building segments
	segments := buildSegments(content, chunks)

	// Then the zero-length chunk is kept as its own empty segment — not
	// silently dropped into the surrounding gap
	require.Len(t, segments, 3)
	assert.Equal(t, domainknowledge.DocumentSegment{Text: "before"}, segments[0])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: "", ChunkID: "c1"}, segments[1])
	assert.Equal(t, domainknowledge.DocumentSegment{Text: " after"}, segments[2])
}

func TestBuildSegments_returnsNoSegments_forEmptyContentAndNoChunks(t *testing.T) {
	// Given empty content and no chunks
	segments := buildSegments("", nil)

	// Then there is nothing to segment
	assert.Empty(t, segments)
}
