package study

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var stateNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func existingConcept(status ConceptStatus) *ConceptState {
	return &ConceptState{
		SessionID:       "session-1",
		Name:            "race condition",
		Status:          status,
		LastStrategy:    StrategyDirectInstruction,
		AssistanceCount: 2,
		EvidenceCount:   3,
		UpdatedAt:       stateNow.Add(-time.Hour),
	}
}

func report(status ConceptStatus, evidence bool) StateReport {
	return StateReport{
		Concept:      "Race Condition",
		Status:       status,
		Strategy:     StrategyCorrectiveFeedback,
		UserEvidence: evidence,
	}
}

func TestApplyReport_newConceptWithEvidence_keepsReportedStatus(t *testing.T) {
	// Given no row for the concept and a report with the user's evidence
	// When applying a DEMONSTRATED report
	next, changed := ApplyReport(nil, report(StatusDemonstrated, true), stateNow)

	// Then a row is created with the reported status, a normalized name and counters
	assert.True(t, changed)
	assert.Equal(t, StatusDemonstrated, next.Status)
	assert.Equal(t, "race condition", next.Name)
	assert.Equal(t, StrategyCorrectiveFeedback, next.LastStrategy)
	assert.Equal(t, 1, next.EvidenceCount)
	assert.Equal(t, 0, next.AssistanceCount)
	assert.Equal(t, stateNow, next.UpdatedAt)
}

func TestApplyReport_newConceptWithoutEvidence_isCappedAtIntroduced(t *testing.T) {
	for _, reported := range []ConceptStatus{StatusIntroduced, StatusPartiallyUnderstood, StatusDemonstrated, StatusNeedsReview} {
		t.Run(string(reported), func(t *testing.T) {
			// Given no row and a report without the user's evidence
			// When applying it
			next, changed := ApplyReport(nil, report(reported, false), stateNow)

			// Then the new row is created at INTRODUCED at most
			assert.True(t, changed)
			assert.Equal(t, StatusIntroduced, next.Status)
			assert.Equal(t, 0, next.EvidenceCount)
		})
	}
}

func TestApplyReport_existingConceptWithoutEvidence_neverChangesStatus(t *testing.T) {
	for _, current := range []ConceptStatus{StatusIntroduced, StatusPartiallyUnderstood, StatusDemonstrated, StatusNeedsReview} {
		for _, reported := range []ConceptStatus{StatusIntroduced, StatusPartiallyUnderstood, StatusDemonstrated, StatusNeedsReview} {
			t.Run(string(current)+"_"+string(reported), func(t *testing.T) {
				// Given an existing row and a report without evidence
				// When applying it
				next, changed := ApplyReport(existingConcept(current), report(reported, false), stateNow)

				// Then the status stays, so a re-explanation never regresses or improves it
				assert.True(t, changed)
				assert.Equal(t, current, next.Status)
			})
		}
	}
}

func TestApplyReport_existingConceptWithEvidence_appliesReportedStatus(t *testing.T) {
	// Given a DEMONSTRATED row and evidence of a gap
	// When applying a NEEDS_REVIEW report
	next, _ := ApplyReport(existingConcept(StatusDemonstrated), report(StatusNeedsReview, true), stateNow)

	// Then the status moves down
	assert.Equal(t, StatusNeedsReview, next.Status)

	// And so does PARTIALLY_UNDERSTOOD
	next, _ = ApplyReport(existingConcept(StatusDemonstrated), report(StatusPartiallyUnderstood, true), stateNow)
	assert.Equal(t, StatusPartiallyUnderstood, next.Status)

	// And an introduced concept the user then showed moves up
	next, _ = ApplyReport(existingConcept(StatusIntroduced), report(StatusDemonstrated, true), stateNow)
	assert.Equal(t, StatusDemonstrated, next.Status)
}

func TestApplyReport_demonstratedOnNeedsReview_stepsToPartiallyUnderstood(t *testing.T) {
	// Given a NEEDS_REVIEW row
	// When a single DEMONSTRATED report with evidence arrives
	next, _ := ApplyReport(existingConcept(StatusNeedsReview), report(StatusDemonstrated, true), stateNow)

	// Then one lucky answer does not close the difficulty
	assert.Equal(t, StatusPartiallyUnderstood, next.Status)
}

