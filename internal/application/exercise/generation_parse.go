package exercise

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

const (
	// generatedKeyPrefix keeps a generated criterion key apart from every
	// base criterion key.
	generatedKeyPrefix = "c-"
	// maxGeneratedSlugLength keeps the key within MaxCriterionKeyLength even
	// with the prefix and a collision suffix.
	maxGeneratedSlugLength = 40
	fallbackKeySlug        = "criterion"
)

// generatedChallenge is the validated content of a model answer.
type generatedChallenge struct {
	Challenge CreateChallengeInput // SourceSessionID is left for the caller
	Criteria  []ChallengeCriterionInput
}

type generationOutput struct {
	Title                     string                `json:"title"`
	Scenario                  string                `json:"scenario"`
	FunctionalRequirements    []string              `json:"functionalRequirements"`
	NonFunctionalRequirements []string              `json:"nonFunctionalRequirements"`
	Constraints               []string              `json:"constraints"`
	Objectives                []string              `json:"objectives"`
	Criteria                  []generationCriterion `json:"criteria"`
}

type generationCriterion struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Weight      int      `json:"weight"`
	Mandatory   *bool    `json:"mandatory"` // nil when absent or null
	Levels      []string `json:"levels"`
}

// parseGeneration turns the model's answer into a challenge and its criteria
// and validates both with the domain rules, so nothing invalid reaches
// storage. Every failure wraps ErrInvalidGeneration. The topic is the source
// session's topic, used only to validate.
func parseGeneration(content, topic string, difficulty domainexercise.Difficulty) (generatedChallenge, error) {
	var output generationOutput
	if err := json.Unmarshal(stripCodeFence(content), &output); err != nil {
		return generatedChallenge{}, fmt.Errorf("%w: reading the JSON: %w", ErrInvalidGeneration, err)
	}
	criteria, err := toCriterionInputs(output.Criteria)
	if err != nil {
		return generatedChallenge{}, err
	}
	generated := generatedChallenge{
		Challenge: CreateChallengeInput{
			Title:                     strings.TrimSpace(output.Title),
			Scenario:                  strings.TrimSpace(output.Scenario),
			FunctionalRequirements:    trimItems(output.FunctionalRequirements),
			NonFunctionalRequirements: trimItems(output.NonFunctionalRequirements),
			Constraints:               trimItems(output.Constraints),
			Objectives:                trimItems(output.Objectives),
			Difficulty:                difficulty,
			Origin:                    domainexercise.OriginAIGenerated,
		},
		Criteria: criteria,
	}
	if err := validateGenerated(generated, topic); err != nil {
		return generatedChallenge{}, fmt.Errorf("%w: %w", ErrInvalidGeneration, err)
	}
	return generated, nil
}

// stripCodeFence removes a Markdown code fence some models wrap around JSON.
func stripCodeFence(content string) []byte {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return []byte(trimmed)
	}
	trimmed = strings.TrimPrefix(trimmed, "```")
	if _, afterInfo, found := strings.Cut(trimmed, "\n"); found {
		trimmed = afterInfo
	}
	return bytes.TrimSpace([]byte(strings.TrimSuffix(strings.TrimSpace(trimmed), "```")))
}

func toCriterionInputs(criteria []generationCriterion) ([]ChallengeCriterionInput, error) {
	inputs := make([]ChallengeCriterionInput, len(criteria))
	used := make(map[string]struct{}, len(criteria))
	for i, criterion := range criteria {
		if criterion.Mandatory == nil {
			return nil, fmt.Errorf("%w: criterion %d has no mandatory flag", ErrInvalidGeneration, i+1)
		}
		if len(criterion.Levels) != domainexercise.LevelCount {
			return nil, fmt.Errorf("%w: criterion %d has %d levels, want %d", ErrInvalidGeneration, i+1, len(criterion.Levels), domainexercise.LevelCount)
		}
		inputs[i] = ChallengeCriterionInput{
			Key:         uniqueKey(strings.TrimSpace(criterion.Name), used),
			Name:        strings.TrimSpace(criterion.Name),
			Description: strings.TrimSpace(criterion.Description),
			Weight:      criterion.Weight,
			Mandatory:   *criterion.Mandatory,
		}
		for level, descriptor := range criterion.Levels {
			inputs[i].Levels[level] = strings.TrimSpace(descriptor)
		}
	}
	return inputs, nil
}

// uniqueKey derives a key from name: a lower-case slug with the generated
// prefix, and a numeric suffix when the key is already in used.
func uniqueKey(name string, used map[string]struct{}) string {
	base := generatedKeyPrefix + slugOf(name)
	key := base
	for n := 2; ; n++ {
		if _, taken := used[key]; !taken {
			used[key] = struct{}{}
			return key
		}
		key = base + "-" + strconv.Itoa(n)
	}
}

// slugOf lower-cases name, joins its letter and digit runs with "-" and cuts
// the result to maxGeneratedSlugLength characters.
func slugOf(name string) string {
	words := strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	slug := []rune(strings.Join(words, "-"))
	slug = slug[:min(len(slug), maxGeneratedSlugLength)]
	if trimmed := strings.Trim(string(slug), "-"); trimmed != "" {
		return trimmed
	}
	return fallbackKeySlug
}

// validateGenerated applies the challenge and rubric rules to the content.
func validateGenerated(generated generatedChallenge, topic string) error {
	probe := domainexercise.Challenge{
		Type:                      domainexercise.ChallengeTypeArchitecture,
		Title:                     generated.Challenge.Title,
		Scenario:                  generated.Challenge.Scenario,
		FunctionalRequirements:    generated.Challenge.FunctionalRequirements,
		NonFunctionalRequirements: generated.Challenge.NonFunctionalRequirements,
		Constraints:               generated.Challenge.Constraints,
		Objectives:                generated.Challenge.Objectives,
		Difficulty:                generated.Challenge.Difficulty,
		Topic:                     topic,
		Origin:                    generated.Challenge.Origin,
		TimeLimitMinutes:          domainexercise.DefaultTimeLimit(generated.Challenge.Difficulty),
		DefenseQuestions:          domainexercise.DefaultDefenseQuestions,
	}
	if err := probe.Validate(); err != nil {
		return err
	}
	criteria := make([]domainexercise.RubricCriterion, len(generated.Criteria))
	for i, input := range generated.Criteria {
		criteria[i] = domainexercise.RubricCriterion{
			Key: input.Key, Name: input.Name, Description: input.Description,
			Weight: input.Weight, Mandatory: input.Mandatory, Levels: input.Levels,
		}
	}
	merged, err := domainexercise.MergeRubric(domainexercise.BaseRubric(), criteria)
	if err != nil {
		return err
	}
	return domainexercise.RubricVersion{Criteria: merged}.Validate()
}
