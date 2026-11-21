package study

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

const validStateJSON = `{"concept":"Race Condition","status":"PARTIALLY_UNDERSTOOD","strategy":"CORRECTIVE_FEEDBACK","userEvidence":true,"assisted":true}`

func stateBlock(body string) string {
	return tutorStateOpenTag + "\n" + body + "\n" + tutorStateCloseTag
}

func TestParseTutorState_validBlockAtTheEnd_returnsTheReportAndCleanText(t *testing.T) {
	// Given a reply that ends with a valid block
	content := "Good, that is a race condition.\n\n" + stateBlock(validStateJSON) + "\n"

	// When parsing it
	cleaned, report, err := parseTutorState(content)

	// Then the text comes back without the block and the report carries every field
	require.NoError(t, err)
	assert.Equal(t, "Good, that is a race condition.", cleaned)
	assert.Equal(t, &domainstudy.StateReport{
		Concept:      "Race Condition",
		Status:       domainstudy.StatusPartiallyUnderstood,
		Strategy:     domainstudy.StrategyCorrectiveFeedback,
		UserEvidence: true,
		Assisted:     true,
	}, report)
}

func TestParseTutorState_withoutABlock_returnsTheContentUntouched(t *testing.T) {
	// Given a reply with no block, and trailing whitespace that must survive
	content := "Plain reply.  \n"

	// When parsing it
	cleaned, report, err := parseTutorState(content)

	// Then nothing changes and there is nothing to log
	require.NoError(t, err)
	assert.Nil(t, report)
	assert.Equal(t, content, cleaned)
}

func TestParseTutorState_blockOnlyReply_cleansToEmpty(t *testing.T) {
	// Given a reply made only of a valid block
	// When parsing it
	cleaned, report, err := parseTutorState(stateBlock(validStateJSON))

	// Then the text is empty and the report is kept
	require.NoError(t, err)
	assert.Equal(t, "", cleaned)
	assert.NotNil(t, report)
}

func TestParseTutorState_badMarkers_dropTheReportButStillCleanTheText(t *testing.T) {
	tests := map[string]struct {
		content string
		cleaned string
		wantErr error
	}{
		"malformed json": {
			"Reply.\n" + stateBlock(`{"concept":`), "Reply.", errTutorStateMalformed,
		},
		"wrong field type": {
			"Reply.\n" + stateBlock(`{"concept":"A","status":"INTRODUCED","userEvidence":"yes"}`), "Reply.", errTutorStateMalformed,
		},
		"unknown status": {
			"Reply.\n" + stateBlock(`{"concept":"A","status":"MASTERED","strategy":"GUIDED_PRACTICE"}`), "Reply.", errTutorStateMalformed,
		},
		"missing status": {
			"Reply.\n" + stateBlock(`{"concept":"A"}`), "Reply.", errTutorStateMalformed,
		},
		"lower-case status": {
			"Reply.\n" + stateBlock(`{"concept":"A","status":"introduced"}`), "Reply.", errTutorStateMalformed,
		},
		"empty block": {
			"Reply.\n" + tutorStateOpenTag + tutorStateCloseTag, "Reply.", errTutorStateMalformed,
		},
		"blank concept": {
			"Reply.\n" + stateBlock(`{"concept":"  ","status":"INTRODUCED"}`), "Reply.", errTutorStateMalformed,
		},
		"missing concept": {
			"Reply.\n" + stateBlock(`{"status":"INTRODUCED"}`), "Reply.", errTutorStateMalformed,
		},
		"two blocks": {
			"Reply.\n" + stateBlock(validStateJSON) + "\n" + stateBlock(validStateJSON), "Reply.", errTutorStateMultiple,
		},
		"text after the block": {
			"Before.\n" + stateBlock(validStateJSON) + "\nAfter.", "Before.\n\nAfter.", errTutorStateNotAtEnd,
		},
		"unterminated block": {
			"Reply.\n" + tutorStateOpenTag + "\n" + `{"concept":"A","status":"INTR`, "Reply.", errTutorStateUnterminated,
		},
		"unterminated block after a complete one": {
			"Reply.\n" + stateBlock(validStateJSON) + "\n" + tutorStateOpenTag + `{"con`, "Reply.", errTutorStateUnterminated,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Given a reply with a bad marker
			// When parsing it
			cleaned, report, err := parseTutorState(tt.content)

			// Then the report is dropped, the reason is reported and the text is clean
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, report)
			assert.Equal(t, tt.cleaned, cleaned)
		})
	}
}

func TestParseTutorState_whitespaceAfterTheBlock_isStillTheEnd(t *testing.T) {
	// Given a block followed only by blank lines
	// When parsing it
	_, report, err := parseTutorState("Reply.\n" + stateBlock(validStateJSON) + "\n \n\t\n")

	// Then it is accepted
	require.NoError(t, err)
	assert.NotNil(t, report)
}

