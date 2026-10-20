package study

import "errors"

// ErrTopicRequired is returned by Start when the topic is blank.
var ErrTopicRequired = errors.New("topic is required")

// ErrGoalRequired is returned by Start when the goal is blank.
var ErrGoalRequired = errors.New("goal is required")

// ErrFolderRequired is returned by Start when no folder is chosen.
var ErrFolderRequired = errors.New("folder is required")

// ErrMessageRequired is returned by SendMessage when the content is blank.
var ErrMessageRequired = errors.New("message content is required")

// ErrStudyTurnInProgress is returned by SendMessage/RequestOpeningTurn when
// a turn is already running for the given session (see
// inFlightCoordinator). Different sessions never contend with each other.
var ErrStudyTurnInProgress = errors.New("study: a turn is already in progress for this session")

// ErrNoMessagesToSummarize is returned by Summarizer.Summarize when it is
// given no messages to fold.
var ErrNoMessagesToSummarize = errors.New("study: no messages to summarize")

// ErrSummaryTooLarge is returned by Summarizer.Summarize when the summary is
// still over the hard size cap after one condensing pass.
var ErrSummaryTooLarge = errors.New("study: conversation summary exceeds the size cap")

// ErrInvalidSummaryInput is returned by Summarizer.Summarize when the session
// id is empty, or the previous summary or any message is missing its id or
// belongs to another session. It is checked before any external call so
// memory never crosses sessions.
var ErrInvalidSummaryInput = errors.New("study: summary input does not belong to the session")

// ErrInvalidSummaryFormat is returned by Summarizer.Summarize when the model's
// summary does not contain the four fixed sections exactly once and in order.
// The call is not retried; the caller keeps the previous summary.
var ErrInvalidSummaryFormat = errors.New("study: conversation summary does not have the required sections")

// ErrNothingToCompact is returned by CompactSession when there is nothing
// that could be folded into the summary (or the context window is unknown).
var ErrNothingToCompact = errors.New("study: nothing to compact")
