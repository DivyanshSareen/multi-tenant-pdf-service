// Package ai provides LLM-backed document summarization.
// The Summarizer interface allows swapping OpenAI for Ollama via config.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/divyansh/multi-tenant-pdf-service/internal/config"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

const (
	// maxTextChars is the character limit before truncation to avoid exceeding token limits.
	maxTextChars = 15000

	systemPrompt = "You are a document summarizer. Provide a concise summary of the following document in 3-5 paragraphs. Focus on the main topics, key findings, and important conclusions."
)

// Summarizer is the interface for LLM-backed text summarization.
type Summarizer interface {
	Summarize(ctx context.Context, text string) (string, error)
}

// NewSummarizer returns an OpenAI or Ollama summarizer based on the provider config.
func NewSummarizer(cfg config.LLMConfig, log *logrus.Logger) Summarizer {
	switch strings.ToLower(cfg.Provider) {
	case "ollama":
		log.WithField("model", cfg.OllamaModel).Info("using ollama summarizer")
		return &OllamaSummarizer{
			baseURL: cfg.OllamaBaseURL,
			model:   cfg.OllamaModel,
			log:     log,
		}
	default:
		log.WithField("model", cfg.OpenAIModel).Info("using openai summarizer")
		return &OpenAISummarizer{
			client: openai.NewClient(cfg.OpenAIAPIKey),
			model:  cfg.OpenAIModel,
			log:    log,
		}
	}
}

// --- OpenAI implementation ---

// OpenAISummarizer calls the OpenAI Chat Completions API.
type OpenAISummarizer struct {
	client *openai.Client
	model  string
	log    *logrus.Logger
}

// Summarize sends the document text to OpenAI and returns a 3-5 paragraph summary.
func (s *OpenAISummarizer) Summarize(ctx context.Context, text string) (string, error) {
	truncated := truncate(text)

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: truncated},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai chat completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}

	summary := resp.Choices[0].Message.Content
	s.log.WithFields(logrus.Fields{
		"model":          s.model,
		"prompt_tokens":  resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
	}).Info("openai summarization complete")

	return summary, nil
}

// --- Ollama implementation ---

// OllamaSummarizer calls a locally-running Ollama HTTP API.
type OllamaSummarizer struct {
	baseURL string
	model   string
	log     *logrus.Logger
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

// Summarize posts to the Ollama /api/generate endpoint and returns the model's response.
func (s *OllamaSummarizer) Summarize(ctx context.Context, text string) (string, error) {
	truncated := truncate(text)
	prompt := systemPrompt + "\n\nDocument:\n" + truncated

	body, err := json.Marshal(ollamaRequest{
		Model:  s.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("building ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}

	s.log.WithField("model", s.model).Info("ollama summarization complete")
	return ollamaResp.Response, nil
}

// truncate caps text at maxTextChars and appends a note so the LLM knows it was cut.
func truncate(text string) string {
	if len(text) <= maxTextChars {
		return text
	}
	return text[:maxTextChars] + "\n\n[Note: document was truncated due to length. Summary is based on the first portion of the text.]"
}
