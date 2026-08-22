package study

import "errors"

// ErrTopicRequired is returned by Start when the topic is blank.
var ErrTopicRequired = errors.New("topic is required")

// ErrMessageRequired is returned by SendMessage when the content is blank.
var ErrMessageRequired = errors.New("message content is required")

// ErrStudyTurnInProgress is returned by SendMessage/RequestOpeningTurn when
// a turn is already running for the given session (see
// inFlightCoordinator). Different sessions never contend with each other.
var ErrStudyTurnInProgress = errors.New("study: a turn is already in progress for this session")
