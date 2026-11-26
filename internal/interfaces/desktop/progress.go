package desktop

import (
	"context"
	"errors"
	"time"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

// progressDashboard is the use case behind GetProgressDashboard (spec 5.1).
type progressDashboard interface {
	GetDashboard(ctx context.Context) (domainprogress.Dashboard, error)
	GetTopicDetail(ctx context.Context, sessionID string) (domainprogress.TopicDetail, error)
	GenerateTopicInsight(ctx context.Context, sessionID string) (domainprogress.Insight, error)
}

// WithProgressService wires the Progress dashboard.
func WithProgressService(dashboard progressDashboard) Option {
	return func(a *App) { a.progress = dashboard }
}

var errProgressUnavailable = errors.New("progress is not available")

// ProgressCountersResult is the header of the Progress screen.
type ProgressCountersResult struct {
	Folders        int `json:"folders"`
	Topics         int `json:"topics"`
	TopicsToReview int `json:"topicsToReview"`
}

// ProgressTopicResult is one topic (a study session) of a folder. A nil score
// means there is nothing to score yet.
type ProgressTopicResult struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	QuizScore *int   `json:"quizScore"`
	LabScore  *int   `json:"labScore"`
	Status    string `json:"status"`
}

// ProgressFolderResult is a folder that has at least one topic.
type ProgressFolderResult struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	TopicCount int                   `json:"topicCount"`
	Topics     []ProgressTopicResult `json:"topics"`
}

// ProgressDashboardResult is everything the Progress screen shows.
type ProgressDashboardResult struct {
	Counters ProgressCountersResult `json:"counters"`
	Folders  []ProgressFolderResult `json:"folders"`
}

// GetProgressDashboard returns the counters and, per folder with sessions, its
// topics with their scores and status. It is derived on read: no LLM call.
func (a *App) GetProgressDashboard() (ProgressDashboardResult, error) {
	if a.progress == nil {
		return ProgressDashboardResult{}, errProgressUnavailable
	}
	dashboard, err := a.progress.GetDashboard(a.ctx)
	if err != nil {
		return ProgressDashboardResult{}, err
	}
	result := ProgressDashboardResult{
		Counters: ProgressCountersResult{
			Folders:        dashboard.Counters.Folders,
			Topics:         dashboard.Counters.Topics,
			TopicsToReview: dashboard.Counters.TopicsToReview,
		},
		Folders: make([]ProgressFolderResult, len(dashboard.Folders)),
	}
	for i, folder := range dashboard.Folders {
		result.Folders[i] = toProgressFolderResult(folder)
	}
	return result, nil
}

func toProgressFolderResult(folder domainprogress.FolderSummary) ProgressFolderResult {
	result := ProgressFolderResult{
		ID: folder.ID, Name: folder.Name, TopicCount: len(folder.Topics),
		Topics: make([]ProgressTopicResult, len(folder.Topics)),
	}
	for i, topic := range folder.Topics {
		result.Topics[i] = ProgressTopicResult{
			SessionID: topic.SessionID, Name: topic.Name,
			QuizScore: topic.QuizScore, LabScore: topic.LabScore, Status: string(topic.Status),
		}
	}
	return result
}

// ProgressQuizResult is the latest result of one quiz of a topic. A nil
// percent means the quiz has no attempt.
type ProgressQuizResult struct {
	QuizID     string `json:"quizId"`
	Number     int    `json:"number"`
	Difficulty string `json:"difficulty"`
	Percent    *int   `json:"percent"`
	Attempts   int    `json:"attempts"`
}

// ProgressChallengeResult is the latest evaluated attempt of one challenge. A
// nil score means that evaluation is pending.
type ProgressChallengeResult struct {
	ChallengeID      string `json:"challengeId"`
	Title            string `json:"title"`
	Difficulty       string `json:"difficulty"`
	TimeLimitMinutes int    `json:"timeLimitMinutes"`
	DiagramScore     *int   `json:"diagramScore"`
	DefenseScore     *int   `json:"defenseScore"`
	Result           string `json:"result"`
	Attempts         int    `json:"attempts"`
}

// ProgressProofResult is one proof the tutor recorded for a concept.
type ProgressProofResult struct {
	Assessment  string `json:"assessment"`
	Description string `json:"description"`
	Assisted    bool   `json:"assisted"`
	CreatedAt   string `json:"createdAt"`
}

// ProgressConceptResult is a concept of the topic with its proofs, newest first.
type ProgressConceptResult struct {
	Name          string                `json:"name"`
	EvidenceCount int                   `json:"evidenceCount"`
	Proofs        []ProgressProofResult `json:"proofs"`
}

