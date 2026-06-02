package chatgptcodex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

var _ app.Chat = (*Chat)(nil)

type ResponseCreator interface {
	CreateResponse(context.Context, ResponseRequest) (Response, error)
}

type Chat struct {
	Client ResponseCreator
}

func NewChat(client ResponseCreator) *Chat {
	return &Chat{Client: client}
}

func (c *Chat) Summarize(ctx context.Context, transcript app.Transcript, languages []app.Language, model app.ModelRef) (app.Summary, error) {
	languageCodes := make([]string, 0, len(languages))
	for _, language := range languages {
		languageCodes = append(languageCodes, string(language))
	}

	response, err := c.create(ctx, ResponseRequest{
		Model: strings.TrimSpace(model.Name),
		Messages: []Message{
			{Role: "system", Text: "You summarize transcripts for language learners. Respond only with a compact JSON object whose keys are the requested language codes and whose values are concise summaries."},
			{Role: "user", Text: fmt.Sprintf("Languages: %s\nTranscript:\n%s", strings.Join(languageCodes, ", "), transcript.Text)},
		},
	})
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(response.Text)
	if text == "" {
		return nil, errors.New("parse ChatGPT Codex summary response: response contained no text")
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("parse ChatGPT Codex summary response: %w", err)
	}

	summary := app.Summary{}
	for _, language := range languages {
		if value := strings.TrimSpace(raw[string(language)]); value != "" {
			summary[language] = value
		}
	}
	return summary, nil
}

func (c *Chat) Answer(ctx context.Context, question app.Question, recent app.RecentContext, model app.ModelRef) (app.Answer, error) {
	response, err := c.create(ctx, ResponseRequest{
		Model: strings.TrimSpace(model.Name),
		Messages: []Message{
			{Role: "system", Text: "Answer using only the recent transcript and multilingual summary. If the answer is not present, say so briefly."},
			{Role: "user", Text: fmt.Sprintf("Transcript:\n%s\n\nSummary:\n%s\n\nQuestion: %s", recent.Transcript.Text, formatSummary(recent.Summary), question)},
		},
	})
	if err != nil {
		return "", err
	}
	return app.Answer(strings.TrimSpace(response.Text)), nil
}

func (c *Chat) Translate(ctx context.Context, text string, languages []app.Language, model app.ModelRef) (app.Translations, error) {
	languageCodes := make([]string, 0, len(languages))
	for _, language := range languages {
		languageCodes = append(languageCodes, string(language))
	}

	response, err := c.create(ctx, ResponseRequest{
		Model: strings.TrimSpace(model.Name),
		Messages: []Message{
			{Role: "system", Text: "Translate user text for language learners. Respond only with a compact JSON object whose keys are the requested language codes and whose values are direct translations."},
			{Role: "user", Text: fmt.Sprintf("Languages: %s\nText:\n%s", strings.Join(languageCodes, ", "), text)},
		},
	})
	if err != nil {
		return nil, err
	}

	responseText := strings.TrimSpace(response.Text)
	if responseText == "" {
		return nil, errors.New("parse ChatGPT Codex translation response: response contained no text")
	}
	var raw map[string]string
	if err := json.Unmarshal([]byte(responseText), &raw); err != nil {
		return nil, fmt.Errorf("parse ChatGPT Codex translation response: %w", err)
	}

	translations := app.Translations{}
	for _, language := range languages {
		if value := strings.TrimSpace(raw[string(language)]); value != "" {
			translations[language] = value
		}
	}
	return translations, nil
}

func (c *Chat) create(ctx context.Context, req ResponseRequest) (Response, error) {
	if c == nil || c.Client == nil {
		return Response{}, errors.New("ChatGPT Codex chat client is required")
	}
	response, err := c.Client.CreateResponse(ctx, req)
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return Response{}, contextErr
		}
		return Response{}, errors.New("create ChatGPT Codex response")
	}
	return response, nil
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
