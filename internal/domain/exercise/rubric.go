package exercise

import "time"

// BaseRubricVersion identifies the built-in base rubric. A merged rubric
// records which base version it used. See
// specs/phases/phase-03-architecture-lab/02-rubrics.md.
const BaseRubricVersion = "base-v1"

// NoBaseRubricVersion is the BaseVersion of a lean rubric: one with no base
// criteria, used by a practice challenge. See
// specs/phases/phase-03-architecture-lab/18-practice-challenges.md.
const NoBaseRubricVersion = "none"

// MaxPracticeCriteria is the most criteria a lean practice rubric may have.
const MaxPracticeCriteria = 3

// CriterionSource records where a rubric criterion came from.
type CriterionSource string

// The two criterion origins.
const (
	CriterionSourceBase      CriterionSource = "BASE"
	CriterionSourceChallenge CriterionSource = "CHALLENGE"
)

// LevelCount is the number of described levels of a criterion, 0 (absent) to
// 4 (excellent).
const LevelCount = 5

// RubricCriterion is one thing an evaluation scores a solution on. Levels
// holds the descriptor of each level, index = level.
type RubricCriterion struct {
	ID          string
	Key         string // stable within a version, e.g. "scalability"
	Name        string
	Description string
	Weight      int // whole percentage points
	Source      CriterionSource
	Mandatory   bool
	Levels      [LevelCount]string
}

// RubricVersion is an immutable, merged rubric for one challenge: the base
// criteria plus the challenge's own, weights summing to 100. Changing a
// challenge's rubric creates version N+1; a version is never updated.
type RubricVersion struct {
	ID          string
	ChallengeID string
	Version     int // 1, 2, 3 ... per challenge
	BaseVersion string
	Criteria    []RubricCriterion
	CreatedAt   time.Time
}

// BaseCriteriaFor returns the base criteria a rubric of the given challenge
// kind starts from: the built-in ones for an interview, none for a practice.
func BaseCriteriaFor(kind ChallengeKind) []RubricCriterion {
	if kind == ChallengeKindPractice {
		return nil
	}
	return BaseRubric()
}

// BaseVersionFor returns the BaseVersion a rubric of the given challenge kind
// records.
func BaseVersionFor(kind ChallengeKind) string {
	if kind == ChallengeKindPractice {
		return NoBaseRubricVersion
	}
	return BaseRubricVersion
}

// Lean reports whether the rubric has no base criteria: its challenge is a
// practice.
func (v RubricVersion) Lean() bool {
	return v.BaseVersion == NoBaseRubricVersion
}

// BaseRubric returns the built-in criteria every rubric starts from, in
// order, each with an initial weight of 20. The slice is a fresh copy, so
// callers may modify it.
func BaseRubric() []RubricCriterion {
	return []RubricCriterion{
		{
			Key:         "architecture",
			Name:        "Overall architecture",
			Description: "Components and data flow are defined and coherent.",
			Weight:      20,
			Source:      CriterionSourceBase,
			Levels: [LevelCount]string{
				"No recognizable architecture: components and data flow are missing.",
				"A few components are named but their responsibilities and connections are unclear.",
				"The main components exist, but the data flow has gaps or inconsistencies.",
				"Components and data flow are defined and coherent, with minor omissions.",
				"Every component has a clear responsibility and the data flow is complete and consistent.",
			},
		},
		{
			Key:         "scalability",
			Name:        "Scalability",
			Description: "Meets the stated load and names how the system scales.",
			Weight:      20,
			Source:      CriterionSourceBase,
			Levels: [LevelCount]string{
				"Scalability is not addressed.",
				"Growth is mentioned but not tied to the stated load.",
				"Some scaling mechanisms are named without showing they meet the stated load.",
				"The design meets the stated load and names how each part scales.",
				"The design meets the stated load, identifies bottlenecks and justifies how each part scales beyond it.",
			},
		},
		{
			Key:         "consistency",
			Name:        "Consistency",
			Description: "Data correctness, staleness and propagation are addressed.",
			Weight:      20,
			Source:      CriterionSourceBase,
			Levels: [LevelCount]string{
				"Data consistency is not addressed.",
				"Consistency is mentioned without saying where it matters.",
				"Some consistency concerns are identified but staleness or propagation is ignored.",
				"Correctness, staleness and propagation are addressed for the main data.",
				"Consistency guarantees are chosen per data type, with staleness bounds and propagation explained.",
			},
		},
		{
			Key:         "resilience",
			Name:        "Resilience",
			Description: "Failure scenarios and degraded behavior are handled.",
			Weight:      20,
			Source:      CriterionSourceBase,
			Levels: [LevelCount]string{
				"Failures are not considered.",
				"Failures are mentioned in general terms with no concrete scenario.",
				"Some failure scenarios are named but the resulting behavior is not described.",
				"The main failure scenarios and the degraded behavior for each are handled.",
				"Failure scenarios are covered end to end, including degraded behavior, recovery and detection.",
			},
		},
		{
			Key:         "tradeoffs",
			Name:        "Trade-offs",
			Description: "Choices are justified and alternatives are discussed.",
			Weight:      20,
			Source:      CriterionSourceBase,
			Levels: [LevelCount]string{
				"Choices are stated with no justification.",
				"Some choices are justified with generic reasons.",
				"Key choices are justified but alternatives are not discussed.",
				"Key choices are justified and the main alternatives are discussed.",
				"Every significant choice is justified against alternatives, with its costs and the conditions that would change it.",
			},
		},
	}
}
