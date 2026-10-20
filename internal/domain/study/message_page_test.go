package study

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageCursor_rejectsAnEmptyMessageID(t *testing.T) {
	// Given an empty message id
	// When building a cursor from it
	_, err := NewMessageCursor("")

	// Then it is rejected as an invalid cursor
	require.ErrorIs(t, err, ErrInvalidMessageCursor)
}

func TestNewMessageCursor_exposesTheMessageID(t *testing.T) {
	// Given a message id
	// When building a cursor from it
	cursor, err := NewMessageCursor("message-7")

	// Then the cursor carries that id
	require.NoError(t, err)
	assert.Equal(t, "message-7", cursor.MessageID())
}

func newestFirst(ids ...string) []Message {
	messages := make([]Message, len(ids))
	for i, id := range ids {
		messages[i] = Message{ID: id}
	}
	return messages
}

func pageIDs(page MessagePage) []string {
	ids := make([]string, len(page.Messages))
	for i, message := range page.Messages {
		ids[i] = message.ID
	}
	return ids
}

func TestNewMessagePage_withFewerMessagesThanTheLimit_hasNoMore(t *testing.T) {
	// Given two messages (newest first) and a limit of three
	// When building the page
	page := NewMessagePage(newestFirst("m3", "m2"), 3)

	// Then both are returned in chronological order and nothing is left
	assert.Equal(t, []string{"m2", "m3"}, pageIDs(page))
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextCursor)
}

func TestNewMessagePage_withExactlyTheLimit_hasNoMore(t *testing.T) {
	// Given exactly as many messages as the limit (the repository fetched
	// limit+1 and found no extra one)
	// When building the page
	page := NewMessagePage(newestFirst("m3", "m2", "m1"), 3)

	// Then the page is complete and has no cursor
	assert.Equal(t, []string{"m1", "m2", "m3"}, pageIDs(page))
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextCursor)
}

func TestNewMessagePage_withAnExtraMessage_dropsItAndPointsTheCursorAtTheOldestKept(t *testing.T) {
	// Given limit+1 messages: the extra, oldest one proves there is more
	// When building the page
	page := NewMessagePage(newestFirst("m4", "m3", "m2", "m1"), 3)

	// Then the extra message is dropped, the rest are chronological, and
	// the cursor is the oldest message kept
	assert.Equal(t, []string{"m2", "m3", "m4"}, pageIDs(page))
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextCursor)
	assert.Equal(t, "m2", page.NextCursor.MessageID())
}

func TestNewMessagePage_withNoMessages_isEmpty(t *testing.T) {
	// Given no messages
	// When building the page
	page := NewMessagePage(nil, 50)

	// Then it is an empty, non-nil page
	assert.NotNil(t, page.Messages)
	assert.Empty(t, page.Messages)
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextCursor)
}
