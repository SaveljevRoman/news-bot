package summary

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"log"
	"strings"
	"sync"
)

type OpenAISummarizer struct {
	client  *openai.Client
	prompt  string
	enabled bool
	mu      sync.Mutex
}

func NewOpenAISummarizer(apiKey string, prompt string) *OpenAISummarizer {
	s := &OpenAISummarizer{
		client: openai.NewClient(apiKey),
		prompt: prompt,
	}

	log.Printf("openai summarizer enabled: %v", apiKey != "")
	if apiKey != "" {
		s.enabled = true
	}

	return s
}

func (s *OpenAISummarizer) Summarize(ctx context.Context, text string) (string, error) {
	//либа github.com/sashabaranov/go-openai стремно работает с конкурентными запросами
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return "", nil
	}

	request := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: fmt.Sprintf("%s%s", text, s.prompt),
			},
		},
		MaxTokens:   256,
		Temperature: 0.7,
		TopP:        1,
	}

	resp, err := s.client.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	//Выбираем самый первый вариант ответа openai
	rawSummary := strings.TrimSpace(resp.Choices[0].Message.Content)
	//проверяем что в конце точка
	if strings.HasSuffix(rawSummary, ".") {
		return rawSummary, nil
	}

	//если в конце не точка
	sentences := strings.Split(rawSummary, ".")

	return strings.Join(sentences[:len(sentences)-1], ".") + ".", nil
}
