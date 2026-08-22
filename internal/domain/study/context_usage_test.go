package study

import "testing"

func TestComputeContextState_thresholds(t *testing.T) {
	cases := []struct {
		name          string
		usedTokens    int
		contextLength int
		want          ContextState
	}{
		{"just below warning", 79, 100, ContextStateNormal},
		{"exactly at warning", 80, 100, ContextStateWarning},
		{"between warning and blocked", 94, 100, ContextStateWarning},
		{"exactly at blocked", 95, 100, ContextStateBlocked},
		{"above blocked", 100, 100, ContextStateBlocked},
		{"zero usage", 0, 100, ContextStateNormal},
		// A length that doesn't divide evenly by 100 is where integer
		// truncation (contextLength*80/100) would misfire; the
		// usedTokens*100 >= contextLength*80 form does not.
		{"non-round length, just below warning", 799, 1000, ContextStateNormal},
		{"non-round length, exactly at warning", 800, 1000, ContextStateWarning},
		{"odd length exact boundary", 76, 95, ContextStateWarning}, // 76*100=7600 >= 95*80=7600
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ComputeContextState(c.usedTokens, c.contextLength)
			if got != c.want {
				t.Errorf("ComputeContextState(%d, %d) = %q, want %q", c.usedTokens, c.contextLength, got, c.want)
			}
		})
	}
}

func TestNextContextUsage_unresolvedContextLength_preservesPreviousState(t *testing.T) {
	// Given a session already at warning
	previous := ContextUsage{State: ContextStateWarning, Model: "m1", UsedTokens: 900, ContextLength: 1000}

	// When a new measurement arrives with no known context length
	got := NextContextUsage(previous, "m1", 50, 0, true)

	// Then the state is preserved unconditionally (thresholds can't be
	// judged without a length), but the other fields update
	if got.State != ContextStateWarning {
		t.Errorf("State = %q, want %q", got.State, ContextStateWarning)
	}
	if got.ContextLength != 0 || got.UsedTokens != 50 || !got.Estimated {
		t.Errorf("got %+v, want ContextLength=0 UsedTokens=50 Estimated=true", got)
	}
}

func TestNextContextUsage_sameModelAndLength_isMonotonic(t *testing.T) {
	// Given a session already at blocked (e.g. from a larger estimate)
	previous := ContextUsage{State: ContextStateBlocked, Model: "m1", UsedTokens: 9600, ContextLength: 10000}

	// When a smaller real measurement replaces the estimate, same model/length
	got := NextContextUsage(previous, "m1", 100, 10000, false)

	// Then the state cannot retreat below blocked even though 100/10000 is
	// well under the warning threshold
	if got.State != ContextStateBlocked {
		t.Errorf("State = %q, want %q (monotonic)", got.State, ContextStateBlocked)
	}
	if got.UsedTokens != 100 || got.Estimated {
		t.Errorf("got %+v, want UsedTokens=100 Estimated=false", got)
	}
}

func TestNextContextUsage_modelChanged_recomputesInEitherDirection(t *testing.T) {
	// Given a session blocked under one model
	previous := ContextUsage{State: ContextStateBlocked, Model: "m1", UsedTokens: 9600, ContextLength: 10000}

	// When the resolved model changes and the new occupancy is comfortably low
	got := NextContextUsage(previous, "m2", 100, 10000, false)

	// Then the state moves back down to normal — blocked -> normal is only
	// allowed because the model changed
	if got.State != ContextStateNormal {
		t.Errorf("State = %q, want %q", got.State, ContextStateNormal)
	}
}

func TestNextContextUsage_contextLengthChanged_recomputesInEitherDirection(t *testing.T) {
	// Given a session warning under a 1000-token window
	previous := ContextUsage{State: ContextStateWarning, Model: "m1", UsedTokens: 850, ContextLength: 1000}

	// When the catalog reports a larger window for the same model
	got := NextContextUsage(previous, "m1", 850, 100000, false)

	// Then the state drops back to normal
	if got.State != ContextStateNormal {
		t.Errorf("State = %q, want %q", got.State, ContextStateNormal)
	}
}

func TestNextContextUsage_freshSession_computesNormally(t *testing.T) {
	got := NextContextUsage(ContextUsage{}, "m1", 950, 1000, false)
	if got.State != ContextStateBlocked {
		t.Errorf("State = %q, want %q", got.State, ContextStateBlocked)
	}
	if got.Model != "m1" || got.ContextLength != 1000 {
		t.Errorf("got %+v", got)
	}
}

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 8},
		{"three runes exactly", "abc", 1 + 8},
		{"four runes, ceils up", "abcd", 2 + 8},
		{"unicode counts runes not bytes", "héllo", 2 + 8}, // 5 runes -> ceil(5/3)=2
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EstimateTokens(c.content)
			if got != c.want {
				t.Errorf("EstimateTokens(%q) = %d, want %d", c.content, got, c.want)
			}
		})
	}
}
