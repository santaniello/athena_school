package progress

import (
	"context"
	"errors"
	"fmt"

	domainprogress "github.com/santaniello/athena/internal/domain/progress"
)

type folderInsightPorts struct {
	repo      domainprogress.FolderInsightRepository
	generator domainprogress.FolderInsightGenerator
}

// WithFolderInsights wires the Athena Insights of a folder (spec 5.5). They
// need WithInsights too, for the assistant language and the clock.
func WithFolderInsights(repo domainprogress.FolderInsightRepository, generator domainprogress.FolderInsightGenerator) Option {
	return func(s *Service) {
		s.folderInsights = &folderInsightPorts{repo: repo, generator: generator}
	}
}

func (s *Service) folderInsightsWired() bool {
	return s.insights != nil && s.folderInsights != nil
}

// GenerateFolderInsight asks the model for the insight of a folder from the
// summaries and difficulties of its topics and stores it, replacing the
// previous one. Like GenerateTopicInsight it runs only on the user's request;
// a folder with no topic outside Insufficient evidence is
// domainprogress.ErrInsufficientEvidence, and on any failure the stored
// insight is left as it was. See
// specs/phases/phase-05-progress/05-folder-insights.md.
func (s *Service) GenerateFolderInsight(ctx context.Context, folderID string) (domainprogress.Insight, error) {
	if !s.folderInsightsWired() {
		return domainprogress.Insight{}, ErrInsightsUnavailable
	}
	input, err := s.folderInsightInput(ctx, folderID)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	if !input.HasEvidence() {
		return domainprogress.Insight{}, domainprogress.ErrInsufficientEvidence
	}
	language, err := s.insights.language.AssistantLanguage(ctx)
	if err != nil {
		return domainprogress.Insight{}, fmt.Errorf("progress: reading the assistant language: %w", err)
	}
	raw, err := s.folderInsights.generator.GenerateFolderInsight(ctx, input, language)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	content, err := domainprogress.ParseFolderInsight(raw.Content, input)
	if err != nil {
		return domainprogress.Insight{}, err
	}
	insight := domainprogress.Insight{
		Analysis: content.Analysis, Recommendations: content.Recommendations,
		Model: raw.Model, Language: language,
		Fingerprint: domainprogress.FolderFingerprint(input, language), CreatedAt: s.insights.clock.Now(),
	}
	if err := s.folderInsights.repo.Save(ctx, folderID, insight); err != nil {
		return domainprogress.Insight{}, fmt.Errorf("progress: storing folder insight: %w", err)
	}
	return insight, nil
}

// GetFolderInsight reads the stored insight of the folder and compares its
// fingerprint with the one of the topics now, with no LLM call. Without the
// ports it is the zero view: nothing to show and nothing to generate. On a
// store or language error it still returns the view without the stored
// insight, so the caller can degrade.
func (s *Service) GetFolderInsight(ctx context.Context, folderID string) (domainprogress.InsightView, error) {
	if !s.folderInsightsWired() {
		return domainprogress.InsightView{}, nil
	}
	input, err := s.folderInsightInput(ctx, folderID)
	if err != nil {
		return domainprogress.InsightView{}, err
	}
	view := domainprogress.InsightView{CanGenerate: input.HasEvidence()}
	stored, err := s.folderInsights.repo.Get(ctx, folderID)
	if errors.Is(err, domainprogress.ErrInsightNotFound) {
		return view, nil
	}
	if err != nil {
		return view, fmt.Errorf("progress: reading folder insight: %w", err)
	}
	language, err := s.insights.language.AssistantLanguage(ctx)
	if err != nil {
		return view, fmt.Errorf("progress: reading the assistant language: %w", err)
	}
	view.Current = &stored
	view.Stale = stored.Fingerprint != domainprogress.FolderFingerprint(input, language)
	return view, nil
}

// folderInsightInput maps the topics of the folder: each one's summary, its
// difficulties and its own fingerprint, never its attempts.
func (s *Service) folderInsightInput(ctx context.Context, folderID string) (domainprogress.FolderInsightInput, error) {
	passScore, err := s.passScores.Current(ctx)
	if err != nil {
		return domainprogress.FolderInsightInput{}, fmt.Errorf("progress: reading the pass score: %w", err)
	}
	folder, err := s.reader.ReadFolderTopics(ctx, folderID)
	if err != nil {
		return domainprogress.FolderInsightInput{}, fmt.Errorf("progress: reading folder: %w", err)
	}
	input := domainprogress.FolderInsightInput{FolderID: folder.ID, FolderName: folder.Name}
	for _, topic := range folder.Topics {
		item, err := folderInsightTopic(topic, passScore)
		if err != nil {
			return domainprogress.FolderInsightInput{}, err
		}
		input.Topics = append(input.Topics, item)
	}
	return input, nil
}

func folderInsightTopic(topic TopicData, passScore int) (domainprogress.FolderInsightTopic, error) {
	summary, err := topicSummary(topic, passScore)
	if err != nil {
		return domainprogress.FolderInsightTopic{}, err
	}
	evidence, err := insightInput(topic, passScore)
	if err != nil {
		return domainprogress.FolderInsightTopic{}, err
	}
	detailInput, err := topicDetailInput(topic, passScore)
	if err != nil {
		return domainprogress.FolderInsightTopic{}, err
	}
	item := domainprogress.FolderInsightTopic{
		SessionID: topic.SessionID, Name: topic.Topic, QuizScore: summary.QuizScore, LabScore: summary.LabScore,
		Status: summary.Status, Fingerprint: domainprogress.Fingerprint(evidence, ""),
	}
	for _, difficulty := range domainprogress.BuildTopicDetail(detailInput).Difficulties {
		item.Difficulties = append(item.Difficulties, domainprogress.FolderInsightDifficulty{Name: difficulty.Name, ToReview: difficulty.ToReview})
	}
	return item, nil
}
