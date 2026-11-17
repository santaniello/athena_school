package quiz

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santaniello/athena/internal/application/llmprompt"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

type generationOutput struct {
	Questions []generationQuestion `json:"questions"`
}

type generationQuestion struct {
	Kind         domainquiz.QuestionKind `json:"kind"`
	Prompt       string                  `json:"prompt"`
	Options      []string                `json:"options"`
	CorrectIndex *int                    `json:"correctIndex"` // nil when absent or null
	Explanation  string                  `json:"explanation"`
}

// parseQuestions turns the model's answer into exactly count questions, with
// their text trimmed. IDs and positions are left for the caller. The domain
// rules for each question are checked later, when the quiz is validated.
// Every failure wraps ErrInvalidGeneration.
func parseQuestions(content string, count int) ([]domainquiz.Question, error) {
	var output generationOutput
	if err := json.Unmarshal([]byte(llmprompt.StripCodeFence(content)), &output); err != nil {
		return nil, fmt.Errorf("%w: reading the JSON: %w", ErrInvalidGeneration, err)
	}
	if len(output.Questions) != count {
		return nil, fmt.Errorf("%w: got %d questions, want %d", ErrInvalidGeneration, len(output.Questions), count)
	}
	questions := make([]domainquiz.Question, count)
	for i, generated := range output.Questions {
		if generated.CorrectIndex == nil {
			return nil, fmt.Errorf("%w: question %d has no correct index", ErrInvalidGeneration, i+1)
		}
		options := make([]string, len(generated.Options))
		for j, option := range generated.Options {
			options[j] = strings.TrimSpace(option)
		}
		questions[i] = domainquiz.Question{
			Kind:         generated.Kind,
			Prompt:       strings.TrimSpace(generated.Prompt),
			Options:      options,
			CorrectIndex: *generated.CorrectIndex,
			Explanation:  strings.TrimSpace(generated.Explanation),
		}
	}
	return questions, nil
}
