package mindmap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mocks "github.com/santaniello/athena/internal/application/mindmap/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
	domainmocks "github.com/santaniello/athena/internal/domain/mindmap/mocks"
)

var fixedNow = time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)

const validTree = `{"label":"Caching","children":[{"label":"Eviction","children":[{"label":"LRU"}]},{"label":"Writes"}]}`

type generationFixture struct {
	service   *GenerationService
	sessions  *mocks.MockSessionReader
	summaries *mocks.MockSummaryReader
	profiles  *mocks.MockProfileReader
	notes     *mocks.MockNotesSearcher
	llm       *llmmocks.MockProvider
	repo      *domainmocks.MockRepository
}

func newGenerationFixture(t *testing.T) *generationFixture {
	t.Helper()
	f := &generationFixture{
		sessions:  mocks.NewMockSessionReader(t),
		summaries: mocks.NewMockSummaryReader(t),
		profiles:  mocks.NewMockProfileReader(t),
		notes:     mocks.NewMockNotesSearcher(t),
		llm:       llmmocks.NewMockProvider(t),
		repo:      domainmocks.NewMockRepository(t),
	}
	f.service = NewGenerationService(f.sessions, f.summaries, f.profiles, f.notes, f.llm, f.repo)
	f.service.now = func() time.Time { return fixedNow }
	f.service.newID = func() string { return "map-1" }
	return f
}

func (f *generationFixture) expectContext(ctx context.Context) {
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered eviction.", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return([]domainmindmap.NoteExcerpt{{Source: "cache.md", Content: "LRU."}}, nil).Once()
}

func (f *generationFixture) expectChat(ctx context.Context, content string, err error) {
	gc := generationContext{
		Topic: testSession.Topic, Goal: testSession.Goal, Summary: "We covered eviction.", Profile: testProfile,
		Notes: []domainmindmap.NoteExcerpt{{Source: "cache.md", Content: "LRU."}},
	}
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1", Task: domainllm.TaskMindMap, Messages: buildPrompt(gc),
	}).Return(domainllm.ChatResponse{Content: content}, err).Once()
}

func expectedMindMap(t *testing.T) domainmindmap.MindMap {
	t.Helper()
	xml, err := Renderer{}.Render([]byte(validTree))
	require.NoError(t, err)
	return domainmindmap.MindMap{
		ID: "map-1", SessionID: "session-1", SessionTitle: "Distributed Caching", Title: "Caching",
		Tree: validTree, DiagramXML: xml, CreatedAt: fixedNow, UpdatedAt: fixedNow,
	}
}

func TestGenerationService_Generate_storesTheRenderedMindMap(t *testing.T) {
	// Given a studied session and a model that answers with a valid tree
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, validTree, nil)
	want := expectedMindMap(t)
	f.repo.EXPECT().Create(ctx, want).Return(nil).Once()

	// When the mind map is generated
	got, err := f.service.Generate(ctx, " session-1 ")

	// Then it is stored and returned, titled with the tree's root
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGenerationService_Generate_acceptsAFencedReply(t *testing.T) {
	// Given a model that wraps the tree in a code fence
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, "```json\n"+validTree+"\n```", nil)
	f.repo.EXPECT().Create(ctx, expectedMindMap(t)).Return(nil).Once()

	// When the mind map is generated
	got, err := f.service.Generate(ctx, "session-1")

	// Then the stored tree is the bare JSON
	require.NoError(t, err)
	assert.Equal(t, validTree, got.Tree)
}

