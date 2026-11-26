package progress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Limits of an insight: how many recommendations and how many sentences of
// analysis it may hold.
const (
	MaxInsightRecommendations = 3
	MaxInsightAnalysis        = 6
)

var (
	// ErrInvalidInsight is returned by ParseInsight when the model's answer is
	// not usable. Nothing is stored and the previous insight stays.
	ErrInvalidInsight = errors.New("invalid topic insight")
	// ErrInsufficientEvidence is returned when an insight is asked for a topic
	// that has no evidence yet (spec 5.4, decision 7).
	ErrInsufficientEvidence = errors.New("topic has insufficient evidence for an insight")
	// ErrInsightNotFound is returned by an InsightRepository when a topic has
	// no stored insight.
	ErrInsightNotFound = errors.New("topic insight not found")
)

// InsightQuizAttempt is one finished quiz attempt.
type InsightQuizAttempt struct {
	Percent int
	At      time.Time
}

// InsightQuiz is a quiz of the topic. Number is the position the user sees.
type InsightQuiz struct {
	ID         string
	Number     int
	Difficulty string
	Attempts   []InsightQuizAttempt // oldest first
}

// InsightCriterion is the level the evaluation gave to one rubric criterion.
type InsightCriterion struct {
	Name                 string
	Level                int // 0..4
	Justification        string
	InsufficientEvidence bool
}

// InsightEvaluation is the diagram or the defense evaluation of a lab attempt.
type InsightEvaluation struct {
	Score           int
	Criteria        []InsightCriterion
	Strengths       []string
	Gaps            []string
	Recommendations []string
}

// InsightLabAttempt is one attempt of a challenge. A nil evaluation does not
// exist. At is the latest of the attempt's own time and its evaluations', so
// an evaluation that lands later still moves the fingerprint.
type InsightLabAttempt struct {
	Result  LabResult
	Diagram *InsightEvaluation
	Defense *InsightEvaluation
	At      time.Time
}

// InsightChallenge is an Arch Lab challenge of the topic.
type InsightChallenge struct {
	ID       string
	Title    string
	Attempts []InsightLabAttempt // oldest first
}

// InsightProof is one proof the tutor recorded for a concept.
type InsightProof struct {
	Assessment  string
	Description string
	Assisted    bool
	At          time.Time
}

// InsightConcept is a concept of the session with its proofs.
type InsightConcept struct {
	Name      string
	Status    ConceptStatus
	UpdatedAt time.Time
	Proofs    []InsightProof
}

// InsightInput is the persisted evidence an insight is written from, never the
// conversation. Quizzes, challenges and concepts are the activities: each gets
// a ref ("Q1", "L1", "K1") in the prompt, which the model cites back.
type InsightInput struct {
	SessionID  string
	Topic      string
	Quizzes    []InsightQuiz
	Challenges []InsightChallenge
	Concepts   []InsightConcept
}

// QuizRef is the id of the i-th quiz (from 0) in the prompt.
func (InsightInput) QuizRef(i int) string { return fmt.Sprintf("Q%d", i+1) }

// ChallengeRef is the id of the i-th challenge (from 0) in the prompt.
func (InsightInput) ChallengeRef(i int) string { return fmt.Sprintf("L%d", i+1) }

// ConceptRef is the id of the i-th concept (from 0) in the prompt.
func (InsightInput) ConceptRef(i int) string { return fmt.Sprintf("K%d", i+1) }

