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
