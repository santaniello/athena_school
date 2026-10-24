package settings

import "errors"

var (
	// ErrUnknownModel is returned when a chosen model ID is not in the
	// provider's catalog.
	ErrUnknownModel = errors.New("settings: model is not in the catalog")
	// ErrCatalogUnavailable is returned when the catalog cannot be fetched,
	// so a chosen model cannot be verified. Saving is refused rather than
	// trusting an unverified ID.
	ErrCatalogUnavailable = errors.New("settings: model catalog is unavailable")
)