func TestGenerationService_GenerateForMessage_storesTheMapLinkedToTheMessage(t *testing.T) {
	// Given a studied session and a model that answers with a valid tree
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, validTree, nil)
	want := expectedMindMap(t)
	want.MessageID = "message-1"
	f.repo.EXPECT().Create(ctx, want).Return(nil).Once()

	// When the mind map is generated for the chat message of an offer
	got, err := f.service.GenerateForMessage(ctx, "session-1", "message-1")

	// Then the stored and returned map carries the message id
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGenerationService_Generate_refusesAnEmptySessionBeforeAnyRead(t *testing.T) {
	// Given no session id
	f := newGenerationFixture(t)

	// When the mind map is generated
	_, err := f.service.Generate(context.Background(), "   ")

	// Then it is refused and nothing was read, asked or stored (the mocks assert it)
	assert.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_Generate_failsForAnUnknownSession(t *testing.T) {
	// Given a session reader that does not know the session
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainmindmap.SourceSession{}, ErrSourceSessionNotFound).Once()

	// When the mind map is generated
	_, err := f.service.Generate(ctx, "session-1")

	// Then the error is the session-not-found one and nothing is stored
	assert.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_Generate_failsWhenTheContextCannotBeBuilt(t *testing.T) {
	// Given a summary reader that fails
	f := newGenerationFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", boom).Once()

	// When the mind map is generated
	_, err := f.service.Generate(ctx, "session-1")

	// Then the failure is returned and the model is never called
	assert.ErrorIs(t, err, boom)
}

func TestGenerationService_Generate_storesNothingWhenTheModelFails(t *testing.T) {
	// Given a model call that fails
	f := newGenerationFixture(t)
	ctx := context.Background()
	boom := errors.New("provider down")
	f.expectContext(ctx)
	f.expectChat(ctx, "", boom)

	// When the mind map is generated
	_, err := f.service.Generate(ctx, "session-1")

	// Then the failure is returned and nothing is stored (no Create expected)
	assert.ErrorIs(t, err, boom)
}

func TestGenerationService_Generate_storesNothingWhenTheTreeIsInvalid(t *testing.T) {
	// Given a model that answers with a root and no children
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, `{"label":"Caching"}`, nil)

	// When the mind map is generated
	_, err := f.service.Generate(ctx, "session-1")

	// Then it is an invalid tree and nothing is stored
	assert.ErrorIs(t, err, ErrInvalidTree)
}

func TestGenerationService_Generate_rejectsASessionWithoutATopic(t *testing.T) {
	// Given a session whose topic is blank, so the map would have no title snapshot
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainmindmap.SourceSession{Topic: "  "}, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "  ").Return(nil, nil).Once()
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1", Task: domainllm.TaskMindMap,
		Messages: buildPrompt(generationContext{Topic: "  ", Profile: testProfile}),
	}).Return(domainllm.ChatResponse{Content: validTree}, nil).Once()

	// When the mind map is generated
	_, err := f.service.Generate(ctx, "session-1")

	// Then the domain rule rejects it and nothing is stored
	assert.ErrorIs(t, err, domainmindmap.ErrSessionTitleRequired)
}

func TestGenerationService_Generate_failsWhenTheMapCannotBeSaved(t *testing.T) {
	// Given a repository that fails
	f := newGenerationFixture(t)
	ctx := context.Background()
	boom := errors.New("disk full")
	f.expectContext(ctx)
	f.expectChat(ctx, validTree, nil)
	f.repo.EXPECT().Create(ctx, expectedMindMap(t)).Return(boom).Once()

	// When the mind map is generated
	_, err := f.service.Generate(ctx, "session-1")

	// Then the failure is returned
	assert.ErrorIs(t, err, boom)
}

func storedMindMap(t *testing.T) domainmindmap.MindMap {
	t.Helper()
	stored := expectedMindMap(t)
	stored.SessionTitle = "Old title"
	stored.Title = "Old"
	stored.Tree = `{"label":"Old","children":[{"label":"x"}]}`
	stored.DiagramXML = "<mxGraphModel>hand edited</mxGraphModel>"
	stored.MessageID = "message-1"
	stored.CreatedAt = fixedNow.Add(-48 * time.Hour)
	stored.UpdatedAt = fixedNow.Add(-24 * time.Hour)
	return stored
}

