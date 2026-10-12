package ingest

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	"github.com/santaniello/athena/internal/infrastructure/openrouter"
)

// noOpUsageRecorder discards usage entries — acceptable for this opt-in,
// manually-run diagnostic suite (never part of `go test ./...`), which has
// no database to record real cost/token usage into.
type noOpUsageRecorder struct{}

func (noOpUsageRecorder) Record(context.Context, domainllm.UsageEntry) error { return nil }

// requireRealAPITests skips the test unless ATHENA_REALAPI_TESTS is set,
// mirroring the `-short` skip convention already used elsewhere in this
// repo (see internal/infrastructure/vectorstore/store_test.go) for a test
// that costs real money/time and is never deterministic enough for CI.
func requireRealAPITests(t *testing.T) {
	t.Helper()
	if os.Getenv("ATHENA_REALAPI_TESTS") == "" {
		t.Skip("set ATHENA_REALAPI_TESTS=1 (and OPENROUTER_API_KEY) to run against the real OpenRouter API")
	}
	if os.Getenv("OPENROUTER_API_KEY") == "" {
		t.Fatal("ATHENA_REALAPI_TESTS is set but OPENROUTER_API_KEY is not")
	}
}

func realAPIService(t *testing.T) *Service {
	t.Helper()
	client := openrouter.NewClient("", os.Getenv("OPENROUTER_API_KEY"), noOpUsageRecorder{})
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).
		Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	return &Service{index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: client}
}

// TestRealAPI_NamedList_LRUFIFOCache replicates the exact case that
// motivated the formatting prompt's "restate shared context per item"
// rule (see TestNoteFormattingPrompts_coverBundledListDefinitions) and the
// structural-validation follow-up (03-01-chunking-by-paragraph.md,
// 20-01's decision 6): a tight, no-blank-line list of named cache-eviction
// policies. Run manually against the real API to check whether formatting
// + structural validation's one corrective retry produces 4 independent
// chunks, 2 (the documented, accepted residual), or still 1 (regression).
func TestRealAPI_NamedList_LRUFIFOCache(t *testing.T) {
	requireRealAPITests(t)

	original := "### Políticas de Evicção\n" +
		"- LRU: remove o item menos usado recentemente.\n" +
		"- LFU: remove o item menos usado com menor frequência.\n" +
		"- FIFO: remove o item que entrou primeiro no cache.\n" +
		"- RR: remove um item aleatório do cache.\n"
	root := fstest.MapFS{"cache.md": {Data: []byte(original)}}
	svc := realAPIService(t)

	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "cache.md", nil)
	require.NoError(t, err)

	t.Logf("formattingFailed=%v chunks=%d structureWarnings=%+v formatted=%q",
		preview.FormattingFailed, len(preview.Chunks), preview.StructureWarnings, preview.FormattedContent)

	// Never silently worse than today's documented residual: either it
	// resolved cleanly (no warning) or the still-collapsed case is
	// reported, visibly, as a StructureWarning — never as plain success
	// with the list still bundled and unflagged.
	if len(preview.Chunks) <= 1 {
		assert.NotEmpty(t, preview.StructureWarnings, "a single-chunk result must be flagged, not silent")
	}
}

// TestRealAPI_PortugueseNote_IsNotTranslatedToEnglish replicates the BFF
// note translation failure mode diagnosed against the real evaluation
// corpus (know-improvements.md, local artifact, not committed): a fluent,
// non-empty rewrite that silently translated a Portuguese note into
// English. validateFormattingFidelity should catch this and fall back.
func TestRealAPI_PortugueseNote_IsNotTranslatedToEnglish(t *testing.T) {
	requireRealAPITests(t)

	original := "## Backend for Frontend\n\n" +
		"O padrão Backend for Frontend (BFF) reduz o blast radius de cada cliente ao isolar " +
		"falhas no backend compartilhado, evitando que uma falha em um serviço afete todos os " +
		"consumidores da API de forma inesperada. Cada cliente (web, mobile, parceiros) ganha " +
		"seu próprio BFF, adaptado ao formato de dados que esse cliente específico precisa."
	root := fstest.MapFS{"bff.md": {Data: []byte(original)}}
	svc := realAPIService(t)

	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "bff.md", nil)
	require.NoError(t, err)

	t.Logf("formattingFailed=%v failedSections=%+v formatted=%q",
		preview.FormattingFailed, preview.FailedSections, preview.FormattedContent)

	assert.Equal(t, "pt", detectDominantLanguage(preview.FormattedContent),
		"the formatted text must stay Portuguese — a translation must be caught by fidelity validation and fall back")
}
