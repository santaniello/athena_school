package modelcatalog

import "errors"

// ErrEmptyCatalog is returned when a catalog load succeeds at the HTTP
// level but leaves no valid entries once blank IDs, non-positive lengths
// and conflicting duplicates are excluded. The previous cache (if any) is
// preserved rather than replaced by an empty one.
var ErrEmptyCatalog = errors.New("modelcatalog: no valid models in catalog response")
