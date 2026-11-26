package progress

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// ConceptStatus is what the tutor observed about a concept. The values match
// study.ConceptStatus; the package keeps its own so it imports nothing (ADR-001).
type ConceptStatus string

// The concept statuses the topic detail tells apart.
const (
	StatusNotAssessed         ConceptStatus = "NOT_ASSESSED"
	StatusIntroduced          ConceptStatus = "INTRODUCED"
	StatusPartiallyUnderstood ConceptStatus = "PARTIALLY_UNDERSTOOD"
	StatusDemonstrated        ConceptStatus = "DEMONSTRATED"
	StatusNeedsReview         ConceptStatus = "NEEDS_REVIEW"
)

// LabResult is the outcome of an Arch Lab attempt, computed by the application
// with the Arch Lab's own pass rule.
type LabResult string

// The results an attempt can have.
const (
	LabResultPending LabResult = "PENDING"
	LabResultPassed  LabResult = "PASSED"
	LabResultFailed  LabResult = "FAILED"
)

// DifficultySource says where a difficulty comes from.
type DifficultySource string

// The sources of a difficulty.
const (
	DifficultySourceTutor   DifficultySource = "TUTOR"
	DifficultySourceArchLab DifficultySource = "ARCH_LAB"
)

// DetailQuiz is a quiz of the topic with its attempts oldest first. The quizzes
// are given oldest first: their position is the number the user sees.
type DetailQuiz struct {
	QuizID     string
	Difficulty string
	Attempts   []ScoredAttempt
}

// DetailLabAttempt is one attempt of a challenge, evaluated or not. A nil
// score means that evaluation does not exist. WeakCriteria are the titles of
// the rubric criteria judged below the mandatory minimum level in the diagram
// or the defense evaluation, without the ones flagged as insufficient evidence.
type DetailLabAttempt struct {
	DiagramScore *int
	DefenseScore *int
	Result       LabResult
	WeakCriteria []string
	At           time.Time
}

func (a DetailLabAttempt) evaluated() bool { return a.DiagramScore != nil || a.DefenseScore != nil }

// DetailChallenge is a challenge of the topic with all its attempts, oldest first.
type DetailChallenge struct {
	ChallengeID      string
	Title            string
	Difficulty       string
	TimeLimitMinutes int
	Attempts         []DetailLabAttempt
}

// DetailProof is one proof the tutor recorded for a concept.
type DetailProof struct {
	Assessment  string
	Description string
	Assisted    bool
	At          time.Time
}

// DetailConcept is one concept of the session. Name is the normalized identity;
// DisplayName is the spelling of the latest report and may be empty for rows
// written before it existed.
type DetailConcept struct {
	Name          string
	DisplayName   string
	Status        ConceptStatus
	EvidenceCount int
	UpdatedAt     time.Time
	Proofs        []DetailProof
}

// TopicDetailInput is everything the detail of one topic is derived from.
type TopicDetailInput struct {
	SessionID  string
	Name       string
	FolderName string
	Quizzes    []DetailQuiz
	Challenges []DetailChallenge
	Concepts   []DetailConcept
}

// QuizResult is the latest result of one quiz, shown as "Quiz Number ·
// Difficulty". A nil Percent means no attempt.
type QuizResult struct {
	QuizID     string
	Number     int
	Difficulty string
	Percent    *int
	Attempts   int
}

// ChallengeResult is the latest evaluated attempt of one challenge. A nil
// score means that evaluation is still pending.
type ChallengeResult struct {
	ChallengeID      string
	Title            string
	Difficulty       string // with TimeLimitMinutes, what opening the exam shows
	TimeLimitMinutes int
	DiagramScore     *int
	DefenseScore     *int
	Result           LabResult
	Attempts         int
}

// ConceptDetail is a concept as the detail shows it, with its proofs newest first.
type ConceptDetail struct {
	Name          string
	Status        ConceptStatus
	EvidenceCount int
	UpdatedAt     time.Time
	Proofs        []DetailProof
}

// Difficulty is something the user struggles with: a concept the tutor saw
// partially understood or in need of review, or a rubric criterion an Arch Lab
// evaluation judged too low. ChallengeTitle is set for the Arch Lab ones and
// ToReview for concepts in NEEDS_REVIEW.
type Difficulty struct {
	Name           string
	Source         DifficultySource
	ChallengeTitle string
	ToReview       bool
	EvidenceCount  int
	UpdatedAt      time.Time
	Proofs         []DetailProof
}

// TopicDetail is what the right panel of Progress shows for a topic.
type TopicDetail struct {
	SessionID      string
	Name           string
	FolderName     string
	QuizCount      int
	ChallengeCount int
	ConceptCount   int
	Quizzes        []QuizResult
	Challenges     []ChallengeResult
	Understood     []ConceptDetail // DEMONSTRATED
	Difficulties   []Difficulty
	ExplainedOnly  []ConceptDetail // INTRODUCED: explained, never shown as understanding
	Evolution      Evolution       // set by the application, from BuildEvolution
	Insight        InsightView     // set by the application, from the stored insight
}

