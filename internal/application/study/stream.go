package study

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// unavailableContextMessage is the transient technical notice shown when a
// session's context limit could not be determined — never persisted state.
const unavailableContextMessage = "Unable to determine this session's context limit."

// streamAndPersist sends messages to the LLM, forwarding every chunk to
// onChunk as it arrives. Only once the stream completes successfully is the
// full assistant reply persisted — atomically with the turn's new
// ContextUsage measurement (real usage when the provider reported it,
// otherwise a conservative per-message estimate) — as a single message;
// never partial content, so a mid-stream failure leaves no assistant row
// (the user's turn it replied to, if any, is already safely persisted).
// priorContext is the session's ContextUsage going into this call, used both
// to seed the monotonicity check in domainstudy.NextContextUsage and to
// detect whether the resolved model/context length changed. sources are the
// local-knowledge Sources (if any) that backed this reply — persisted
// atomically with the message itself so they survive a resume (see
// specs/phases/phase-02-knowledge-engine/09-persistent-provenance.md); an
// empty/nil slice is a valid no-op (a strict-notes miss, or the opening
// turn, which never retrieves).
//
// The model may end a reply with the exercise offer marker (see
// offer_marker.go): the user never sees it, not while streaming and not
// after, and the stored text is clean. When detectOffer is set (a reply to
// the user's own message) and the marker was there, an offer is created under
// the stored message once the reply is safely persisted; failing to create it
// never fails the reply.
func (s *Service) streamAndPersist(
	ctx context.Context, sessionID string, priorContext domainstudy.ContextUsage,
	messages []domainllm.Message, sources []domainknowledge.Source, detectOffer bool, onChunk func(chunk string) error,
	onContext ContextCallback, onContextUnavailable ContextUnavailableCallback,
) (string, error) {
	var buf strings.Builder
	var filter offerMarkerFilter
	streamResp, err := s.llm.ChatStream(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskStudy,
		Messages:  messages,
	}, func(chunk string) error {
		buf.WriteString(chunk)
		if safe := filter.write(chunk); safe != "" {
			return onChunk(safe)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if held := filter.flush(); held != "" {
		if err := onChunk(held); err != nil {
			return "", err
		}
	}
	content, offered := stripExerciseOfferMarker(buf.String())

	assistantMessage := domainstudy.Message{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Role:      domainstudy.RoleAssistant,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}

	usedTokens, estimated := measureUsage(streamResp, messages, assistantMessage.Content)
	contextLength, known := s.resolveContextLength(streamResp.Model)

	var newUsage domainstudy.ContextUsage
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.messages.Append(ctx, assistantMessage); err != nil {
			return err
		}
		if err := s.messageSources.Save(ctx, assistantMessage.ID, sources); err != nil {
			return err
		}
		length := 0
		switch {
		case known:
			length = contextLength
		case priorContext.Model == streamResp.Model:
			// A cache miss for the same model that was already resolved
			// (e.g. right after an app restart, before the catalog has
			// finished warming up) must not erase the window already known
			// for it — reuse it rather than reporting unresolved.
			length = priorContext.ContextLength
		}
		newUsage = domainstudy.NextContextUsage(priorContext, streamResp.Model, usedTokens, length, estimated)
		return s.sessions.UpdateContext(ctx, sessionID, newUsage)
	})
	if err != nil {
		return "", fmt.Errorf("study: persisting assistant reply: %w", err)
	}

	emitContextTransition(priorContext, newUsage, onContext)

	if detectOffer && offered && s.offers != nil {
		// Best effort: a missed offer is a normal reply, and the user can ask again.
		_ = s.offers.CreateOffer(ctx, sessionID, assistantMessage.ID)
	}

	switch {
	case streamResp.Model == "":
		if onContextUnavailable != nil {
			onContextUnavailable(unavailableContextMessage)
		}
	case !known:
		s.resolveContextLengthInBackground(ctx, sessionID, streamResp.Model, newUsage, onContext, onContextUnavailable)
	}

	return assistantMessage.Content, nil
}

// resolveContextLength looks up model's context length from the in-memory
// catalog cache only — never performs I/O, so it never delays a completed
// response.
func (s *Service) resolveContextLength(model string) (length int, known bool) {
	if model == "" {
		return 0, false
	}
	return s.catalog.CachedContextLength(model)
}

// resolveContextLengthInBackground refreshes the model catalog and, if it
// now resolves model, applies the newly known context length to sessionID —
// but only if the session's persisted ContextUsage still matches snapshot
// (the measurement that triggered this refresh); otherwise the result is
// stale and must not overwrite a newer measurement. Runs in its own
// goroutine against ctx, which is the application-lifetime context (see
// internal/interfaces/desktop.App.ctx) — navigation away from the screen
// that triggered it does not cancel it. Reused by streamAndPersist and
// Resume.
func (s *Service) resolveContextLengthInBackground(
	ctx context.Context, sessionID, model string, snapshot domainstudy.ContextUsage,
	onContext ContextCallback, onContextUnavailable ContextUnavailableCallback,
) {
	go func() {
		length, err := s.catalog.RefreshContextLength(ctx, model)
		if err != nil || length <= 0 {
			if onContextUnavailable != nil {
				onContextUnavailable(unavailableContextMessage)
			}
			return
		}

		var newUsage domainstudy.ContextUsage
		applied := false
		txErr := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			current, err := s.sessions.GetByID(ctx, sessionID)
			if err != nil {
				return err
			}
			if current.Context != snapshot {
				return nil // stale: a newer measurement already landed.
			}
			newUsage = domainstudy.NextContextUsage(current.Context, model, current.Context.UsedTokens, length, current.Context.Estimated)
			applied = true
			return s.sessions.UpdateContext(ctx, sessionID, newUsage)
		})
		if txErr != nil || !applied {
			return
		}
		emitContextTransition(snapshot, newUsage, onContext)
	}()
}
