package study

// OfferKind says which kind of offer a tutor reply carries: the study chat can
// offer an architecture exercise or an interactive quiz. It lives here, in the
// chat's own domain, so the chat port and its mock share it without importing
// the exercise or the quiz context.
type OfferKind string

// The offer kinds. OfferKindNone means the reply carries no offer.
const (
	OfferKindNone     OfferKind = ""
	OfferKindExercise OfferKind = "exercise"
	OfferKindQuiz     OfferKind = "quiz"
)
