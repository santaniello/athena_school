package study

import (
	"encoding/json"
	"errors"
	"strings"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// The tutor ends a reply with a hidden block that reports the pedagogical
// state of the turn. The backend reads it, removes it from the stored and
// displayed text and stores the report. See
// specs/phases/phase-04-tutor/02-pedagogical-state.md.
const (
	tutorStateOpenTag  = "[[ATHENA_TUTOR_STATE]]"
	tutorStateCloseTag = "[[/ATHENA_TUTOR_STATE]]"
)

// tutorStateInstruction is the study-prompt rule that makes the model report
// the state of the turn. It is a pure string so the prompt test pins it, and a
// var built with strings.Join (not a const) so mutation testing has no string-
// concatenation mutants to chase. It starts with a newline, like the offer
// instructions it follows, and holds no percent sign because it becomes part
// of the Sprintf format of buildSystemPrompt.
var tutorStateInstruction = strings.Join([]string{
	"\n- Pedagogical state: end a reply with one state block, only when you actually worked on a concept in this reply (never on a greeting or small talk).",
	"The user never sees this block. Write it exactly like this, on its own last lines, after all the text, with nothing after it, at most once per reply:",
	tutorStateOpenTag,
	`{"concept":"Race Condition","status":"PARTIALLY_UNDERSTOOD","strategy":"CORRECTIVE_FEEDBACK","userEvidence":true,"assisted":true,"assessment":"PARTIALLY_CORRECT","description":"Named the shared state but missed the missing lock"}`,
	tutorStateCloseTag,
	"concept is a short name for what the turn was about. status is NOT_ASSESSED, INTRODUCED, PARTIALLY_UNDERSTOOD, DEMONSTRATED or NEEDS_REVIEW.",
	"strategy is one of the six strategy names above, for the approach you used in this reply.",
	"Set userEvidence to true only when the user's own latest message shows the behavior behind the status.",
	"Use DEMONSTRATED only when userEvidence is true, never for a concept you only explained. A concept you explain is at most INTRODUCED.",
	"Set assisted to true when you had to give a hint or explain to get there.",
	"Include assessment and description only when userEvidence is true. assessment is CORRECT, PARTIALLY_CORRECT or INCORRECT, for the user's latest message about the concept.",
	"description is one short sentence, in the user's language, saying what the user showed.",
	"Use DEMONSTRATED only with assessment CORRECT or PARTIALLY_CORRECT, never with INCORRECT.",
	"The strategy field of this block is the only place you may write a strategy name; everywhere else keep describing your approach in plain words.",
}, "\n")

// The reasons a block is not used. They are fixed texts, never built from the
// reply, so they can be logged without exposing message content.
var (
	errTutorStateMalformed    = errors.New("tutor state block is malformed")
	errTutorStateMultiple     = errors.New("tutor state block appears more than once")
	errTutorStateUnterminated = errors.New("tutor state block is not closed")
	errTutorStateNotAtEnd     = errors.New("tutor state block is not at the end of the reply")
	// errTutorStateStrategy is not fatal: the report is still returned.
	errTutorStateStrategy = errors.New("tutor state strategy is missing or unknown")
)

// tutorStatePayload is the JSON inside the block.
type tutorStatePayload struct {
	Concept      string `json:"concept"`
	Status       string `json:"status"`
	Strategy     string `json:"strategy"`
	UserEvidence bool   `json:"userEvidence"`
	Assisted     bool   `json:"assisted"`
	Assessment   string `json:"assessment"`
	Description  string `json:"description"`
}

// parseTutorState removes the state block from content and reads its report.
// content without a block is returned untouched. With a block, cleaned is the
// text without it, trimmed, even when the block is not used.
//
// A non-nil report is to be applied; a non-nil err is to be logged. They are
// both set only for an omitted or unknown strategy, which does not invalidate
// the report (see errTutorStateStrategy). Every other problem (malformed
// JSON, unknown status, blank concept, more than one block, an unterminated
// block, text after the block) returns a nil report. The report's Concept is
// as the model wrote it; the domain normalizes it.
func parseTutorState(content string) (cleaned string, report *domainstudy.StateReport, err error) {
	if !strings.Contains(content, tutorStateOpenTag) {
		return content, nil, nil
	}

	var text strings.Builder
	var bodies []string
	unterminated := false
	rest := content
	for {
		open := strings.Index(rest, tutorStateOpenTag)
		if open < 0 {
			text.WriteString(rest)
			break
		}
		text.WriteString(rest[:open])
		rest = rest[open+len(tutorStateOpenTag):]
		end := strings.Index(rest, tutorStateCloseTag)
		if end < 0 {
			unterminated = true
			break
		}
		bodies = append(bodies, rest[:end])
		rest = rest[end+len(tutorStateCloseTag):]
	}
	cleaned = strings.TrimSpace(text.String())

	if unterminated {
		return cleaned, nil, errTutorStateUnterminated
	}
	if len(bodies) > 1 {
		return cleaned, nil, errTutorStateMultiple
	}
	if strings.TrimSpace(rest) != "" {
		return cleaned, nil, errTutorStateNotAtEnd
	}
	report, err = readTutorStateReport(bodies[0])
	return cleaned, report, err
}

// readTutorStateReport validates the JSON body of one block.
func readTutorStateReport(body string) (*domainstudy.StateReport, error) {
	var payload tutorStatePayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, errTutorStateMalformed
	}
	status := domainstudy.ConceptStatus(payload.Status)
	if !status.IsValid() || domainstudy.NormalizeConceptName(payload.Concept) == "" {
		return nil, errTutorStateMalformed
	}

	report := &domainstudy.StateReport{
		Concept:      payload.Concept,
		Status:       status,
		UserEvidence: payload.UserEvidence,
		Assisted:     payload.Assisted,
		Description:  payload.Description,
	}
	// An omitted or unknown assessment only means no evidence row is written;
	// it never invalidates the report.
	if assessment := domainstudy.Assessment(strings.ToUpper(strings.TrimSpace(payload.Assessment))); assessment.IsValid() {
		report.Assessment = assessment
	}
	strategy := domainstudy.TutorStrategy(strings.ToUpper(strings.TrimSpace(payload.Strategy)))
	if !strategy.IsValid() {
		return report, errTutorStateStrategy
	}
	report.Strategy = strategy
	return report, nil
}

