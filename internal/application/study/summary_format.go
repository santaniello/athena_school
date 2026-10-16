package study

import (
	"fmt"
	"strings"
)

// summaryHeadings are the section titles every summary must contain, exactly
// once each and in this order.
var summaryHeadings = []string{"## Progress", "## Open difficulties", "## Pending", "## Discussion log"}

// validateSummaryFormat checks that content has each of summaryHeadings as a
// line of its own, exactly once and in order. Anything else is allowed in the
// section bodies. It returns an error wrapping ErrInvalidSummaryFormat.
func validateSummaryFormat(content string) error {
	next := 0
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, " \t\r")
		for i, heading := range summaryHeadings {
			if line != heading {
				continue
			}
			if i != next {
				return fmt.Errorf("%w: %q is duplicated or out of order", ErrInvalidSummaryFormat, heading)
			}
			next++
		}
	}
	if next != len(summaryHeadings) {
		return fmt.Errorf("%w: missing %q", ErrInvalidSummaryFormat, summaryHeadings[next])
	}
	return nil
}
