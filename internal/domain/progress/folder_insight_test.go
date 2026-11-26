package progress

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func folderInsightInput() FolderInsightInput {
	score := 80
	return FolderInsightInput{
		FolderID: "f1", FolderName: "System Design",
		Topics: []FolderInsightTopic{
			{SessionID: "s1", Name: "Cache", QuizScore: &score, Status: StatusStable, Fingerprint: "fp1"},
			{SessionID: "s2", Name: "Queues", Status: StatusNeedsReinforcement, Fingerprint: "fp2",
				Difficulties: []FolderInsightDifficulty{{Name: "Ordering", ToReview: true}}},
		},
	}
}

func TestFolderInsightTopicRef_numbersTheTopicsFromOne(t *testing.T) {
	// Given a folder input
	in := folderInsightInput()

	// When the refs are asked
	// Then the topics are T1, T2 in order
	assert.Equal(t, "T1", in.TopicRef(0))
	assert.Equal(t, "T2", in.TopicRef(1))
}

func TestFolderInsightInput_hasEvidenceWhenSomeTopicIsOutsideInsufficientEvidence(t *testing.T) {
	// Given a folder whose topics are all in Insufficient evidence
	in := FolderInsightInput{Topics: []FolderInsightTopic{{Status: StatusInsufficientEvidence}}}
	assert.False(t, in.HasEvidence())

	// When one topic gets evidence
	in.Topics = append(in.Topics, FolderInsightTopic{Status: StatusImproving})

	// Then the folder has evidence
	assert.True(t, in.HasEvidence())
}

func TestFolderInsightInput_anEmptyFolderHasNoEvidence(t *testing.T) {
	assert.False(t, FolderInsightInput{}.HasEvidence())
}

func TestFolderFingerprint_isStableForTheSameInputAndLanguage(t *testing.T) {
	assert.Equal(t, FolderFingerprint(folderInsightInput(), "pt"), FolderFingerprint(folderInsightInput(), "pt"))
}

func TestFolderFingerprint_changesWithTheLanguage(t *testing.T) {
	assert.NotEqual(t, FolderFingerprint(folderInsightInput(), "pt"), FolderFingerprint(folderInsightInput(), "en"))
}

func TestFolderFingerprint_changesWhenATopicHasNewActivity(t *testing.T) {
	// Given a topic whose own fingerprint moved
	changed := folderInsightInput()
	changed.Topics[0].Fingerprint = "fp1-new"

	// Then the folder fingerprint moves
	assert.NotEqual(t, FolderFingerprint(folderInsightInput(), "pt"), FolderFingerprint(changed, "pt"))
}

func TestFolderFingerprint_changesWhenASessionLeavesOrEntersTheFolder(t *testing.T) {
	// Given the folder without its second topic
	without := folderInsightInput()
	without.Topics = without.Topics[:1]

	// Then it differs from the folder with it
	assert.NotEqual(t, FolderFingerprint(folderInsightInput(), "pt"), FolderFingerprint(without, "pt"))
}

func TestFolderFingerprint_changesWhenASessionIsSwappedForAnotherWithTheSameFingerprint(t *testing.T) {
	// Given a topic replaced by another session with the same own fingerprint
	swapped := folderInsightInput()
	swapped.Topics[1].SessionID = "s3"

	// Then the folder fingerprint moves
	assert.NotEqual(t, FolderFingerprint(folderInsightInput(), "pt"), FolderFingerprint(swapped, "pt"))
}

func TestFolderFingerprint_doesNotDependOnTheOrderOfTheTopics(t *testing.T) {
	// Given the same topics in another order
	reversed := folderInsightInput()
	reversed.Topics[0], reversed.Topics[1] = reversed.Topics[1], reversed.Topics[0]

	// Then the fingerprint is the same
	assert.Equal(t, FolderFingerprint(folderInsightInput(), "pt"), FolderFingerprint(reversed, "pt"))
}

func TestFolderFingerprint_doesNotConfuseAnIdWithTheFingerprintNextToIt(t *testing.T) {
	// Given two folders whose id and fingerprint concatenate the same
	a := FolderInsightInput{Topics: []FolderInsightTopic{{SessionID: "s1", Fingerprint: "2x"}}}
	b := FolderInsightInput{Topics: []FolderInsightTopic{{SessionID: "s12", Fingerprint: "x"}}}

	// Then the fingerprints differ
	assert.NotEqual(t, FolderFingerprint(a, "pt"), FolderFingerprint(b, "pt"))
}

func TestParseFolderInsight_buildsStatementsCitingTopics(t *testing.T) {
	// Given a valid answer that cites a topic
	raw := `{"analysis":[{"text":" Cache is solid. ","activities":["T1"]}],"recommendations":[{"text":"Review queues.","activities":["T2"]}]}`

	// When it is parsed
	content, err := ParseFolderInsight(raw, folderInsightInput())

	// Then the citations point to the topic, by session id, labelled with its name
	require.NoError(t, err)
	assert.Equal(t, []Statement{{Text: "Cache is solid.", Citations: []Citation{{Kind: CitationTopic, ID: "s1", Label: "Cache"}}}}, content.Analysis)
	assert.Equal(t, []Statement{{Text: "Review queues.", Citations: []Citation{{Kind: CitationTopic, ID: "s2", Label: "Queues"}}}}, content.Recommendations)
}

func TestParseFolderInsight_dropsCitationsOfUnknownTopics(t *testing.T) {
	// Given an answer that cites a topic that is not in the folder and an activity ref of a topic insight
	raw := `{"analysis":[{"text":"A.","activities":["T9","Q1","T1"]}],"recommendations":[{"text":"B.","activities":[]}]}`

	// When it is parsed
	content, err := ParseFolderInsight(raw, folderInsightInput())

	// Then only the known topic stays
	require.NoError(t, err)
	assert.Equal(t, []Citation{{Kind: CitationTopic, ID: "s1", Label: "Cache"}}, content.Analysis[0].Citations)
}

func TestParseFolderInsight_rejectsAnInvalidAnswer(t *testing.T) {
	_, err := ParseFolderInsight(`{"analysis":[],"recommendations":[]}`, folderInsightInput())
	assert.ErrorIs(t, err, ErrInvalidInsight)
}
