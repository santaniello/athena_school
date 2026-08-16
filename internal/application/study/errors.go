package study

import "errors"

// ErrTopicRequired is returned by Start when the topic is blank.
var ErrTopicRequired = errors.New("topic is required")

// ErrMessageRequired is returned by SendMessage when the content is blank.
var ErrMessageRequired = errors.New("message content is required")
