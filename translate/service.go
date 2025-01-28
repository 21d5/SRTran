package translate

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/soup/SRTran/srt"
)

// ServiceConfig holds the configuration for the translation service
type ServiceConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Verbose bool
}

// Service handles the translation of subtitles using AI models
type Service struct {
	client  *openai.Client
	config  ServiceConfig
	verbose bool
}

// NewService creates a new translation service
func NewService(config ServiceConfig) (*Service, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	clientConfig := openai.DefaultConfig(config.APIKey)

	// Configure OpenRouter if BaseURL is provided
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	client := openai.NewClientWithConfig(clientConfig)
	return &Service{
		client:  client,
		config:  config,
		verbose: config.Verbose,
	}, nil
}

// TranslateSubtitles translates a slice of subtitles to the target language
func (s *Service) TranslateSubtitles(ctx context.Context, subtitles []srt.Subtitle, sourceLang, targetLang string) error {
	for i := range subtitles {
		if err := s.translateSubtitle(ctx, &subtitles[i], sourceLang, targetLang); err != nil {
			return fmt.Errorf("failed to translate subtitle %d: %w", i+1, err)
		}
		if s.verbose {
			fmt.Printf("Translated subtitle %d/%d\n", i+1, len(subtitles))
		}
	}
	return nil
}

// translateSubtitle translates a single subtitle
func (s *Service) translateSubtitle(ctx context.Context, sub *srt.Subtitle, sourceLang, targetLang string) error {
	text := strings.Join(sub.Text, "\n")
	prompt := fmt.Sprintf("Translate the following %s text to %s, preserving line breaks:\n\n%s", sourceLang, targetLang, text)

	model := openai.GPT4
	if s.config.Model != "" {
		model = s.config.Model
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a professional translator. Translate the text exactly as provided, maintaining the same tone and meaning. Preserve any formatting and line breaks.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return fmt.Errorf("no translation received from OpenAI")
	}

	// Split the translated text by newlines to preserve formatting
	translatedText := resp.Choices[0].Message.Content
	sub.Translated = strings.Split(strings.TrimSpace(translatedText), "\n")

	return nil
}
