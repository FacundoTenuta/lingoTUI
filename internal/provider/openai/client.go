package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const defaultBaseURL = "https://api.openai.com"

var _ app.Transcriber = (*Client)(nil)
var _ app.Chat = (*Client)(nil)

type Client struct {
	apiKey     string
	baseURL    *url.URL
	httpClient *http.Client
}

type Option func(*Client)

func WithBaseURL(rawURL string) Option {
	return func(c *Client) {
		if rawURL == "" {
			return
		}
		parsed, err := url.Parse(rawURL)
		if err == nil {
			c.baseURL = parsed
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func NewClient(apiKey string, options ...Option) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("openai api key is required")
	}
	baseURL, _ := url.Parse(defaultBaseURL)
	client := &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
	for _, option := range options {
		option(client)
	}
	if client.baseURL == nil || client.baseURL.Scheme == "" || client.baseURL.Host == "" {
		return nil, errors.New("openai base url must be absolute")
	}
	return client, nil
}

func (c *Client) Transcribe(ctx context.Context, file app.AudioFile, model app.ModelRef) (app.Transcript, error) {
	if strings.TrimSpace(file.Path) == "" {
		return app.Transcript{}, errors.New("audio file path is required")
	}
	audio, err := os.Open(file.Path)
	if err != nil {
		return app.Transcript{}, fmt.Errorf("open audio file: %w", err)
	}
	defer audio.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", modelName(model, app.DefaultTranscriptionModel)); err != nil {
		return app.Transcript{}, err
	}
	part, err := writer.CreateFormFile("file", filepath.Base(file.Path))
	if err != nil {
		return app.Transcript{}, err
	}
	if _, err := io.Copy(part, audio); err != nil {
		return app.Transcript{}, err
	}
	if err := writer.Close(); err != nil {
		return app.Transcript{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/audio/transcriptions"), &body)
	if err != nil {
		return app.Transcript{}, err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	var response struct {
		Text string `json:"text"`
	}
	if err := c.doJSON(request, &response, "transcription"); err != nil {
		return app.Transcript{}, err
	}
	return app.Transcript{Text: response.Text}, nil
}

func (c *Client) Summarize(ctx context.Context, transcript app.Transcript, languages []app.Language, model app.ModelRef) (app.Summary, error) {
	languageNames := make([]string, 0, len(languages))
	for _, language := range languages {
		languageNames = append(languageNames, string(language))
	}
	content, err := c.chat(ctx, model, []chatMessage{
		{Role: "system", Content: "You summarize transcripts for language learners. Respond only with a compact JSON object whose keys are the requested language codes and whose values are concise summaries."},
		{Role: "user", Content: fmt.Sprintf("Languages: %s\nTranscript:\n%s", strings.Join(languageNames, ", "), transcript.Text)},
	}, true)
	if err != nil {
		return nil, err
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse summary response: %w", err)
	}
	summary := app.Summary{}
	for _, language := range languages {
		if text := strings.TrimSpace(raw[string(language)]); text != "" {
			summary[language] = text
		}
	}
	return summary, nil
}

func (c *Client) Answer(ctx context.Context, question app.Question, recent app.RecentContext, model app.ModelRef) (app.Answer, error) {
	content, err := c.chat(ctx, model, []chatMessage{
		{Role: "system", Content: "Answer using only the recent transcript and multilingual summary. If the answer is not present, say so briefly."},
		{Role: "user", Content: fmt.Sprintf("Transcript:\n%s\n\nSummary:\n%s\n\nQuestion: %s", recent.Transcript.Text, formatSummary(recent.Summary), question)},
	}, false)
	if err != nil {
		return "", err
	}
	return app.Answer(strings.TrimSpace(content)), nil
}

func (c *Client) Translate(ctx context.Context, text string, languages []app.Language, model app.ModelRef) (app.Translations, error) {
	languageNames := make([]string, 0, len(languages))
	for _, language := range languages {
		languageNames = append(languageNames, string(language))
	}
	content, err := c.chat(ctx, model, []chatMessage{
		{Role: "system", Content: "Translate user text for language learners. Respond only with a compact JSON object whose keys are the requested language codes and whose values are direct translations."},
		{Role: "user", Content: fmt.Sprintf("Languages: %s\nText:\n%s", strings.Join(languageNames, ", "), text)},
	}, true)
	if err != nil {
		return nil, err
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse translation response: %w", err)
	}
	translations := app.Translations{}
	for _, language := range languages {
		if translated := strings.TrimSpace(raw[string(language)]); translated != "" {
			translations[language] = translated
		}
	}
	return translations, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *Client) chat(ctx context.Context, model app.ModelRef, messages []chatMessage, jsonObject bool) (string, error) {
	payload := chatRequest{Model: modelName(model, app.DefaultChatModel), Messages: messages}
	if jsonObject {
		payload.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/v1/chat/completions"), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")

	var response chatResponse
	if err := c.doJSON(request, &response, "chat"); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", errors.New("openai chat response had no choices")
	}
	return response.Choices[0].Message.Content, nil
}

func (c *Client) doJSON(request *http.Request, target interface{}, operation string) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("openai %s request: %w", operation, err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read openai %s response: %w", operation, err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("openai %s request failed: status %d: %s", operation, response.StatusCode, sanitizeProviderMessage(string(data), c.apiKey))
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode openai %s response: %w", operation, err)
	}
	return nil
}

func (c *Client) endpoint(path string) string {
	copy := *c.baseURL
	copy.Path = strings.TrimRight(copy.Path, "/") + path
	return copy.String()
}

func modelName(model app.ModelRef, fallback string) string {
	if strings.TrimSpace(model.Name) == "" {
		return fallback
	}
	return model.Name
}

func formatSummary(summary app.Summary) string {
	var builder strings.Builder
	for _, language := range app.SummaryLanguages() {
		if text := strings.TrimSpace(summary[language]); text != "" {
			fmt.Fprintf(&builder, "%s: %s\n", strings.ToUpper(string(language)), text)
		}
	}
	return builder.String()
}

func sanitizeProviderMessage(message, apiKey string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return http.StatusText(http.StatusInternalServerError)
	}
	message = strings.ReplaceAll(message, apiKey, "[redacted]")
	if len(message) > 512 {
		message = message[:512] + "..."
	}
	return message
}