func TestApplyReport_notAssessedReport_changesNothing(t *testing.T) {
	// Given a report whose status is NOT_ASSESSED, with and without a row
	// When applying it
	_, changedNew := ApplyReport(nil, report(StatusNotAssessed, true), stateNow)
	next, changedExisting := ApplyReport(existingConcept(StatusDemonstrated), report(StatusNotAssessed, true), stateNow)

	// Then it is ignored: no row is created and the existing one is not touched
	assert.False(t, changedNew)
	assert.False(t, changedExisting)
	assert.Equal(t, *existingConcept(StatusDemonstrated), next)
}

func TestApplyReport_countersAndStrategy_updateWithOrWithoutEvidence(t *testing.T) {
	// Given an existing row and an assisted report without evidence
	assisted := report(StatusIntroduced, false)
	assisted.Assisted = true

	// When applying it
	next, _ := ApplyReport(existingConcept(StatusPartiallyUnderstood), assisted, stateNow)

	// Then the assistance count and the strategy move, the evidence count does not
	assert.Equal(t, 3, next.AssistanceCount)
	assert.Equal(t, 3, next.EvidenceCount)
	assert.Equal(t, StrategyCorrectiveFeedback, next.LastStrategy)
	assert.Equal(t, stateNow, next.UpdatedAt)
	assert.Equal(t, "session-1", next.SessionID)

	// And evidence without assistance moves the evidence count only
	next, _ = ApplyReport(existingConcept(StatusPartiallyUnderstood), report(StatusDemonstrated, true), stateNow)
	assert.Equal(t, 2, next.AssistanceCount)
	assert.Equal(t, 4, next.EvidenceCount)
}

func TestApplyReport_emptyStrategy_keepsPreviousStrategy(t *testing.T) {
	// Given a report whose strategy was omitted
	noStrategy := report(StatusIntroduced, true)
	noStrategy.Strategy = ""

	// When applying it to an existing row and to a new one
	next, _ := ApplyReport(existingConcept(StatusIntroduced), noStrategy, stateNow)
	created, _ := ApplyReport(nil, noStrategy, stateNow)

	// Then the previous strategy is kept, and a new row has none
	assert.Equal(t, StrategyDirectInstruction, next.LastStrategy)
	assert.Equal(t, TutorStrategy(""), created.LastStrategy)
}

func TestApplyReport_doesNotMutateTheCurrentRow(t *testing.T) {
	// Given an existing row
	current := existingConcept(StatusIntroduced)

	// When applying a report
	ApplyReport(current, report(StatusDemonstrated, true), stateNow)

	// Then the input is untouched
	assert.Equal(t, *existingConcept(StatusIntroduced), *current)
}

func TestNormalizeConceptName(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"trims and lower-cases":       {"  Race Condition ", "race condition"},
		"collapses inner whitespace":  {"Race \t\n  Condition", "race condition"},
		"keeps accents":               {"Condição de Corrida", "condição de corrida"},
		"blank stays blank":           {" \t ", ""},
		"cuts on runes, not on bytes": {strings.Repeat("ç", 100), strings.Repeat("ç", MaxConceptNameRunes)},
		"keeps the limit exactly":     {strings.Repeat("a", MaxConceptNameRunes), strings.Repeat("a", MaxConceptNameRunes)},
		"trims a space left at the cut": {
			strings.Repeat("a", MaxConceptNameRunes-1) + " b",
			strings.Repeat("a", MaxConceptNameRunes-1),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Given a raw concept name
			// When normalizing it
			got := NormalizeConceptName(tt.input)

			// Then it is the identity used for the row
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConceptStatus_IsValid(t *testing.T) {
	// Given the five statuses and two unknown values
	for _, valid := range []ConceptStatus{StatusNotAssessed, StatusIntroduced, StatusPartiallyUnderstood, StatusDemonstrated, StatusNeedsReview} {
		assert.True(t, valid.IsValid(), valid)
	}

	// Then anything else is rejected
	assert.False(t, ConceptStatus("").IsValid())
	assert.False(t, ConceptStatus("MASTERED").IsValid())
}

func TestTutorStrategy_IsValid(t *testing.T) {
	// Given the six strategies of the adaptive tutor and two unknown values
	for _, valid := range []TutorStrategy{
		StrategyDirectInstruction, StrategySocraticQuestioning, StrategyGuidedPractice,
		StrategyComprehensionCheck, StrategyCorrectiveFeedback, StrategyConceptReview,
	} {
		assert.True(t, valid.IsValid(), valid)
	}

	// Then anything else is rejected
	assert.False(t, TutorStrategy("").IsValid())
	assert.False(t, TutorStrategy("HINT").IsValid())
}

func reportWithAssessment(status ConceptStatus, assessment Assessment) StateReport {
	r := report(status, true)
	r.Assessment = assessment
	return r
}

func TestApplyReport_demonstratedWithAnIncorrectAssessment_isStoredAsPartiallyUnderstood(t *testing.T) {
	// Given a DEMONSTRATED report whose own assessment says the user's message was incorrect
	for name, current := range map[string]*ConceptState{
		"new concept":      nil,
		"existing concept": existingConcept(StatusIntroduced),
	} {
		t.Run(name, func(t *testing.T) {
			// When applying it
			next, changed := ApplyReport(current, reportWithAssessment(StatusDemonstrated, AssessmentIncorrect), stateNow)

			// Then the contradiction never reaches the state: the concept is only partially understood
			assert.True(t, changed)
			assert.Equal(t, StatusPartiallyUnderstood, next.Status)
		})
	}
}

func TestApplyReport_coherentAssessments_leaveTheReportedStatusAlone(t *testing.T) {
	cases := map[string]struct {
		status     ConceptStatus
		assessment Assessment
	}{
		"demonstrated and correct":           {StatusDemonstrated, AssessmentCorrect},
		"demonstrated and partially correct": {StatusDemonstrated, AssessmentPartiallyCorrect},
		"demonstrated without assessment":    {StatusDemonstrated, ""},
		"needs review and incorrect":         {StatusNeedsReview, AssessmentIncorrect},
		"partially understood and incorrect": {StatusPartiallyUnderstood, AssessmentIncorrect},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Given a report whose assessment does not contradict its status
			// When applying it to a new concept
			next, _ := ApplyReport(nil, reportWithAssessment(tc.status, tc.assessment), stateNow)

			// Then the reported status is kept
			assert.Equal(t, tc.status, next.Status)
		})
	}
}