// BuildTopicDetail derives the detail of a topic. Every list is non-nil.
// Concepts are ordered by most recent update, ties by name; the difficulties
// of the tutor come before the ones of the Arch Lab.
func BuildTopicDetail(in TopicDetailInput) TopicDetail {
	detail := TopicDetail{
		SessionID: in.SessionID, Name: in.Name, FolderName: in.FolderName,
		QuizCount: len(in.Quizzes), ChallengeCount: len(in.Challenges), ConceptCount: len(in.Concepts),
		Quizzes:       make([]QuizResult, len(in.Quizzes)),
		Challenges:    make([]ChallengeResult, len(in.Challenges)),
		Understood:    []ConceptDetail{},
		Difficulties:  []Difficulty{},
		ExplainedOnly: []ConceptDetail{},
	}
	for i, quiz := range in.Quizzes {
		detail.Quizzes[i] = quizResult(quiz, i+1)
	}
	for i, challenge := range in.Challenges {
		detail.Challenges[i] = challengeResult(challenge)
	}
	concepts := slices.Clone(in.Concepts)
	slices.SortStableFunc(concepts, func(a, b DetailConcept) int {
		return cmp.Or(b.UpdatedAt.Compare(a.UpdatedAt), strings.Compare(a.Name, b.Name))
	})
	for _, c := range concepts {
		switch c.Status {
		case StatusDemonstrated:
			detail.Understood = append(detail.Understood, conceptDetail(c))
		case StatusIntroduced:
			detail.ExplainedOnly = append(detail.ExplainedOnly, conceptDetail(c))
		case StatusPartiallyUnderstood, StatusNeedsReview:
			detail.Difficulties = append(detail.Difficulties, tutorDifficulty(c))
		}
	}
	for _, challenge := range in.Challenges {
		detail.Difficulties = append(detail.Difficulties, labDifficulties(challenge)...)
	}
	return detail
}

func quizResult(quiz DetailQuiz, number int) QuizResult {
	result := QuizResult{QuizID: quiz.QuizID, Number: number, Difficulty: quiz.Difficulty, Attempts: len(quiz.Attempts)}
	if n := len(quiz.Attempts); n > 0 {
		result.Percent = &quiz.Attempts[n-1].Percent
	}
	return result
}

// latestEvaluated is the most recent attempt that has at least one evaluation.
func latestEvaluated(challenge DetailChallenge) (DetailLabAttempt, bool) {
	for i := len(challenge.Attempts) - 1; i >= 0; i-- {
		if challenge.Attempts[i].evaluated() {
			return challenge.Attempts[i], true
		}
	}
	return DetailLabAttempt{}, false
}

func challengeResult(challenge DetailChallenge) ChallengeResult {
	result := ChallengeResult{
		ChallengeID: challenge.ChallengeID, Title: challenge.Title,
		Difficulty: challenge.Difficulty, TimeLimitMinutes: challenge.TimeLimitMinutes,
		Result: LabResultPending, Attempts: len(challenge.Attempts),
	}
	if attempt, ok := latestEvaluated(challenge); ok {
		result.DiagramScore, result.DefenseScore, result.Result = attempt.DiagramScore, attempt.DefenseScore, attempt.Result
	}
	return result
}

func conceptName(c DetailConcept) string {
	if c.DisplayName != "" {
		return c.DisplayName
	}
	return c.Name
}

func proofsNewestFirst(proofs []DetailProof) []DetailProof {
	sorted := slices.Clone(proofs)
	slices.SortStableFunc(sorted, func(a, b DetailProof) int { return b.At.Compare(a.At) })
	return sorted
}

func conceptDetail(c DetailConcept) ConceptDetail {
	return ConceptDetail{
		Name: conceptName(c), Status: c.Status, EvidenceCount: c.EvidenceCount,
		UpdatedAt: c.UpdatedAt, Proofs: proofsNewestFirst(c.Proofs),
	}
}

func tutorDifficulty(c DetailConcept) Difficulty {
	return Difficulty{
		Name: conceptName(c), Source: DifficultySourceTutor, ToReview: c.Status == StatusNeedsReview,
		EvidenceCount: c.EvidenceCount, UpdatedAt: c.UpdatedAt, Proofs: proofsNewestFirst(c.Proofs),
	}
}

func labDifficulties(challenge DetailChallenge) []Difficulty {
	attempt, ok := latestEvaluated(challenge)
	if !ok {
		return nil
	}
	difficulties := make([]Difficulty, len(attempt.WeakCriteria))
	for i, criterion := range attempt.WeakCriteria {
		difficulties[i] = Difficulty{Name: criterion, Source: DifficultySourceArchLab, ChallengeTitle: challenge.Title}
	}
	return difficulties
}
