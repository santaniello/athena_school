package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testPassScore = 80

var t0 = time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

func at(days int) time.Time { return t0.AddDate(0, 0, days) }

func quiz(id string, percents ...int) QuizInput {
	attempts := make([]ScoredAttempt, len(percents))
	for i, percent := range percents {
		attempts[i] = ScoredAttempt{Percent: percent, At: at(i + 1)}
	}
	return QuizInput{QuizID: id, Attempts: attempts}
}

func lab(diagram, understanding int, failed bool, day int) LabAttempt {
	return LabAttempt{DiagramScore: diagram, UnderstandingScore: understanding, Failed: failed, At: at(day)}
}

func ptr(v int) *int { return &v }

func TestSummarizeTopic_QuizScore(t *testing.T) {
	tests := []struct {
		name    string
		quizzes []QuizInput
		want    *int
	}{
		{"no quiz has no score", nil, nil},
		{"a quiz without attempts has no score", []QuizInput{{QuizID: "q1"}}, nil},
		{"a retaken quiz counts its latest attempt", []QuizInput{quiz("q1", 60, 90)}, ptr(90)},
		{"two quizzes are averaged", []QuizInput{quiz("q1", 80), quiz("q2", 50)}, ptr(65)},
		{"a quiz without attempts does not weigh in", []QuizInput{quiz("q1", 80), {QuizID: "q2"}}, ptr(80)},
		{"the mean rounds half up", []QuizInput{quiz("q1", 80), quiz("q2", 51)}, ptr(66)},
		{"the mean rounds down below half", []QuizInput{quiz("q1", 80), quiz("q2", 50), quiz("q3", 50)}, ptr(60)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := SummarizeTopic(TopicInput{Quizzes: tt.quizzes}, testPassScore)

			assert.Equal(t, tt.want, summary.QuizScore)
		})
	}
}

func TestSummarizeTopic_LabScore(t *testing.T) {
	tests := []struct {
		name       string
		challenges []ChallengeInput
		want       *int
	}{
		{"no challenge has no score", nil, nil},
		{"a challenge without attempts has no score", []ChallengeInput{{ChallengeID: "c1"}}, nil},
		{"the lab score is the mean of diagram and defense", []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{lab(85, 40, true, 1)}}}, ptr(63)},
		{"the latest attempt of a challenge counts", []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{lab(40, 40, true, 1), lab(90, 80, false, 2)}}}, ptr(85)},
		{"challenges are averaged", []ChallengeInput{
			{ChallengeID: "c1", Attempts: []LabAttempt{lab(90, 90, false, 1)}},
			{ChallengeID: "c2", Attempts: []LabAttempt{lab(50, 50, true, 1)}},
		}, ptr(70)},
		{"a challenge without attempts does not weigh in", []ChallengeInput{
			{ChallengeID: "c1", Attempts: []LabAttempt{lab(90, 90, false, 1)}},
			{ChallengeID: "c2"},
		}, ptr(90)},
		{"the attempt mean rounds half up", []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{lab(81, 80, false, 1)}}}, ptr(81)},
		{"the mean over challenges rounds half up", []ChallengeInput{
			{ChallengeID: "c1", Attempts: []LabAttempt{lab(80, 80, false, 1)}},
			{ChallengeID: "c2", Attempts: []LabAttempt{lab(81, 80, false, 1)}},
		}, ptr(81)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := SummarizeTopic(TopicInput{Challenges: tt.challenges}, testPassScore)

			assert.Equal(t, tt.want, summary.LabScore)
		})
	}
}