// ProgressDifficultyResult is a difficulty of the topic. Source is TUTOR or
// ARCH_LAB; ChallengeTitle names the challenge for the latter.
type ProgressDifficultyResult struct {
	Name           string                `json:"name"`
	Source         string                `json:"source"`
	ChallengeTitle string                `json:"challengeTitle"`
	ToReview       bool                  `json:"toReview"`
	EvidenceCount  int                   `json:"evidenceCount"`
	Proofs         []ProgressProofResult `json:"proofs"`
}

// ProgressEvolutionPointResult is one attempt in the evolution chart; At is RFC 3339.
type ProgressEvolutionPointResult struct {
	Order      int    `json:"order"`
	ItemNumber int    `json:"itemNumber"`
	Title      string `json:"title"`
	Score      int    `json:"score"`
	At         string `json:"at"`
}

// ProgressEvolutionResult is the two series of the evolution chart. Enough is
// false when there are too few points to draw it.
type ProgressEvolutionResult struct {
	Quiz      []ProgressEvolutionPointResult `json:"quiz"`
	Lab       []ProgressEvolutionPointResult `json:"lab"`
	PassScore int                            `json:"passScore"`
	Enough    bool                           `json:"enough"`
}

// ProgressTopicDetailResult is everything the right panel of Progress shows
// for a topic.
type ProgressTopicDetailResult struct {
	SessionID      string                     `json:"sessionId"`
	Name           string                     `json:"name"`
	FolderName     string                     `json:"folderName"`
	QuizCount      int                        `json:"quizCount"`
	ChallengeCount int                        `json:"challengeCount"`
	ConceptCount   int                        `json:"conceptCount"`
	Quizzes        []ProgressQuizResult       `json:"quizzes"`
	Challenges     []ProgressChallengeResult  `json:"challenges"`
	Understood     []ProgressConceptResult    `json:"understood"`
	Difficulties   []ProgressDifficultyResult `json:"difficulties"`
	ExplainedOnly  []ProgressConceptResult    `json:"explainedOnly"`
	Evolution      ProgressEvolutionResult    `json:"evolution"`
	Insight        ProgressInsightStateResult `json:"insight"`
}

// ProgressCitationResult is an activity that supports a statement of an
// insight. Kind is QUIZ, CHALLENGE or CONCEPT; Label is what the user sees.
type ProgressCitationResult struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ProgressStatementResult is one sentence of the analysis or one
// recommendation, with the activities that support it.
type ProgressStatementResult struct {
	Text      string                   `json:"text"`
	Citations []ProgressCitationResult `json:"citations"`
}

// ProgressInsightResult is the stored Athena Insights of a topic; CreatedAt is
// RFC 3339.
type ProgressInsightResult struct {
	Analysis        []ProgressStatementResult `json:"analysis"`
	Recommendations []ProgressStatementResult `json:"recommendations"`
	Model           string                    `json:"model"`
	Language        string                    `json:"language"`
	CreatedAt       string                    `json:"createdAt"`
}

// ProgressInsightStateResult is the insight card of a topic. A nil Insight
// means none was generated yet; Stale means new activity or a new language
// since it was written; CanGenerate is false while the topic has no evidence.
type ProgressInsightStateResult struct {
	Insight     *ProgressInsightResult `json:"insight"`
	Stale       bool                   `json:"stale"`
	CanGenerate bool                   `json:"canGenerate"`
}

