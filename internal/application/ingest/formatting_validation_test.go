package ingest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectDominantLanguage_portugueseText_returnsPT(t *testing.T) {
	text := "Esta nota explica como funciona o padrão Backend for Frontend, e também discute a redução do blast radius para cada cliente."
	assert.Equal(t, "pt", detectDominantLanguage(text))
}

func TestDetectDominantLanguage_englishText_returnsEN(t *testing.T) {
	text := "This note explains how the Backend for Frontend pattern works, and also discusses reducing the blast radius for each client."
	assert.Equal(t, "en", detectDominantLanguage(text))
}

func TestDetectDominantLanguage_shortText_isInconclusive(t *testing.T) {
	// Given a batch too short to carry a reliable signal
	assert.Empty(t, detectDominantLanguage("LRU remove o mais antigo"))
}

func TestDetectDominantLanguage_numericHeavyText_isInconclusive(t *testing.T) {
	// Given a batch that is mostly numbers, not prose
	assert.Empty(t, detectDominantLanguage("1 2 3 4 5 6 7 8 9 10 11 12"))
}

func TestExtractNumbersPercentsURLsVersions_findsEachKind(t *testing.T) {
	text := "Taxa de acerto de 95.5%, ver https://example.com/docs, versão 4.5.2, limite de 200 itens."
	tokens := extractNumbersPercentsURLsVersions(text)
	assert.Contains(t, tokens, "95.5%")
	assert.Contains(t, tokens, "https://example.com/docs")
	assert.Contains(t, tokens, "4.5.2")
	assert.Contains(t, tokens, "200")
}

func TestExtractCodeBlocks_returnsFencedBlocksVerbatim(t *testing.T) {
	text := "Explicação.\n\n```go\nfunc main() {}\n```\n\nMais texto."
	blocks := extractCodeBlocks(text)
	assert.Equal(t, []string{"```go\nfunc main() {}\n```"}, blocks)
}

func TestSizeRatio_shrunkenText_isBelowOne(t *testing.T) {
	assert.Less(t, sizeRatio("original text here", "short"), 1.0)
}

func TestValidateFormattingFidelity_emptyFormatted_fails(t *testing.T) {
	v := validateFormattingFidelity("original content", "   ", nil)
	assert.False(t, v.Passed)
	assert.Contains(t, v.Reasons[0], "empty")
}

func TestValidateFormattingFidelity_drasticallyShrunkenText_fails(t *testing.T) {
	original := exactlyNChars(1000)
	formatted := exactlyNChars(100)
	v := validateFormattingFidelity(original, formatted, nil)
	assert.False(t, v.Passed)
}

func TestValidateFormattingFidelity_languageChangedFromPortugueseToEnglish_fails(t *testing.T) {
	// Given the real failure mode diagnosed against a BFF-style note:
	// fluent, non-empty output that silently translated the note
	original := "Este padrão reduz o blast radius de cada cliente ao isolar falhas no backend compartilhado, " +
		"evitando que uma falha em um serviço afete todos os consumidores da API."
	formatted := "This pattern reduces the blast radius of each client by isolating failures in the shared backend, " +
		"preventing a failure in one service from affecting every consumer of the API."

	v := validateFormattingFidelity(original, formatted, nil)

	assert.False(t, v.Passed)
	assert.Equal(t, "pt", v.LanguageBefore)
	assert.Equal(t, "en", v.LanguageAfter)
	assert.True(t, strings.Contains(strings.Join(v.Reasons, "; "), "language changed"))
}

func TestValidateFormattingFidelity_sameLanguageRewrite_passes(t *testing.T) {
	original := "Esta nota explica como funciona o padrão Backend for Frontend para reduzir o blast radius."
	formatted := "## Backend for Frontend\n\nEsta nota explica como o padrão Backend for Frontend funciona, " +
		"reduzindo o blast radius de cada cliente ao isolar falhas no backend compartilhado."

	v := validateFormattingFidelity(original, formatted, nil)

	assert.True(t, v.Passed, "reasons: %v", v.Reasons)
}

func TestValidateFormattingFidelity_droppedNumber_fails(t *testing.T) {
	original := "O limite é de 200 caracteres por chunk, cobrindo 95% dos casos."
	formatted := "O limite é definido por chunk, cobrindo a maior parte dos casos."

	v := validateFormattingFidelity(original, formatted, nil)

	assert.False(t, v.Passed)
}

func TestValidateFormattingFidelity_alteredCodeBlock_fails(t *testing.T) {
	original := "Veja o exemplo:\n\n```go\nfunc main() {}\n```\n\nFim."
	formatted := "Veja o exemplo:\n\n```go\nfunc main() { panic(\"changed\") }\n```\n\nFim."

	v := validateFormattingFidelity(original, formatted, nil)

	assert.False(t, v.Passed)
}

func TestValidateFormattingFidelity_translatedHeading_fails(t *testing.T) {
	original := "## Políticas de Evicção\n\nLRU remove o item menos usado recentemente."
	formatted := "## Eviction Policies\n\nLRU removes the least recently used item."

	v := validateFormattingFidelity(original, formatted, []string{"Políticas de Evicção"})

	assert.False(t, v.Passed)
	assert.Contains(t, strings.Join(v.Reasons, "; "), "Políticas de Evicção")
}

func TestValidateFormattingFidelity_headingSurvivesCaseInsensitively_passes(t *testing.T) {
	original := "## Políticas de Evicção\n\nLRU remove o item menos usado recentemente, de forma consistente."
	formatted := "## políticas de evicção\n\nLRU remove o item menos usado recentemente, de forma consistente e clara."

	v := validateFormattingFidelity(original, formatted, []string{"Políticas de Evicção"})

	assert.True(t, v.Passed, "reasons: %v", v.Reasons)
}
