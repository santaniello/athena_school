package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApp_starts(t *testing.T) {
	assert.NotPanics(t, main)
}