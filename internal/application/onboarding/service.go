// Package onboarding holds the onboarding use cases: saving the OpenRouter
// key gate and saving the collected UserProfile. See
// specs/phases/phase-01-desktop-mvp/04-onboarding.md.
package onboarding

import (
	domainconfig "github.com/santaniello/athena/internal/domain/config"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
)

// Service implements the onboarding use cases against a
// domainprofile.Store, a domainconfig.Store and a domainconfig.KeyValidator.
type Service struct {
	profiles  domainprofile.Store
	config    domainconfig.Store
	validator domainconfig.KeyValidator
}

// NewService creates a Service backed by the given ports.
func NewService(profiles domainprofile.Store, config domainconfig.Store, validator domainconfig.KeyValidator) *Service {
	return &Service{profiles: profiles, config: config, validator: validator}
}
