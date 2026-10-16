package main

import (
	"log"

	"github.com/santaniello/athena/internal/application/modelcatalog"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// newContextResolver returns the catalog service as-is unless the development
// override is on, in which case every model reports the overridden window.
// An invalid value is logged and ignored, never silently applied.
func newContextResolver(catalog *modelcatalog.Service) domainllm.ModelContextResolver {
	length, err := modelcatalog.ParseContextWindowOverride(contextWindowOverrideSetting())
	if err != nil {
		log.Printf("ignoring %s: %v", contextWindowOverrideEnv, err)
		return catalog
	}
	if length == 0 {
		return catalog
	}
	log.Printf("WARNING: %s is on: every model reports a %d-token context window", contextWindowOverrideEnv, length)
	return modelcatalog.NewOverrideResolver(catalog, length)
}