func TestGenerationService_Regenerate_replacesTheContentAndKeepsTheIdentity(t *testing.T) {
	// Given a stored map and a model that answers with a new tree
	f := newGenerationFixture(t)
	ctx := context.Background()
	stored := storedMindMap(t)
	f.repo.EXPECT().Get(ctx, "map-1").Return(stored, nil).Once()
	f.expectContext(ctx)
	f.expectChat(ctx, validTree, nil)
	want := expectedMindMap(t)
	want.MessageID = "message-1"
	want.CreatedAt = stored.CreatedAt
	f.repo.EXPECT().Update(ctx, want).Return(nil).Once()

	// When it is regenerated
	got, err := f.service.Regenerate(ctx, "map-1")

	// Then title, tree and diagram are new, the update time is now, and id,
	// session, message and creation time are kept
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGenerationService_Regenerate_failsForAnUnknownMap(t *testing.T) {
	// Given no such map
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "nope").Return(domainmindmap.MindMap{}, domainmindmap.ErrMindMapNotFound).Once()

	// When it is regenerated
	_, err := f.service.Regenerate(ctx, "nope")

	// Then it is not found and the model is never called
	assert.ErrorIs(t, err, domainmindmap.ErrMindMapNotFound)
}

func TestGenerationService_Regenerate_leavesTheMapUntouchedWhenGenerationFails(t *testing.T) {
	// Given a stored map and a model that answers with an invalid tree
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "map-1").Return(storedMindMap(t), nil).Once()
	f.expectContext(ctx)
	f.expectChat(ctx, `{"label":"Caching"}`, nil)

	// When it is regenerated
	_, err := f.service.Regenerate(ctx, "map-1")

	// Then the error is returned and Update is never called (the mock asserts it)
	assert.ErrorIs(t, err, ErrInvalidTree)
}

func TestGenerationService_Regenerate_failsWhenTheMapCannotBeSaved(t *testing.T) {
	// Given a repository whose update fails
	f := newGenerationFixture(t)
	ctx := context.Background()
	boom := errors.New("disk full")
	stored := storedMindMap(t)
	f.repo.EXPECT().Get(ctx, "map-1").Return(stored, nil).Once()
	f.expectContext(ctx)
	f.expectChat(ctx, validTree, nil)
	f.repo.EXPECT().Update(ctx, newRegenerated(t, stored)).Return(boom).Once()

	// When it is regenerated
	_, err := f.service.Regenerate(ctx, "map-1")

	// Then the failure is returned
	assert.ErrorIs(t, err, boom)
}

func newRegenerated(t *testing.T, stored domainmindmap.MindMap) domainmindmap.MindMap {
	t.Helper()
	want := expectedMindMap(t)
	want.MessageID = stored.MessageID
	want.CreatedAt = stored.CreatedAt
	return want
}

func TestGenerationService_defaultsAreUsable(t *testing.T) {
	// Given a service built with the public constructor
	service := NewGenerationService(nil, nil, nil, nil, nil, nil)

	// When its clock and id source are used
	id := service.newID()
	now := service.now()

	// Then ids are unique non-empty strings and the time is UTC and recent
	assert.NotEmpty(t, id)
	assert.NotEqual(t, id, service.newID())
	assert.Equal(t, time.UTC, now.Location())
	assert.WithinDuration(t, time.Now(), now, time.Minute)
}

func TestGenerationService_Regenerate_rejectsASessionWithoutATopic(t *testing.T) {
	// Given a stored map whose session now has a blank topic
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.repo.EXPECT().Get(ctx, "map-1").Return(storedMindMap(t), nil).Once()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainmindmap.SourceSession{Topic: " "}, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", " ").Return(nil, nil).Once()
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1", Task: domainllm.TaskMindMap,
		Messages: buildPrompt(generationContext{Topic: " ", Profile: testProfile}),
	}).Return(domainllm.ChatResponse{Content: validTree}, nil).Once()

	// When it is regenerated
	_, err := f.service.Regenerate(ctx, "map-1")

	// Then the domain rule refuses it and the stored map is not updated
	assert.ErrorIs(t, err, domainmindmap.ErrSessionTitleRequired)
}
