package progress

import (
	"slices"
	"time"
)

// EvolutionPoint is one attempt in a series. Order is its position within the
// series; ItemNumber is the position of its quiz or challenge among the topic's
// ones, and Title names the challenge (empty for quizzes).
type EvolutionPoint struct {
	Order      int
	ItemNumber int
	Title      string
	Score      int
	At         time.Time
}

// Evolution is how the results of a topic changed over its attempts: every
// attempt of every quiz and challenge, not only the latest of each. Enough is
// false when there are fewer than MinEvolutionPoints points in total.
type Evolution struct {
	Quiz      []EvolutionPoint
	Lab       []EvolutionPoint
	PassScore int
	Enough    bool
}

// MinEvolutionPoints is the number of points, over both series, below which
// the chart is not worth showing.
const MinEvolutionPoints = 2

// BuildEvolution derives the two series of a topic, each chronological and
// numbered on its own. Lab points use the lab score (mean of diagram and
// defense), so only attempts with both evaluations appear. Both lists are
// non-nil. See specs/phases/phase-05-progress/03-evolution-chart.md.
func BuildEvolution(in TopicInput, passScore int) Evolution {
	var quiz, lab []EvolutionPoint
	for i, q := range in.Quizzes {
		for _, attempt := range q.Attempts {
			quiz = append(quiz, EvolutionPoint{ItemNumber: i + 1, Score: attempt.Percent, At: attempt.At})
		}
	}
	for i, c := range in.Challenges {
		for _, attempt := range c.Attempts {
			lab = append(lab, EvolutionPoint{ItemNumber: i + 1, Title: c.Title, Score: attempt.labScore(), At: attempt.At})
		}
	}
	quiz, lab = numbered(quiz), numbered(lab)
	return Evolution{Quiz: quiz, Lab: lab, PassScore: passScore, Enough: len(quiz)+len(lab) >= MinEvolutionPoints}
}

// numbered orders the points by date, keeping the given order for equal dates,
// and numbers them from 1.
func numbered(points []EvolutionPoint) []EvolutionPoint {
	if points == nil {
		return []EvolutionPoint{}
	}
	slices.SortStableFunc(points, func(a, b EvolutionPoint) int { return a.At.Compare(b.At) })
	for i := range points {
		points[i].Order = i + 1
	}
	return points
}
