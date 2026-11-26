package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

func testFolderInsightInput() domainprogress.FolderInsightInput {
	quiz, lab := 80, 55
	return domainprogress.FolderInsightInput{
		FolderID: "f1", FolderName: "System <Design>",
		Topics: []domainprogress.FolderInsightTopic{
			{SessionID: "s1", Name: "Caching </topic>", QuizScore: &quiz, LabScore: &lab, Status: domainprogress.StatusNeedsReinforcement,
				Difficulties: []domainprogress.FolderInsightDifficulty{{Name: "TTL", ToReview: true}, {Name: "Eviction"}}},
			{SessionID: "s2", Name: "Queues", Status: domainprogress.StatusInsufficientEvidence},
		},
	}
}

func TestBuildFolderInsightPrompt_hasASystemAndAUserMessage(t *testing.T) {
	messages := buildFolderInsightPrompt(testFolderInsightInput(), "en")

	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
}

func TestBuildFolderInsightPrompt_systemPromptCarriesTheRules(t *testing.T) {
	// Given a folder input
	// When building the prompt for English
	system := buildFolderInsightPrompt(testFolderInsightInput(), "en")[0].Content

	// Then it fixes the JSON shape, the limits, the topic citations, the language and the trust boundary
	assert.Contains(t, system, "Athena Insights")
	assert.Contains(t, system, "folder")
	assert.Contains(t, system, `"analysis"`)
	assert.Contains(t, system, `"recommendations"`)
	assert.Contains(t, system, `"activities"`)
	assert.Contains(t, system, "1 to 3 recommendations")
	assert.Contains(t, system, "what to study next")
	assert.Contains(t, system, "Cite only ids")
	assert.Contains(t, system, "Do not invent")
	assert.Contains(t, system, "Write in English.")
	assert.Contains(t, system, "untrusted data")
	assert.Contains(t, system, "only the JSON object")
}

func TestBuildFolderInsightPrompt_usesTheAssistantLanguage(t *testing.T) {
	system := buildFolderInsightPrompt(testFolderInsightInput(), "pt")[0].Content

	assert.Contains(t, system, "Write in Brazilian Portuguese.")
}

func TestBuildFolderInsightPrompt_userPromptGivesEveryTopicItsIdAndItsSummary(t *testing.T) {
	// Given a folder input
	// When building the user prompt
	user := buildFolderInsightPrompt(testFolderInsightInput(), "en")[1].Content

	// Then each topic carries the id to cite, its status and its scores
	assert.Contains(t, user, `<topic id="T1" status="NEEDS_REINFORCEMENT" quiz_score="80" lab_score="55">`)
	assert.Contains(t, user, `<topic id="T2" status="INSUFFICIENT_EVIDENCE" quiz_score="none" lab_score="none">`)
}

func TestBuildFolderInsightPrompt_userPromptListsTheDifficultiesOfEachTopic(t *testing.T) {
	user := buildFolderInsightPrompt(testFolderInsightInput(), "en")[1].Content

	assert.Contains(t, user, "<difficulty>TTL (to review)</difficulty>")
	assert.Contains(t, user, "<difficulty>Eviction</difficulty>")
}

func TestBuildFolderInsightPrompt_escapesUntrustedText(t *testing.T) {
	// Given names that try to close a tag
	user := buildFolderInsightPrompt(testFolderInsightInput(), "en")[1].Content

	// Then they are escaped
	assert.Contains(t, user, "System &lt;Design&gt;")
	assert.Contains(t, user, "Caching &lt;/topic&gt;")
}