func TestParseTutorState_strategyIsCaseInsensitive(t *testing.T) {
	// Given a block with a lower-case, padded strategy
	body := `{"concept":"A","status":"INTRODUCED","strategy":" corrective_feedback "}`

	// When parsing it
	_, report, err := parseTutorState("Reply.\n" + stateBlock(body))

	// Then it is stored in upper case, with nothing to log
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, domainstudy.StrategyCorrectiveFeedback, report.Strategy)
}

func TestParseTutorState_omittedOrUnknownStrategy_keepsTheReportButReportsIt(t *testing.T) {
	for name, body := range map[string]string{
		"omitted": `{"concept":"A","status":"DEMONSTRATED","userEvidence":true}`,
		"unknown": `{"concept":"A","status":"DEMONSTRATED","userEvidence":true,"strategy":"HINT"}`,
		"blank":   `{"concept":"A","status":"DEMONSTRATED","userEvidence":true,"strategy":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given a block whose strategy is missing or unknown
			// When parsing it
			cleaned, report, err := parseTutorState("Reply.\n" + stateBlock(body))

			// Then the report is still returned, without a strategy, and the problem is reported
			assert.ErrorIs(t, err, errTutorStateStrategy)
			require.NotNil(t, report)
			assert.Equal(t, domainstudy.TutorStrategy(""), report.Strategy)
			assert.Equal(t, domainstudy.StatusDemonstrated, report.Status)
			assert.True(t, report.UserEvidence)
			assert.Equal(t, "Reply.", cleaned)
		})
	}
}

func TestParseTutorState_ignoresUnknownJSONFields(t *testing.T) {
	// Given a block with an extra field
	body := `{"concept":"A","status":"INTRODUCED","strategy":"GUIDED_PRACTICE","confidence":0.9}`

	// When parsing it
	_, report, err := parseTutorState("Reply.\n" + stateBlock(body))

	// Then it is accepted
	require.NoError(t, err)
	assert.NotNil(t, report)
}

func TestParseTutorState_errorsNeverCarryTheMessageContent(t *testing.T) {
	// Given a bad marker whose body holds recognizable text
	_, _, err := parseTutorState("Reply.\n" + stateBlock(`{"concept":"secret-topic","status":"secret-status"}`))

	// Then the error text can be logged safely
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
}

// filterOutput runs chunks through a fresh filter and returns what the user
// would see, flush included.
func filterOutput(chunks ...string) string {
	var filter tutorStateFilter
	var shown strings.Builder
	for _, chunk := range chunks {
		shown.WriteString(filter.write(chunk))
	}
	shown.WriteString(filter.flush())
	return shown.String()
}

func TestTutorStateFilter_dropsAWholeBlockInOneChunk(t *testing.T) {
	// Given a block in one chunk, after some text
	// When filtering
	shown := filterOutput("Hello.\n\n" + stateBlock(validStateJSON))

	// Then only the text is shown
	assert.Equal(t, "Hello.\n\n", shown)
}

func TestTutorStateFilter_dropsTheBlockWhereverTheChunksSplit(t *testing.T) {
	// Given a reply with a block, and every way of cutting it in two
	reply := "Hello.\n\n" + stateBlock(validStateJSON)
	for cut := 0; cut <= len(reply); cut++ {
		// When it streams in two chunks cut at that point
		shown := filterOutput(reply[:cut], reply[cut:])

		// Then the user never sees any of the block
		assert.Equalf(t, "Hello.\n\n", shown, "cut at %d", cut)
	}
}

func TestTutorStateFilter_dropsTheBlockWhenStreamedOneByteAtATime(t *testing.T) {
	// Given a reply streamed byte by byte, with text after the block
	reply := "Before.\n" + stateBlock(validStateJSON) + "\nAfter."
	chunks := strings.Split(reply, "")

	// When filtering
	shown := filterOutput(chunks...)

	// Then the block is gone and the text around it stays
	assert.Equal(t, "Before.\n\nAfter.", shown)
}

func TestTutorStateFilter_dropsAnUnterminatedBlockAtTheEnd(t *testing.T) {
	// Given a block that never closes
	// When the stream ends
	shown := filterOutput("Hello.\n", tutorStateOpenTag+"\n", `{"concept":"A","sta`)

	// Then the user sees only the text before the block
	assert.Equal(t, "Hello.\n", shown)
}

func TestTutorStateFilter_dropsTwoBlocks(t *testing.T) {
	// Given two blocks
	// When filtering
	shown := filterOutput("A" + stateBlock("{}") + "B" + stateBlock("{}") + "C")

	// Then both are dropped
	assert.Equal(t, "ABC", shown)
}

func TestTutorStateFilter_releasesAFragmentThatNeverBecomesTheTag(t *testing.T) {
	// Given text that ends in what looked like the start of the tag
	// When the stream ends there
	shown := filterOutput("Index arrays with [[")

	// Then it is delivered whole
	assert.Equal(t, "Index arrays with [[", shown)
}

func TestTutorStateFilter_holdsAFragmentUntilItIsKnownNotToBeTheTag(t *testing.T) {
	// Given a chunk ending with a possible tag start
	var filter tutorStateFilter

	// When it is written, and then a chunk that shows it was ordinary text
	held := filter.write("see [[ATH")
	released := filter.write("ENS is a city")

	// Then the fragment was not shown at first, and is shown with the rest
	assert.Equal(t, "see ", held)
	assert.Equal(t, "[[ATHENS is a city", released)
}

func TestTutorStateFilter_passesPlainTextThrough(t *testing.T) {
	// Given text with no block
	// When filtering it in several chunks
	shown := filterOutput("One ", "two ", "three")

	// Then it is unchanged
	assert.Equal(t, "One two three", shown)
}

func TestTutorStateFilter_dropsTheBlockEvenWhenItsBodyHoldsPartOfTheClosingTag(t *testing.T) {
	// Given a block body that contains a lone bracket pair before the real close
	body := `{"concept":"a [[ b","status":"INTRODUCED"}`

	// When filtering
	shown := filterOutput("Hi.", stateBlock(body), " Bye.")

	// Then only the real closing tag ends the block
	assert.Equal(t, "Hi. Bye.", shown)
}

func TestTutorStateFilter_dropsAnEmptyBlock(t *testing.T) {
	// Given a block with nothing between its tags, the closing tag right at the start
	// When filtering
	shown := filterOutput("A", tutorStateOpenTag+tutorStateCloseTag, "B")

	// Then it is dropped and the text around it stays
	assert.Equal(t, "AB", shown)
}

func TestPartialTagSuffixLength(t *testing.T) {
	const tag = "[[ABC]]"
	tests := map[string]struct {
		text string
		want int
	}{
		"empty text":                           {"", 0},
		"no overlap":                           {"hello", 0},
		"one bracket":                          {"hello [", 1},
		"the longest overlap wins":             {"x [[AB", 4},
		"text that is a prefix of the tag":     {"[[A", 3},
		"a proper prefix up to the last char":  {"y [[ABC]", 6},
		"never the whole tag":                  {"y " + tag, 0},
		"the middle of the tag is not a start": {"AB", 0},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Given a text and a tag
			// When measuring the held suffix
			got := partialTagSuffixLength(tt.text, tag)

			// Then it is the longest proper prefix of the tag at the end of the text
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTutorState_assessmentAndDescription_areReadWhenPresent(t *testing.T) {
	// Given a block that carries an assessment and a description
	body := `{"concept":"Mutex","status":"DEMONSTRATED","strategy":"COMPREHENSION_CHECK","userEvidence":true,"assessment":"CORRECT","description":"Explained why the lock is needed"}`

	// When parsing it
	_, report, err := parseTutorState("Good.\n" + stateBlock(body))

	// Then both reach the report
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, domainstudy.AssessmentCorrect, report.Assessment)
	assert.Equal(t, "Explained why the lock is needed", report.Description)
}

func TestParseTutorState_assessmentIsCaseInsensitive(t *testing.T) {
	// Given an assessment in lower case
	body := `{"concept":"Mutex","status":"DEMONSTRATED","strategy":"COMPREHENSION_CHECK","userEvidence":true,"assessment":" partially_correct "}`

	// When parsing it
	_, report, err := parseTutorState(stateBlock(body))

	// Then it is read as the upper-case value
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, domainstudy.AssessmentPartiallyCorrect, report.Assessment)
}

func TestParseTutorState_missingOrUnknownAssessment_keepsTheReportValidWithoutOne(t *testing.T) {
	for name, field := range map[string]string{
		"missing": ``,
		"unknown": `,"assessment":"GREAT"`,
		"blank":   `,"assessment":""`,
	} {
		t.Run(name, func(t *testing.T) {
			// Given a block whose assessment is not usable
			body := `{"concept":"Mutex","status":"DEMONSTRATED","strategy":"COMPREHENSION_CHECK","userEvidence":true` + field + `}`

			// When parsing it
			_, report, err := parseTutorState(stateBlock(body))

			// Then the report still applies (the status is unaffected), with no assessment
			require.NoError(t, err)
			require.NotNil(t, report)
			assert.Equal(t, domainstudy.StatusDemonstrated, report.Status)
			assert.Empty(t, report.Assessment)
		})
	}
}
