package study

import "context"

// OfferGate connects the chat to the exercise offers without importing any lab
// code. It is defined here (consumer side) and implemented by the composition
// root on top of the lab's offer service.
type OfferGate interface {
	// HasPending reports whether the session has an offer waiting for an answer.
	HasPending(ctx context.Context, sessionID string) (bool, error)
	// CreateOffer stores a pending offer under the assistant message.
	CreateOffer(ctx context.Context, sessionID, messageID string) error
}

// WithOfferGate wires the exercise offers into the chat. Without it the chat
// never creates an offer and never blocks on one.
func WithOfferGate(gate OfferGate) Option {
	return func(s *Service) { s.offers = gate }
}