// GetProgressTopic returns the detail of the topic that is the given session.
// It is derived on read: no LLM call.
func (a *App) GetProgressTopic(sessionID string) (ProgressTopicDetailResult, error) {
	if a.progress == nil {
		return ProgressTopicDetailResult{}, errProgressUnavailable
	}
	detail, err := a.progress.GetTopicDetail(a.ctx, sessionID)
	if err != nil {
		return ProgressTopicDetailResult{}, err
	}
	result := ProgressTopicDetailResult{
		SessionID: detail.SessionID, Name: detail.Name, FolderName: detail.FolderName,
		QuizCount: detail.QuizCount, ChallengeCount: detail.ChallengeCount, ConceptCount: detail.ConceptCount,
		Quizzes:       make([]ProgressQuizResult, len(detail.Quizzes)),
		Challenges:    make([]ProgressChallengeResult, len(detail.Challenges)),
		Understood:    toProgressConceptResults(detail.Understood),
		Difficulties:  make([]ProgressDifficultyResult, len(detail.Difficulties)),
		ExplainedOnly: toProgressConceptResults(detail.ExplainedOnly),
		Evolution: ProgressEvolutionResult{
			Quiz: toEvolutionPoints(detail.Evolution.Quiz), Lab: toEvolutionPoints(detail.Evolution.Lab),
			PassScore: detail.Evolution.PassScore, Enough: detail.Evolution.Enough,
		},
		Insight: toProgressInsightState(detail.Insight),
	}
	for i, quiz := range detail.Quizzes {
		result.Quizzes[i] = ProgressQuizResult{
			QuizID: quiz.QuizID, Number: quiz.Number, Difficulty: quiz.Difficulty, Percent: quiz.Percent, Attempts: quiz.Attempts,
		}
	}
	for i, challenge := range detail.Challenges {
		result.Challenges[i] = ProgressChallengeResult{
			ChallengeID: challenge.ChallengeID, Title: challenge.Title, Difficulty: challenge.Difficulty,
			TimeLimitMinutes: challenge.TimeLimitMinutes, DiagramScore: challenge.DiagramScore, DefenseScore: challenge.DefenseScore, Result: string(challenge.Result), Attempts: challenge.Attempts,
		}
	}
	for i, difficulty := range detail.Difficulties {
		result.Difficulties[i] = ProgressDifficultyResult{
			Name: difficulty.Name, Source: string(difficulty.Source), ChallengeTitle: difficulty.ChallengeTitle,
			ToReview: difficulty.ToReview, EvidenceCount: difficulty.EvidenceCount, Proofs: toProgressProofResults(difficulty.Proofs),
		}
	}
	return result, nil
}

func toEvolutionPoints(points []domainprogress.EvolutionPoint) []ProgressEvolutionPointResult {
	results := make([]ProgressEvolutionPointResult, len(points))
	for i, point := range points {
		results[i] = ProgressEvolutionPointResult{
			Order: point.Order, ItemNumber: point.ItemNumber, Title: point.Title, Score: point.Score, At: point.At.Format(time.RFC3339),
		}
	}
	return results
}

func toProgressConceptResults(concepts []domainprogress.ConceptDetail) []ProgressConceptResult {
	results := make([]ProgressConceptResult, len(concepts))
	for i, concept := range concepts {
		results[i] = ProgressConceptResult{
			Name: concept.Name, EvidenceCount: concept.EvidenceCount, Proofs: toProgressProofResults(concept.Proofs),
		}
	}
	return results
}

func toProgressProofResults(proofs []domainprogress.DetailProof) []ProgressProofResult {
	results := make([]ProgressProofResult, len(proofs))
	for i, proof := range proofs {
		results[i] = ProgressProofResult{
			Assessment: proof.Assessment, Description: proof.Description, Assisted: proof.Assisted,
			CreatedAt: proof.At.UTC().Format(time.RFC3339),
		}
	}
	return results
}

// GenerateProgressInsight asks the model for the Athena Insights of the topic
// that is the given session, from its stored evidence, stores it and returns
// the card, fresh. A topic without evidence is not an error: the card comes
// back with canGenerate false. It is the only Progress call that uses the LLM, and only
// when the user asks for it.
func (a *App) GenerateProgressInsight(sessionID string) (ProgressInsightStateResult, error) {
	if a.progress == nil {
		return ProgressInsightStateResult{}, errProgressUnavailable
	}
	insight, err := a.progress.GenerateTopicInsight(a.ctx, sessionID)
	if errors.Is(err, domainprogress.ErrInsufficientEvidence) {
		// Not a failure to retry: the card shows the guidance to gather evidence.
		return ProgressInsightStateResult{CanGenerate: false}, nil
	}
	if err != nil {
		return ProgressInsightStateResult{}, err
	}
	return toProgressInsightState(domainprogress.InsightView{Current: &insight, CanGenerate: true}), nil
}

func toProgressInsightState(view domainprogress.InsightView) ProgressInsightStateResult {
	state := ProgressInsightStateResult{Stale: view.Stale, CanGenerate: view.CanGenerate}
	if view.Current != nil {
		state.Insight = &ProgressInsightResult{
			Analysis:        toProgressStatements(view.Current.Analysis),
			Recommendations: toProgressStatements(view.Current.Recommendations),
			Model:           view.Current.Model, Language: view.Current.Language,
			CreatedAt: view.Current.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	return state
}

func toProgressStatements(statements []domainprogress.Statement) []ProgressStatementResult {
	results := make([]ProgressStatementResult, len(statements))
	for i, statement := range statements {
		citations := make([]ProgressCitationResult, len(statement.Citations))
		for j, citation := range statement.Citations {
			citations[j] = ProgressCitationResult{Kind: string(citation.Kind), ID: citation.ID, Label: citation.Label}
		}
		results[i] = ProgressStatementResult{Text: statement.Text, Citations: citations}
	}
	return results
}
