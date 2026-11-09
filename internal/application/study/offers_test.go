package study

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// replyOutcome is what a streamed reply left behind.
type replyOutcome struct {
	received []string
	stored   domainstudy.Message
}

// streamReply sends a message whose reply streams the given chunks, with the
// offer gate in place (nil for none), and reports what the user saw and what
// was stored.
func streamReply(t *testing.T, chunks []string, gate OfferGate) (replyOutcome, error) {
	t.Helper()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("[]string")).
		Return(domainknowledge.RetrievalResult{}, nil).Once()
	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool { return m.Role == domainstudy.RoleUser })).Return(nil).Once()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			for _, chunk := range chunks {
				require.NoError(t, handler(chunk))
			}
		}).
		Return(domainllm.StreamResponse{}, nil).Once()
	var outcome replyOutcome
	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool { return m.Role == domainstudy.RoleAssistant })).
		Run(func(_ context.Context, m domainstudy.Message) { outcome.stored = m }).Return(nil).Once()
	messageSources := mockMessageSourceSave(t, nil)

	options := []Option{}
	if gate != nil {
		options = append(options, WithOfferGate(gate))
	}
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil, messageSources, nil, nil, options...)

	err := service.SendMessage(context.Background(), "session-1", "Caching", "give me an exercise", domainknowledge.SourceModeNotes,
		noopSourcesHandler, func(chunk string) error { outcome.received = append(outcome.received, chunk); return nil }, nil, nil)
	return outcome, err
}

func joined(chunks []string) string {
	text := ""
	for _, chunk := range chunks {
		text += chunk
	}
	return text
}

func TestSendMessage_returnsErrOfferPending_beforeStoringOrCallingAnything(t *testing.T) {
	// Given a session with a pending offer; no other mock has expectations
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(true, nil).Once()
	service := NewService(studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), nil, nil, nil, nil, nil,
		WithOfferGate(gate))

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Caching", "hello", domainknowledge.SourceModeNotes, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then ErrOfferPending is returned and nothing was stored or sent
	require.ErrorIs(t, err, ErrOfferPending)
}

func TestSendMessage_failsWhenThePendingOfferCannotBeChecked(t *testing.T) {
	// Given an offer check that fails
	boom := errors.New("db down")
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(false, boom).Once()
	service := NewService(studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), nil, nil, nil, nil, nil,
		WithOfferGate(gate))

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Caching", "hello", domainknowledge.SourceModeNotes, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then the error comes back and nothing was stored or sent
	require.ErrorIs(t, err, boom)
}

func TestSendMessage_createsTheOfferUnderTheStoredReply_andHidesTheMarker(t *testing.T) {
	// Given a reply that ends with the marker, split mid-marker
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(false, nil).Once()
	var offerMessageID string
	gate.EXPECT().CreateOffer(context.Background(), "session-1", mock.AnythingOfType("string")).
		Run(func(_ context.Context, _, messageID string) { offerMessageID = messageID }).Return(nil).Once()

	// When it streams
	outcome, err := streamReply(t, []string{"Sure, let's practice.\n", "[[ATHENA_EXER", "CISE_OFFER]]"}, gate)

	// Then the user never saw the marker, the stored text is clean and the offer hangs under that message
	require.NoError(t, err)
	require.Equal(t, "Sure, let's practice.\n", joined(outcome.received))
	require.Equal(t, "Sure, let's practice.", outcome.stored.Content)
	require.Equal(t, outcome.stored.ID, offerMessageID)
}

func TestSendMessage_createsNoOffer_whenTheReplyHasNoMarker(t *testing.T) {
	// Given a normal reply; CreateOffer has no expectation
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(false, nil).Once()

	// When it streams
	outcome, err := streamReply(t, []string{"CAP says ", "pick two."}, gate)

	// Then the reply is untouched
	require.NoError(t, err)
	require.Equal(t, "CAP says pick two.", joined(outcome.received))
	require.Equal(t, "CAP says pick two.", outcome.stored.Content)
}

func TestSendMessage_deliversAHeldPartialMarker_whenTheReplyEndsWithoutCompletingIt(t *testing.T) {
	// Given a reply that ends in what looked like the marker's start
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(false, nil).Once()

	// When it streams
	outcome, err := streamReply(t, []string{"Index arrays with [["}, gate)

	// Then the text reaches the user whole and is stored whole
	require.NoError(t, err)
	require.Equal(t, "Index arrays with [[", joined(outcome.received))
	require.Equal(t, "Index arrays with [[", outcome.stored.Content)
}

func TestSendMessage_doesNotFailTheReply_whenTheOfferCannotBeCreated(t *testing.T) {
	// Given an offer creation that fails
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(false, nil).Once()
	gate.EXPECT().CreateOffer(context.Background(), "session-1", mock.AnythingOfType("string")).Return(errors.New("db down")).Once()

	// When a reply with the marker streams
	outcome, err := streamReply(t, []string{"Sure.", exerciseOfferMarker}, gate)

	// Then the reply still succeeds, clean
	require.NoError(t, err)
	require.Equal(t, "Sure.", outcome.stored.Content)
}

func TestSendMessage_stripsTheMarkerButCreatesNoOffer_whenNoGateIsWired(t *testing.T) {
	// Given a service without an offer gate
	// When a reply with the marker streams
	outcome, err := streamReply(t, []string{"Sure.", exerciseOfferMarker}, nil)

	// Then the marker is still hidden and stored text is clean
	require.NoError(t, err)
	require.Equal(t, "Sure.", joined(outcome.received))
	require.Equal(t, "Sure.", outcome.stored.Content)
}

func TestRequestOpeningTurn_stripsTheMarkerAndCreatesNoOffer(t *testing.T) {
	// Given an opening reply that carries the marker although the user asked nothing;
	// the gate has no expectations, so any call fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	tx := applicationstudymocks.NewMockTransactor(t)
	runWithinTx(tx)
	gate := applicationstudymocks.NewMockOfferGate(t)
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(normalSession("session-1"), nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("Hi! "+exerciseOfferMarker))
		}).Return(domainllm.StreamResponse{}, nil).Once()
	var stored domainstudy.Message
	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool { return m.Role == domainstudy.RoleAssistant })).
		Run(func(_ context.Context, m domainstudy.Message) { stored = m }).Return(nil).Once()
	sessions.EXPECT().UpdateContext(context.Background(), "session-1", mock.Anything).Return(nil).Once()
	messageSources := mockMessageSourceSave(t, nil)
	service := NewService(sessions, messages, llm, profiles, foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), tx, nil, messageSources, nil, nil,
		WithOfferGate(gate))
	var received []string

	// When requesting the opening turn
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Caching",
		func(chunk string) error { received = append(received, chunk); return nil }, nil, nil)

	// Then the marker is hidden and stored clean, with no offer
	require.NoError(t, err)
	require.Equal(t, "Hi! ", joined(received))
	require.Equal(t, "Hi!", stored.Content)
}