func TestSummarizeTopic_Status(t *testing.T) {
	passedLab := ChallengeInput{ChallengeID: "c1", Attempts: []LabAttempt{lab(90, 90, false, 1)}}
	tests := []struct {
		name  string
		input TopicInput
		want  Status
	}{
		{"nothing at all is insufficient evidence", TopicInput{}, StatusInsufficientEvidence},
		{"a challenge without attempts is insufficient evidence", TopicInput{Challenges: []ChallengeInput{{ChallengeID: "c1"}}}, StatusInsufficientEvidence},
		{"concepts without evidence are insufficient evidence", TopicInput{Concepts: []ConceptInput{{EvidenceCount: 0}}}, StatusInsufficientEvidence},
		{"a concept needing review without evidence is insufficient evidence", TopicInput{Concepts: []ConceptInput{{NeedsReview: true}}}, StatusInsufficientEvidence},
		{"a concept with evidence is stable", TopicInput{Concepts: []ConceptInput{{EvidenceCount: 1}}}, StatusStable},
		{"a quiz at the minimum is stable", TopicInput{Quizzes: []QuizInput{quiz("q1", 80)}}, StatusStable},
		{"a quiz below the minimum needs reinforcement", TopicInput{Quizzes: []QuizInput{quiz("q1", 79)}}, StatusNeedsReinforcement},
		{"a failed lab needs reinforcement", TopicInput{Challenges: []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{lab(85, 40, true, 1)}}}}, StatusNeedsReinforcement},
		{"only the latest lab attempt counts as failed", TopicInput{Challenges: []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{lab(40, 40, true, 1), lab(90, 90, false, 2)}}}}, StatusImproving},
		{"a passed lab is stable", TopicInput{Challenges: []ChallengeInput{passedLab}}, StatusStable},
		{"a failure on any challenge needs reinforcement", TopicInput{Challenges: []ChallengeInput{
			passedLab,
			{ChallengeID: "c2", Attempts: []LabAttempt{lab(50, 50, true, 1)}},
		}}, StatusNeedsReinforcement},
		{"a concept needing review with evidence needs reinforcement", TopicInput{Concepts: []ConceptInput{{NeedsReview: true, EvidenceCount: 1}}}, StatusNeedsReinforcement},
		{"a quiz retaken 60 to 85 is improving", TopicInput{Quizzes: []QuizInput{quiz("q1", 60, 85)}}, StatusImproving},
		{"a rise of 10 points is improving", TopicInput{Quizzes: []QuizInput{quiz("q1", 80, 90)}}, StatusImproving},
		{"a rise of 9 points is stable", TopicInput{Quizzes: []QuizInput{quiz("q1", 81, 90)}}, StatusStable},
		{"a drop is stable", TopicInput{Quizzes: []QuizInput{quiz("q1", 95, 85)}}, StatusStable},
		{"only the last two attempts are compared", TopicInput{Quizzes: []QuizInput{quiz("q1", 60, 95, 90)}}, StatusStable},
		{"a lab rising 10 points in score is improving", TopicInput{Challenges: []ChallengeInput{{ChallengeID: "c1", Attempts: []LabAttempt{lab(80, 80, false, 1), lab(90, 90, false, 2)}}}}, StatusImproving},
		{"attempts of different quizzes are not compared", TopicInput{Quizzes: []QuizInput{quiz("q1", 80), quiz("q2", 95)}}, StatusStable},
		{"needs reinforcement wins over improving", TopicInput{Quizzes: []QuizInput{quiz("q1", 40, 70)}}, StatusNeedsReinforcement},
		{"insufficient evidence wins over everything else", TopicInput{Quizzes: []QuizInput{{QuizID: "q1"}}}, StatusInsufficientEvidence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := SummarizeTopic(tt.input, testPassScore)

			assert.Equal(t, tt.want, summary.Status)
		})
	}
}

func TestSummarizeTopic_UsesThePassScoreGiven(t *testing.T) {
	input := TopicInput{Quizzes: []QuizInput{quiz("q1", 70)}}

	assert.Equal(t, StatusNeedsReinforcement, SummarizeTopic(input, 80).Status)
	assert.Equal(t, StatusStable, SummarizeTopic(input, 70).Status)
}

func TestSummarizeTopic_CarriesTheSessionIdentity(t *testing.T) {
	input := TopicInput{SessionID: "s1", Name: "Cache"}

	summary := SummarizeTopic(input, testPassScore)

	assert.Equal(t, "s1", summary.SessionID)
	assert.Equal(t, "Cache", summary.Name)
}

func TestSummarizeTopic_LastActivity(t *testing.T) {
	tests := []struct {
		name  string
		input TopicInput
		want  time.Time
	}{
		{"the session start when nothing else happened", TopicInput{StartedAt: at(1)}, at(1)},
		{"a later quiz attempt", TopicInput{StartedAt: at(1), Quizzes: []QuizInput{{QuizID: "q", Attempts: []ScoredAttempt{{Percent: 50, At: at(5)}}}}}, at(5)},
		{"a later lab attempt", TopicInput{StartedAt: at(1), Challenges: []ChallengeInput{{ChallengeID: "c", Attempts: []LabAttempt{lab(50, 50, true, 7)}}}}, at(7)},
		{"a later concept update", TopicInput{StartedAt: at(1), Concepts: []ConceptInput{{UpdatedAt: at(9)}}}, at(9)},
		{"the session start when it is the latest", TopicInput{StartedAt: at(20), Concepts: []ConceptInput{{UpdatedAt: at(9)}}}, at(20)},
		{"the latest of several attempts", TopicInput{StartedAt: at(1), Quizzes: []QuizInput{{QuizID: "q", Attempts: []ScoredAttempt{{Percent: 50, At: at(8)}, {Percent: 60, At: at(4)}}}}}, at(8)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := SummarizeTopic(tt.input, testPassScore)

			assert.True(t, tt.want.Equal(summary.LastActivity), "want %s, got %s", tt.want, summary.LastActivity)
		})
	}
}
