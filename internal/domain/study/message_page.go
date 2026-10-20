package study

import "errors"

// ErrInvalidMessageCursor is returned when a pagination cursor is not a
// usable message id.
var ErrInvalidMessageCursor = errors.New("message cursor is invalid")

// MessageCursor marks a position in a session's history: the id of a message.
// A page read with a cursor holds the messages that precede that message.
type MessageCursor struct {
	messageID string
}

// NewMessageCursor builds a cursor from a message id, or returns
// ErrInvalidMessageCursor when the id is empty. Whether the message belongs
// to the session is checked by MessageRepository.ListPage.
func NewMessageCursor(messageID string) (MessageCursor, error) {
	if messageID == "" {
		return MessageCursor{}, ErrInvalidMessageCursor
	}
	return MessageCursor{messageID: messageID}, nil
}

// MessageID returns the id of the message the cursor points at.
func (c MessageCursor) MessageID() string {
	return c.messageID
}

// MessagePage is one slice of a session's history. Messages are in
// chronological order; NextCursor points at the oldest of them and is nil
// when HasMore is false.
type MessagePage struct {
	Messages   []Message
	NextCursor *MessageCursor
	HasMore    bool
}

// NewMessagePage builds a page from newestFirst, the result of a
// MessageRepository.ListPage call asked for limit+1 messages: an extra,
// older message proves that more history exists. The extra message is
// dropped and the rest are put in chronological order.
func NewMessagePage(newestFirst []Message, limit int) MessagePage {
	hasMore := len(newestFirst) > limit
	if hasMore {
		newestFirst = newestFirst[:limit]
	}

	messages := make([]Message, len(newestFirst))
	for i, message := range newestFirst {
		messages[len(newestFirst)-1-i] = message
	}

	page := MessagePage{Messages: messages, HasMore: hasMore}
	if hasMore {
		cursor := MessageCursor{messageID: messages[0].ID}
		page.NextCursor = &cursor
	}
	return page
}