// Fingerprint is a hash of the activities of the input with the time of their
// latest attempt, update or proof, the result of each lab attempt (it moves
// with the pass score) and the assistant language. Two equal fingerprints mean
// an insight written for one is still current for the other.
func Fingerprint(in InsightInput, language string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "language %q\n", language)
	for _, quiz := range in.Quizzes {
		var latest time.Time
		for _, attempt := range quiz.Attempts {
			latest = laterOf(latest, attempt.At)
		}
		fmt.Fprintf(&b, "quiz %q %d\n", quiz.ID, unixNano(latest))
	}
	for _, challenge := range in.Challenges {
		var latest time.Time
		for _, attempt := range challenge.Attempts {
			latest = laterOf(latest, attempt.At)
			fmt.Fprintf(&b, "result %q\n", attempt.Result)
		}
		fmt.Fprintf(&b, "challenge %q %d\n", challenge.ID, unixNano(latest))
	}
	for _, concept := range in.Concepts {
		latest := concept.UpdatedAt
		for _, proof := range concept.Proofs {
			latest = laterOf(latest, proof.At)
		}
		fmt.Fprintf(&b, "concept %q %d\n", concept.Name, unixNano(latest))
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func unixNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// CitationKind says what kind of activity a Citation points to.
type CitationKind string

// The activities an insight can cite.
const (
	CitationQuiz      CitationKind = "QUIZ"
	CitationChallenge CitationKind = "CHALLENGE"
	CitationConcept   CitationKind = "CONCEPT"
)

// Citation points to the activity that supports a statement. ID is the id of
// the quiz or challenge, or the name of the concept; Label is what the user
// sees: "Quiz 2", the challenge title or the concept name.
type Citation struct {
	Kind  CitationKind
	ID    string
	Label string
}

// Statement is one sentence of the analysis or one recommendation, with the
// activities that support it. Citations is never nil.
type Statement struct {
	Text      string
	Citations []Citation
}

// InsightContent is what the model wrote, validated.
type InsightContent struct {
	Analysis        []Statement
	Recommendations []Statement
}

// Insight is the stored Athena Insights of a topic. Fingerprint is the one of
// the input it was written from; Language is the assistant language it is in.
type Insight struct {
	Analysis        []Statement
	Recommendations []Statement
	Model           string
	Language        string
	Fingerprint     string
	CreatedAt       time.Time
}

// InsightView is what the topic detail shows about the insight: the stored one,
// if any, whether new evidence or a new language made it stale, and whether an
// insight can be generated at all (not while the topic has no evidence).
type InsightView struct {
	Current     *Insight
	Stale       bool
	CanGenerate bool
}

// RawInsight is the model's answer to InsightGenerator: the JSON text, without
// any Markdown fence, and the exact model that wrote it.
type RawInsight struct {
	Content string
	Model   string
}

// InsightGenerator writes an insight from the evidence of a topic, in the
// given assistant language. Implemented in the application layer on top of the
// LLM provider.
type InsightGenerator interface {
	GenerateInsight(ctx context.Context, in InsightInput, language string) (RawInsight, error)
}

// InsightRepository stores the latest insight of each topic.
type InsightRepository interface {
	// Get returns the insight of the session, or ErrInsightNotFound.
	Get(ctx context.Context, sessionID string) (Insight, error)
	// Save stores the insight of the session, replacing any previous one.
	Save(ctx context.Context, sessionID string, insight Insight) error
}

type insightAnswer struct {
	Analysis        []statementAnswer `json:"analysis"`
	Recommendations []statementAnswer `json:"recommendations"`
}

type statementAnswer struct {
	Text       string   `json:"text"`
	Activities []string `json:"activities"`
}

// ParseInsight validates the model's JSON answer against the input: at least
// one to MaxInsightAnalysis sentences of analysis and one to
// MaxInsightRecommendations recommendations, none of them blank. A cited activity that is not in the
// input is dropped, and so is a repeated one. Anything else wrong is
// ErrInvalidInsight.
func ParseInsight(raw string, in InsightInput) (InsightContent, error) {
	var answer insightAnswer
	if err := json.Unmarshal([]byte(raw), &answer); err != nil {
		return InsightContent{}, fmt.Errorf("%w: %v", ErrInvalidInsight, err)
	}
	if len(answer.Analysis) > MaxInsightAnalysis {
		return InsightContent{}, fmt.Errorf("%w: %d analysis sentences, at most %d", ErrInvalidInsight, len(answer.Analysis), MaxInsightAnalysis)
	}
	if len(answer.Recommendations) > MaxInsightRecommendations {
		return InsightContent{}, fmt.Errorf("%w: %d recommendations, at most %d", ErrInvalidInsight, len(answer.Recommendations), MaxInsightRecommendations)
	}
	known := citationsByRef(in)
	analysis, err := statements(answer.Analysis, known)
	if err != nil {
		return InsightContent{}, fmt.Errorf("%w: analysis: %v", ErrInvalidInsight, err)
	}
	recommendations, err := statements(answer.Recommendations, known)
	if err != nil {
		return InsightContent{}, fmt.Errorf("%w: recommendations: %v", ErrInvalidInsight, err)
	}
	return InsightContent{Analysis: analysis, Recommendations: recommendations}, nil
}

func statements(answers []statementAnswer, known map[string]Citation) ([]Statement, error) {
	if len(answers) == 0 {
		return nil, errors.New("empty")
	}
	result := make([]Statement, len(answers))
	for i, answer := range answers {
		text := strings.TrimSpace(answer.Text)
		if text == "" {
			return nil, fmt.Errorf("item %d is blank", i+1)
		}
		result[i] = Statement{Text: text, Citations: citationsOf(answer.Activities, known)}
	}
	return result, nil
}

func citationsOf(refs []string, known map[string]Citation) []Citation {
	citations := []Citation{}
	seen := map[string]bool{}
	for _, ref := range refs {
		citation, ok := known[ref]
		if !ok || seen[ref] {
			continue
		}
		seen[ref] = true
		citations = append(citations, citation)
	}
	return citations
}

func citationsByRef(in InsightInput) map[string]Citation {
	known := map[string]Citation{}
	for i, quiz := range in.Quizzes {
		known[in.QuizRef(i)] = Citation{Kind: CitationQuiz, ID: quiz.ID, Label: fmt.Sprintf("Quiz %d", quiz.Number)}
	}
	for i, challenge := range in.Challenges {
		known[in.ChallengeRef(i)] = Citation{Kind: CitationChallenge, ID: challenge.ID, Label: challenge.Title}
	}
	for i, concept := range in.Concepts {
		known[in.ConceptRef(i)] = Citation{Kind: CitationConcept, ID: concept.Name, Label: concept.Name}
	}
	return known
}
