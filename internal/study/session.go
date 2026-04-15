package study

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fsantaniello/athena_school/internal/platform/llm"
)

// Session drives a single guided study interaction for one topic.
type Session struct {
	topic    string
	subtopic string
	model    string
	provider llm.LLMProvider
	scanner  *bufio.Scanner
	out      io.Writer
}

// NewSession constructs a Session. in and out allow injection in tests.
// A single bufio.Scanner is shared across all input reads to avoid buffering conflicts.
func NewSession(topic, subtopic, model string, provider llm.LLMProvider, in io.Reader, out io.Writer) *Session {
	return &Session{
		topic:    topic,
		subtopic: subtopic,
		model:    model,
		provider: provider,
		scanner:  bufio.NewScanner(in),
		out:      out,
	}
}

// Run executes the full study loop until the user exits or chooses not to continue.
func (s *Session) Run(ctx context.Context) error {
	history := []llm.ChatMessage{}

	for {
		s.printHeader()

		explanation, history, err := s.fetchExplanation(ctx, history)
		if err != nil {
			return err
		}
		fmt.Fprintln(s.out, explanation)

		question, history, err := s.fetchQuestion(ctx, history)
		if err != nil {
			return err
		}
		fmt.Fprintf(s.out, "\n❓ Question:\n%s\n\n", question)

		fmt.Fprintln(s.out, "Your answer (press Enter on a blank line to submit):")
		answer, err := readMultilineAnswer(s.scanner)
		if err != nil {
			return err
		}
		if answer == "exit" {
			return nil
		}

		evaluation, err := s.fetchEvaluation(ctx, history, question, answer)
		if err != nil {
			return err
		}
		fmt.Fprintln(s.out, evaluation)

		if !s.askContinue() {
			return nil
		}
	}
}

func (s *Session) printHeader() {
	label := topicLabel(s.topic, s.subtopic)
	separator := strings.Repeat("━", 8)
	fmt.Fprintf(s.out, "\n%s Study: %s %s\n\n", separator, label, separator)
}

func (s *Session) fetchExplanation(ctx context.Context, history []llm.ChatMessage) (string, []llm.ChatMessage, error) {
	prompt, err := BuildExplanationPrompt(s.topic, s.subtopic)
	if err != nil {
		return "", history, err
	}
	return s.chatWithSpinner(ctx, "Generating explanation...", prompt, history)
}

func (s *Session) fetchQuestion(ctx context.Context, history []llm.ChatMessage) (string, []llm.ChatMessage, error) {
	prompt, err := BuildQuestionPrompt(topicLabel(s.topic, s.subtopic))
	if err != nil {
		return "", history, err
	}
	return s.chatWithSpinner(ctx, "Generating question...", prompt, history)
}

func (s *Session) fetchEvaluation(ctx context.Context, history []llm.ChatMessage, question, answer string) (string, error) {
	prompt, err := BuildEvaluationPrompt(topicLabel(s.topic, s.subtopic), question, answer)
	if err != nil {
		return "", err
	}
	content, _, err := s.chatWithSpinner(ctx, "Evaluating answer...", prompt, history)
	return content, err
}

// chatWithSpinner appends a user message to history, shows a spinner while calling the LLM,
// then appends the assistant reply and returns the updated history alongside the response content.
func (s *Session) chatWithSpinner(ctx context.Context, label, prompt string, history []llm.ChatMessage) (string, []llm.ChatMessage, error) {
	history = append(history, llm.ChatMessage{Role: "user", Content: prompt})

	var content string
	err := withSpinner(s.out, label, func() error {
		resp, chatErr := s.provider.Chat(ctx, llm.ChatRequest{
			Model:    s.model,
			Messages: history,
		})
		if chatErr != nil {
			return chatErr
		}
		content = resp.Content
		return nil
	})
	if err != nil {
		return "", history, err
	}

	history = append(history, llm.ChatMessage{Role: "assistant", Content: content})
	return content, history, nil
}

func (s *Session) askContinue() bool {
	fmt.Fprint(s.out, "\nContinue? [y/n]: ")
	if s.scanner.Scan() {
		return strings.TrimSpace(strings.ToLower(s.scanner.Text())) == "y"
	}
	return false
}

// readMultilineAnswer reads lines from sc until a blank line,
// a "." on its own line, or "exit" is encountered.
// A blank line signals the user pressed Enter twice (once to end the last line, once to submit).
func readMultilineAnswer(sc *bufio.Scanner) (string, error) {
	var lines []string

	for sc.Scan() {
		line := sc.Text()
		if line == "exit" {
			return "exit", nil
		}
		if line == "." || line == "" {
			break
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// withSpinner starts a spinner writing to out, runs fn, then stops the spinner.
// The spinner is always stopped via defer to prevent goroutine leaks.
func withSpinner(out io.Writer, label string, fn func() error) error {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(out))
	s.Suffix = " " + label
	s.Start()
	defer s.Stop()
	return fn()
}