// tutorStateFilter sits in the chunk stream shown to the user. It drops every
// block, from the opening tag to the closing one, and holds back a trailing
// fragment that could be the start of a tag, so the user never sees any of the
// protocol. flush ends the stream: an unterminated block is dropped, a held
// fragment that never completed is released.
type tutorStateFilter struct {
	pending string
	inBlock bool
}

// write takes the next chunk and returns the text that is safe to show now.
func (f *tutorStateFilter) write(chunk string) string {
	f.pending += chunk
	var shown strings.Builder
	for {
		if f.inBlock {
			end := strings.Index(f.pending, tutorStateCloseTag)
			if end < 0 {
				// Nothing in the block is shown; only a possible start of the
				// closing tag has to be kept.
				f.pending = f.pending[len(f.pending)-partialTagSuffixLength(f.pending, tutorStateCloseTag):]
				return shown.String()
			}
			f.pending = f.pending[end+len(tutorStateCloseTag):]
			f.inBlock = false
			continue
		}
		start := strings.Index(f.pending, tutorStateOpenTag)
		if start < 0 {
			held := partialTagSuffixLength(f.pending, tutorStateOpenTag)
			shown.WriteString(f.pending[:len(f.pending)-held])
			f.pending = f.pending[len(f.pending)-held:]
			return shown.String()
		}
		shown.WriteString(f.pending[:start])
		f.pending = f.pending[start+len(tutorStateOpenTag):]
		f.inBlock = true
	}
}

// flush returns what is still held when the stream ends.
func (f *tutorStateFilter) flush() string {
	rest := f.pending
	if f.inBlock {
		rest = ""
	}
	f.pending = ""
	f.inBlock = false
	return rest
}

// partialTagSuffixLength is the length of the longest suffix of text that is a
// proper prefix of tag (never the whole tag).
func partialTagSuffixLength(text, tag string) int {
	longest := 0
	for n := 1; n < len(tag) && n <= len(text); n++ {
		if strings.HasSuffix(text, tag[:n]) {
			longest = n
		}
	}
	return longest
}
