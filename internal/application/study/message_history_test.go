package study

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// newestFirstMessages returns count messages "message-<count>" … "message-1",
// newest first, as MessageRepository.ListPage returns them.
func newestFirstMessages(count int) []domainstudy.Message {
	messages := make([]domainstudy.Message, 0, count)
	for i := count; i >= 1; i-- {
		messages = append(messages, domainstudy.Message{ID: fmt.Sprintf("message-%d", i), SessionID: "session-1"})
	}
	return messages
}

func newHistoryService(
	t *testing.T, messages *studymocks.MockMessageRepository, sources *knowledgemocks.MockMessageSourceRepository,
) *Service {
	t.Helper()
	return NewService(
		studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t),
		foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), nil, nil, sources, nil, nil,
	)
}

func TestListMessagesBefore_returnsTheOlderPageWithItsSourcesOnly(t *testing.T) {
	// Given a session where, before message-10, three messages remain (asked
	// for limit+1 = 3 with a limit of 2, so the third proves there is more)
	messages := studymocks.NewMockMessageRepository(t)
	cursor, err := domainstudy.NewMessageCursor("message-10")
	require.NoError(t, err)
	messages.EXPECT().
		ListPage(context.Background(), "session-1", &cursor, 3).
		Return([]domainstudy.Message{{ID: "message-9"}, {ID: "message-8"}, {ID: "message-7"}}, nil).
		Once()
	persisted := []domainknowledge.Source{{ChunkID: "chunk-1"}}
	sources := knowledgemocks.NewMockMessageSourceRepository(t)
	sources.EXPECT().
		ListByMessageIDs(context.Background(), []string{"message-8", "message-9"}).
		Return(map[string][]domainknowledge.Source{"message-9": persisted}, nil).
		Once()
	service := newHistoryService(t, messages, sources)

	// When asking for 2 messages before message-10
	page, err := service.ListMessagesBefore(context.Background(), "session-1", "message-10", 2)

	// Then the page is chronological, carries sources only for its own
	// messages, and points further back from its oldest one
	require.NoError(t, err)
	require.Equal(t, []MessageWithSources{
		{Message: domainstudy.Message{ID: "message-8"}},
		{Message: domainstudy.Message{ID: "message-9"}, Sources: persisted},
	}, page.Messages)
	require.True(t, page.HasMore)
	require.Equal(t, "message-8", page.NextCursor)
}

func TestListMessagesBefore_clampsTheLimit(t *testing.T) {
	cases := []struct {
		name      string
		limit     int
		wantFetch int
	}{
		{"zero uses the default", 0, DefaultMessagePageSize + 1},
		{"negative uses the default", -5, DefaultMessagePageSize + 1},
		{"the minimum is kept", 1, 2},
		{"within range is kept", 10, 11},
		{"the maximum is kept", MaxMessagePageSize, MaxMessagePageSize + 1},
		{"above the maximum is capped", MaxMessagePageSize + 1, MaxMessagePageSize + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given a repository that records the limit it is asked for
			messages := studymocks.NewMockMessageRepository(t)
			cursor, err := domainstudy.NewMessageCursor("message-10")
			require.NoError(t, err)
			messages.EXPECT().
				ListPage(context.Background(), "session-1", &cursor, tc.wantFetch).
				Return(nil, nil).
				Once()
			sources := knowledgemocks.NewMockMessageSourceRepository(t)
			sources.EXPECT().ListByMessageIDs(context.Background(), []string{}).Return(nil, nil).Once()
			service := newHistoryService(t, messages, sources)

			// When asking for a page with that limit
			_, err = service.ListMessagesBefore(context.Background(), "session-1", "message-10", tc.limit)

			// Then the repository was asked for the clamped limit plus one
			require.NoError(t, err)
		})
	}
}

