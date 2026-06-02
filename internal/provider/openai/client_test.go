package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

func TestClientSummarizeParsesJSONSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"es\":\"saludo\",\"en\":\"greeting\",\"de\":\"begrüßung\"}"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient("sk-test", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	summary, err := client.Summarize(context.Background(), app.Transcript{Text: "hola"}, app.SummaryLanguages(), app.ModelRef{})
	if err != nil {
		t.Fatal(err)
	}
	if summary[app.LanguageSpanish] != "saludo" || summary[app.LanguageGerman] == "" {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestClientTranslateBuildsRequestAndParsesTranslations(t *testing.T) {
	var payload chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"es\":\"hola\",\"en\":\"hello\",\"de\":\"hallo\"}"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient("sk-test", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	translations, err := client.Translate(context.Background(), "hello", app.SummaryLanguages(), app.ModelRef{Name: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}

	if translations[app.LanguageSpanish] != "hola" || translations[app.LanguageEnglish] != "hello" || translations[app.LanguageGerman] != "hallo" {
		t.Fatalf("translations = %+v", translations)
	}
	if payload.Model != "gpt-test" {
		t.Fatalf("model = %q", payload.Model)
	}
	if payload.ResponseFormat == nil || payload.ResponseFormat.Type != "json_object" {
		t.Fatalf("response format = %+v", payload.ResponseFormat)
	}
	if len(payload.Messages) != 2 || !strings.Contains(payload.Messages[1].Content, "Languages: es, en, de") || !strings.Contains(payload.Messages[1].Content, "hello") {
		t.Fatalf("messages = %+v", payload.Messages)
	}
}

func TestClientTranslateMalformedJSONReturnsParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{bad json"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient("sk-test", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Translate(context.Background(), "hello", app.SummaryLanguages(), app.ModelRef{})
	if err == nil || !strings.Contains(err.Error(), "parse translation response") {
		t.Fatalf("error = %v", err)
	}
}

func TestClientErrorDoesNotLeakAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad key sk-secret", http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient("sk-secret", WithBaseURL(server.URL), WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Answer(context.Background(), app.Question("q"), app.RecentContext{}, app.ModelRef{})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error leaked api key: %v", err)
	}
}
