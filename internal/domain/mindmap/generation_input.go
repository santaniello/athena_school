package mindmap

// SourceSession is what a mind map needs from the study session it comes from.
type SourceSession struct {
	Topic string
	Goal  string
}

// GenerationProfile is what mind map generation needs from the user's profile.
type GenerationProfile struct {
	Area            string
	ExperienceLevel string
	Language        string // the assistant language
}

// NoteExcerpt is one chunk of a session's attached notes, offered to mind map
// generation as context.
type NoteExcerpt struct {
	Source  string
	Content string
}