func TestListMessagesBefore_rejectsAnEmptyCursorWithoutReadingAnything(t *testing.T) {
	// Given repositories that must not be touched
	service := newHistoryService(t, studymocks.NewMockMessageRepository(t), knowledgemocks.NewMockMessageSourceRepository(t))

	// When asking for a page with an empty cursor
	_, err := service.ListMessagesBefore(context.Background(), "session-1", "", 10)

	// Then it fails as an invalid cursor
	require.ErrorIs(t, err, domainstudy.ErrInvalidMessageCursor)
}

func TestListMessagesBefore_propagatesACursorOutsideTheSession(t *testing.T) {
	// Given a repository that does not know the cursor in this session
	messages := studymocks.NewMockMessageRepository(t)
	cursor, err := domainstudy.NewMessageCursor("foreign")
	require.NoError(t, err)
	messages.EXPECT().
		ListPage(context.Background(), "session-1", &cursor, 11).
		Return(nil, domainstudy.ErrMessageNotInSession).
		Once()
	service := newHistoryService(t, messages, knowledgemocks.NewMockMessageSourceRepository(t))

	// When asking for a page with that cursor
	page, err := service.ListMessagesBefore(context.Background(), "session-1", "foreign", 10)

	// Then it fails and returns no data (sources are never loaded)
	require.ErrorIs(t, err, domainstudy.ErrMessageNotInSession)
	require.Empty(t, page.Messages)
}

func TestListMessagesBefore_propagatesASourcesError(t *testing.T) {
	// Given a sources repository that fails
	messages := studymocks.NewMockMessageRepository(t)
	cursor, err := domainstudy.NewMessageCursor("message-10")
	require.NoError(t, err)
	messages.EXPECT().ListPage(context.Background(), "session-1", &cursor, 11).Return(nil, nil).Once()
	sourcesErr := errors.New("database unavailable")
	sources := knowledgemocks.NewMockMessageSourceRepository(t)
	sources.EXPECT().ListByMessageIDs(context.Background(), []string{}).Return(nil, sourcesErr).Once()
	service := newHistoryService(t, messages, sources)

	// When asking for a page
	_, err = service.ListMessagesBefore(context.Background(), "session-1", "message-10", 10)

	// Then the error propagates
	require.ErrorIs(t, err, sourcesErr)
}

func TestResume_withMoreThanOnePageOfHistory_returnsTheLatestPageAndACursor(t *testing.T) {
	// Given a session with 51 messages (the repository returns limit+1)
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	session := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	expectLatestPage(messages, newestFirstMessages(DefaultMessagePageSize+1))
	ids := make([]string, 0, DefaultMessagePageSize)
	for i := 2; i <= DefaultMessagePageSize+1; i++ {
		ids = append(ids, fmt.Sprintf("message-%d", i))
	}
	sources := knowledgemocks.NewMockMessageSourceRepository(t)
	sources.EXPECT().ListByMessageIDs(context.Background(), ids).Return(map[string][]domainknowledge.Source{}, nil).Once()
	service := NewService(
		sessions, messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t),
		foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), nil, nil, sources, nil, nil,
	)

	// When resuming
	_, page, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the 50 most recent are returned, oldest first, with a cursor to the rest
	require.NoError(t, err)
	require.Len(t, page.Messages, DefaultMessagePageSize)
	require.Equal(t, "message-2", page.Messages[0].ID)
	require.Equal(t, "message-51", page.Messages[DefaultMessagePageSize-1].ID)
	require.True(t, page.HasMore)
	require.Equal(t, "message-2", page.NextCursor)
}

func TestResume_propagatesALatestPageError(t *testing.T) {
	// Given a message repository that fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{ID: "session-1"}, nil).Once()
	listErr := errors.New("database unavailable")
	messages.EXPECT().
		ListPage(context.Background(), "session-1", (*domainstudy.MessageCursor)(nil), DefaultMessagePageSize+1).
		Return(nil, listErr).
		Once()
	service := NewService(
		sessions, messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t),
		foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), nil, nil,
		knowledgemocks.NewMockMessageSourceRepository(t), nil, nil,
	)

	// When resuming
	_, _, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the error propagates
	require.ErrorIs(t, err, listErr)
}
