package chatgptcodex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FacundoTenuta/lingoTUI/internal/app"
)

const defaultBaseURL = "https://chatgpt.com"

type AccessTokenProvider interface {
	AccessToken(context.Context) (app.Secret, error)
}

type AccountIDProvider interface {
	AccountID(context.Context) (string, error)
}

type Client struct {
	tokens     AccessTokenProvider
	accounts   AccountIDProvider
	baseURL    *url.URL
	httpClient *http.Client
}

type Option func(*Client)

type Message struct {
	Role string
	Text string
}

type ResponseRequest struct {
	Model    string
	Messages []Message
}

type Response struct {
	Text string
}

func NewClient(tokens AccessTokenProvider, opts ...Option) (*Client, error) {
	if tokens == nil {
		return nil, errors.New("chatgpt codex access token provider is required")
	}
	baseURL, _ := url.Parse(defaultBaseURL)
	client := &Client{
		tokens:  tokens,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(client)
	}
	if client.baseURL == nil || client.baseURL.Scheme == "" || client.baseURL.Host == "" {
		return nil, errors.New("chatgpt codex base url must be absolute")
	}
	return client, nil
}

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

func WithAccountIDProvider(accounts AccountIDProvider) Option {
	return func(c *Client) {
		c.accounts = accounts
	}
}

func (c *Client) CreateResponse(ctx context.Context, req ResponseRequest) (Response, error) {
	if strings.TrimSpace(req.Model) == "" {
		return Response{}, errors.New("chatgpt codex model is required")
	}
	if len(req.Messages) == 0 {
		return Response{}, errors.New("chatgpt codex messages are required")
	}

	input := make([]codexInputMessage, 0, len(req.Messages))
	for _, message := range req.Messages {
		if strings.TrimSpace(message.Text) == "" {
			return Response{}, errors.New("chatgpt codex message text is required")
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		input = append(input, codexInputMessage{
			Role: role,
			Content: []codexInputContent{{
				Type: "input_text",
				Text: message.Text,
			}},
		})
	}

	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return Response{}, contextErr
		}
		return Response{}, errors.New("get ChatGPT access token")
	}
	if token.Empty() {
		return Response{}, errors.New("chatgpt codex access token is required")
	}

	var accountID string
	if c.accounts != nil {
		accountID, err = c.accounts.AccountID(ctx)
		if err != nil {
			if contextErr := cleanContextError(err); contextErr != nil {
				return Response{}, contextErr
			}
			return Response{}, errors.New("get ChatGPT account id")
		}
		accountID = strings.TrimSpace(accountID)
	}

	payload := codexRequest{
		Model:  strings.TrimSpace(req.Model),
		Input:  input,
		Stream: false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Response{}, errors.New("encode ChatGPT Codex request")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("/backend-api/codex/responses"), bytes.NewReader(data))
	if err != nil {
		return Response{}, errors.New("create ChatGPT Codex request")
	}
	request.Header.Set("Authorization", "Bearer "+token.Value)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("originator", "lingotui")
	if accountID != "" {
		request.Header.Set("ChatGPT-Account-Id", accountID)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return Response{}, contextErr
		}
		return Response{}, errors.New("send ChatGPT Codex request")
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		if contextErr := cleanContextError(err); contextErr != nil {
			return Response{}, contextErr
		}
		return Response{}, errors.New("read ChatGPT Codex response")
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return Response{}, fmt.Errorf("ChatGPT Codex request failed: status %d", response.StatusCode)
	}

	text, err := parseResponseText(body)
	if err != nil {
		return Response{}, err
	}
	return Response{Text: text}, nil
}

type codexRequest struct {
	Model  string              `json:"model"`
	Input  []codexInputMessage `json:"input"`
	Stream bool                `json:"stream"`
}

type codexInputMessage struct {
	Role    string              `json:"role"`
	Content []codexInputContent `json:"content"`
}

type codexInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexResponse struct {
	OutputText string `json:"output_text"`
	Output     []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func parseResponseText(data []byte) (string, error) {
	var response codexResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return "", errors.New("decode ChatGPT Codex response")
	}
	if strings.TrimSpace(response.OutputText) != "" {
		return response.OutputText, nil
	}
	for _, output := range response.Output {
		for _, content := range output.Content {
			if strings.TrimSpace(content.Text) != "" {
				return content.Text, nil
			}
		}
	}
	return "", errors.New("ChatGPT Codex response contained no text")
}

func (c *Client) endpoint(path string) string {
	copy := *c.baseURL
	copy.Path = strings.TrimRight(copy.Path, "/") + path
	return copy.String()
}

func cleanContextError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return nil
}
