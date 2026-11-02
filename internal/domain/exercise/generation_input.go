package exercise

// GenerationProfile is what challenge generation needs from the user's
// profile.
type GenerationProfile struct {
	Area            string
	ExperienceLevel string
	Language        string // the assistant language
}

// NoteExcerpt is one chunk of a session's attached notes, offered to
// challenge generation as context.
type NoteExcerpt struct {
	Source  string
	Content string
}