func TestApplyReport_demonstratedWithAnIncorrectAssessment_onANeedsReviewConcept_staysNeedsReview(t *testing.T) {
	// Given a concept to review and a DEMONSTRATED report that is contradicted by its own assessment
	// When applying it
	next, _ := ApplyReport(existingConcept(StatusNeedsReview), reportWithAssessment(StatusDemonstrated, AssessmentIncorrect), stateNow)

	// Then it is treated as PARTIALLY_UNDERSTOOD, which is what the user showed: no step up from NEEDS_REVIEW is lost or invented
	assert.Equal(t, StatusPartiallyUnderstood, next.Status)
}

func TestApplyReport_newConcept_keepsTheReportedSpellingAsDisplayName(t *testing.T) {
	// Given a report that spells the concept "Cache-aside"
	r := report(StatusIntroduced, false)
	r.Concept = "  Cache-aside "

	// When applying it to a concept with no row
	next, _ := ApplyReport(nil, r, stateNow)

	// Then the identity is lower case and the display name keeps the spelling
	assert.Equal(t, "cache-aside", next.Name)
	assert.Equal(t, "Cache-aside", next.DisplayName)
}

func TestApplyReport_existingConcept_takesTheLatestSpelling(t *testing.T) {
	// Given a row displayed as "Race condition"
	current := existingConcept(StatusIntroduced)
	current.DisplayName = "Race condition"

	// When a later report spells it "Race Condition"
	next, _ := ApplyReport(current, report(StatusIntroduced, false), stateNow)

	// Then the display name follows the latest report
	assert.Equal(t, "Race Condition", next.DisplayName)
}

func TestApplyReport_blankReportedConcept_keepsTheDisplayName(t *testing.T) {
	// Given a row with a display name and a report whose concept is blank
	current := existingConcept(StatusIntroduced)
	current.DisplayName = "Race condition"
	r := report(StatusIntroduced, false)
	r.Concept = "   "

	// When applying it
	next, _ := ApplyReport(current, r, stateNow)

	// Then the display name is not erased
	assert.Equal(t, "Race condition", next.DisplayName)
}

func TestApplyReport_displayName_collapsesSpacesAndIsCappedLikeTheName(t *testing.T) {
	// Given a spelling with inner whitespace and more runes than the cap allows
	r := report(StatusIntroduced, false)
	r.Concept = "Big   " + strings.Repeat("Ä", MaxConceptNameRunes+10)

	// When applying it to a new concept
	next, _ := ApplyReport(nil, r, stateNow)

	// Then the display name has single spaces and at most MaxConceptNameRunes runes
	assert.True(t, strings.HasPrefix(next.DisplayName, "Big Ä"))
	assert.Len(t, []rune(next.DisplayName), MaxConceptNameRunes)
}
