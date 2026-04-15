package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Message is a single conversation turn sent to or received from Ollama.
type Message struct {
	Role    string
	Content string
}

// Request is the input to Client.Chat.
type Request struct {
	Model    string
	Messages []Message
	Stream   bool
}

// Response is the output from Client.Chat.
type Response struct {
	Content string
}

// Client is an HTTP client for the Ollama /api/chat endpoint.
type Client struct {
	host       string
	httpClient *http.Client
	out        io.Writer
}

// New creates a Client with the given base URL, writing streamed tokens to os.Stdout.
func New(host string) *Client {
	return &Client{
		host:       host,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		out:        os.Stdout,
	}
}

// newWithWriter creates a Client that writes streamed tokens to out. Used in tests.
func newWithWriter(host string, out io.Writer) *Client {
	return &Client{
		host:       host,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		out:        out,
	}
}

// wire types — used only for JSON serialization with the Ollama API.

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

// Chat sends a request to the Ollama /api/chat endpoint and returns the response.
func (c *Client) Chat(ctx context.Context, req Request) (Response, error) {
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}

	body, err := json.Marshal(ollamaRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	})
	if err != nil {
		return Response{}, fmt.Errorf("could not encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("could not build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, c.classifyNetworkError(err, req.Model)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Response{}, fmt.Errorf("model %q not found in Ollama — run: ollama pull %s", req.Model, req.Model)
	}

	if req.Stream {
		return c.readStreaming(resp.Body)
	}
	return readNonStreaming(resp.Body)
}

func (c *Client) classifyNetworkError(err error, _ string) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if errors.Is(urlErr.Err, context.DeadlineExceeded) {
			return fmt.Errorf("request timed out after 120s")
		}
		if strings.Contains(urlErr.Err.Error(), "connection refused") {
			return fmt.Errorf("could not reach Ollama at %s — is it running?", c.host)
		}
	}
	return err
}

func readNonStreaming(body io.Reader) (Response, error) {
	var resp ollamaResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("could not decode response: %w", err)
	}
	return Response{Content: resp.Message.Content}, nil
}

func (c *Client) readStreaming(body io.Reader) (Response, error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Bytes()
		var chunk ollamaResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			return Response{}, fmt.Errorf("could not decode stream chunk: %w", err)
		}
		if chunk.Message.Content != "" {
			fmt.Fprint(c.out, chunk.Message.Content)
			sb.WriteString(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("error reading stream: %w", err)
	}
	return Response{Content: sb.String()}, nil
}
