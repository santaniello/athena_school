package ingest

import "strings"

// maxFormattingChars bounds how much raw source text one formatting Chat
// call is asked to rewrite. A budget, not a hard ceiling: a lone heading
// section (or paragraph, for a heading-less document) larger than this on
// its own still becomes its own, over-budget batch — formatting never
// splits a section mid-way, the same "never drop/split" philosophy
// packParagraphs/mergeUndersized already use for chunking. Keeping most
// notes at one or two batches while still bounding any single Chat call is
// what makes a timeout's blast radius one section instead of the whole
// document.
const maxFormattingChars = 6000

// FormattingSection is one deterministically-cut, heading- or paragraph-
// scoped slice of a document's raw source text, before being grouped into
// a FormattingBatch for formatting. Breadcrumb is the full heading
// breadcrumb enclosing this section (see findHeadingSections), or "" for
// text with no enclosing heading — the same shape ChunkCandidate.Heading
// uses, since both ultimately walk the same heading boundaries.
type FormattingSection struct {
	Breadcrumb string
	Content    string
	Start, End int
}

// FormattingBatch is one or more adjacent FormattingSections grouped under
// maxFormattingChars, formatted by exactly one Chat call. Start/End are
// Sections[0].Start and Sections[last].End — the batch's own span into the
// original document source.
type FormattingBatch struct {
	Sections   []FormattingSection
	Start, End int
}

// splitIntoFormattingSections cuts source into the same heading-bounded
// slices findHeadingSections produces for chunking, carrying each
// section's full breadcrumb — but, unlike chunking, never packs paragraphs
// or merges undersized pieces: formatting batches group sections by
// character budget (see packFormattingBatches), not by a minimum size.
// When source has no H1-H3 heading at all (including every .txt source),
// it falls back to paragraph-scoped sections with an empty breadcrumb, so
// a large heading-less document still gets split into more than one
// formatting call instead of becoming a single oversized one.
func splitIntoFormattingSections(source string) []FormattingSection {
	raw := findHeadingSections(source)
	if raw != nil {
		sections := make([]FormattingSection, len(raw))
		for i, c := range raw {
			sections[i] = FormattingSection{Breadcrumb: c.Heading, Content: c.Content, Start: c.Start, End: c.End}
		}
		return sections
	}

	paragraphs := splitParagraphs(source)
	sections := make([]FormattingSection, len(paragraphs))
	for i, p := range paragraphs {
		sections[i] = FormattingSection{Breadcrumb: "", Content: p.Text, Start: p.Start, End: p.End}
	}
	return sections
}

// packFormattingBatches greedily groups adjacent sections into batches
// under maxFormattingChars, in document order. A section is only ever
// added to the batch currently being built; once adding it would push that
// batch over budget, the batch closes and a new one starts with that
// section — so a single section already over budget on its own still
// becomes its own (over-budget) batch, never split across two.
func packFormattingBatches(sections []FormattingSection) []FormattingBatch {
	if len(sections) == 0 {
		return nil
	}

	var batches []FormattingBatch
	var current []FormattingSection
	currentLen := 0
	for _, s := range sections {
		sLen := runeLen(s.Content)
		if len(current) > 0 && currentLen+sLen > maxFormattingChars {
			batches = append(batches, newFormattingBatch(current))
			current = nil
			currentLen = 0
		}
		current = append(current, s)
		currentLen += sLen
	}
	if len(current) > 0 {
		batches = append(batches, newFormattingBatch(current))
	}
	return batches
}

func newFormattingBatch(sections []FormattingSection) FormattingBatch {
	return FormattingBatch{
		Sections: sections,
		Start:    sections[0].Start,
		End:      sections[len(sections)-1].End,
	}
}

// breadcrumb returns the batch's representative section breadcrumb — its
// first section's — used as the formatting call's section-context hint.
// A batch spanning several sections under different headings still only
// needs one representative hint: the model is told it is formatting an
// excerpt of a larger note, not asked to resolve per-paragraph headings
// itself.
func (b FormattingBatch) breadcrumb() string {
	if len(b.Sections) == 0 {
		return ""
	}
	return b.Sections[0].Breadcrumb
}

// leafHeading returns breadcrumb's own last segment — the heading that
// actually opened this section, as opposed to any ancestor heading also
// folded into the breadcrumb.
func leafHeading(breadcrumb string) string {
	if idx := strings.LastIndex(breadcrumb, " > "); idx >= 0 {
		return breadcrumb[idx+len(" > "):]
	}
	return breadcrumb
}

// headings returns the batch's own sections' leaf headings (see
// leafHeading), skipping any section with no heading at all —
// validateFormattingFidelity checks these survive formatting untranslated.
func (b FormattingBatch) headings() []string {
	var out []string
	for _, s := range b.Sections {
		if s.Breadcrumb == "" {
			continue
		}
		out = append(out, leafHeading(s.Breadcrumb))
	}
	return out
}

// BatchOutcome is one batch's formatting result: either the model's
// rewrite, or Original unchanged when Fallback is true. Validation is the
// deterministic fidelity verdict that accepted (or rejected) it, and
// Structure is the chunking-health verdict computed on whichever text was
// ultimately accepted — both zero-valued when the batch fell back before
// they ever ran (e.g. a Provider error).
type BatchOutcome struct {
	Batch      FormattingBatch
	Original   string
	Formatted  string
	Fallback   bool
	Validation BatchValidation
	Structure  StructureCheck
}

// reassembleFormattedContent concatenates every outcome's Formatted text
// in document order, re-inserting the exact original gap text between
// batches (source[prevBatch.End:nextBatch.Start] — whatever whitespace
// separated them, verbatim) so that when every batch falls back
// (Formatted == Original == source[Batch.Start:Batch.End]), the result is
// byte-identical to source sliced from the first batch's Start to the last
// batch's End — the "reviewed text is exactly the persisted text"
// invariant surviving section-by-section formatting. Content before the
// first batch's Start or after the last batch's End is never included —
// the same leading/trailing whitespace findHeadingSections/splitParagraphs
// already trim away. The gap bounds are always valid in practice (both
// come from batches built from this same source), but are checked
// explicitly rather than trusted blindly before slicing.
func reassembleFormattedContent(source string, outcomes []BatchOutcome) string {
	var b strings.Builder
	prevEnd, hasPrev := 0, false
	for _, outcome := range outcomes {
		if hasPrev {
			gapStart, gapEnd := prevEnd, outcome.Batch.Start
			if gapStart >= 0 && gapEnd <= len(source) && gapStart <= gapEnd {
				b.WriteString(source[gapStart:gapEnd])
			}
		}
		b.WriteString(outcome.Formatted)
		prevEnd, hasPrev = outcome.Batch.End, true
	}
	return b.String()
}
