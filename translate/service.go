package translate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
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
	logger  zerolog.Logger
}

// batch size for translations
const defaultBatchSize = 20

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
		logger:  zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}),
	}, nil
}

// translateBatch translates a batch of subtitles
func (s *Service) translateBatch(ctx context.Context, subtitles []srt.Subtitle, sourceLang, targetLang string) ([]srt.Subtitle, error) {
	if len(subtitles) == 0 {
		return subtitles, nil
	}

	// combine subtitle texts with numbered markers
	var batchText strings.Builder
	for i, sub := range subtitles {
		if i > 0 {
			batchText.WriteString("\n===SUBTITLE===\n")
		}
		batchText.WriteString(fmt.Sprintf("[%d]\n", i+1))
		// preserve original line breaks
		batchText.WriteString(strings.Join(sub.Text, "\n"))
		batchText.WriteString("\n")
	}

	//s.logger.Trace().
	//	Str("batch_text", batchText.String()).
	//	Msg("sending batch for translation")

	// use configured model or fallback to GPT-4
	model := openai.GPT4
	if s.config.Model != "" {
		model = s.config.Model
	}

	resp, err := s.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{
					Role: openai.ChatMessageRoleSystem,
					Content: fmt.Sprintf(
						"You are a professional subtitle translator. Translate exactly %s to %s following these rules:\n"+
							"1. Preserve exact timing by keeping text length similar\n"+
							"2. Maintain original line breaks and formatting symbols (e.g., <i>, [music])\n"+
							"3. Never split or merge subtitle blocks\n"+
							"4. Keep proper nouns/technical terms in original language when no direct translation exists\n"+
							"5. Use colloquial speech matching the source register\n"+
							"6. Handle idioms with culturally equivalent expressions\n"+
							"7. Preserve numbers, measurements, and codes exactly\n"+
							"8. Maintain capitalization style for on-screen text\n"+
							"9. Keep placeholder markers like [%%1] unchanged\n"+
							"10. Use contractions where natural for spoken language\n\n"+
							"Format:\n"+
							"[N] (subtitle number)\n"+
							"Translated text (same line breaks)\n"+
							"===SUBTITLE=== separator between blocks",
						sourceLang, targetLang),
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: batchText.String(),
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to translate batch: %w", err)
	}

	//s.logger.Trace().
	//	Str("response", resp.Choices[0].Message.Content).
	//	Msg("received translation response")

	// split response by subtitle separator
	translations := strings.Split(resp.Choices[0].Message.Content, "===SUBTITLE===")

	var cleanTranslations [][]string
	for _, t := range translations {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		// remove the [N] prefix
		if idx := strings.Index(t, "]\n"); idx != -1 {
			t = strings.TrimSpace(t[idx+2:])
		}

		// split by natural line breaks
		lines := strings.Split(t, "\n")
		if len(lines) > 0 {
			cleanTranslations = append(cleanTranslations, lines)
		}
	}

	// ensure we got the same number of translations as inputs
	if len(cleanTranslations) != len(subtitles) {
		return nil, fmt.Errorf("received %d translations for %d subtitles (response: %s)",
			len(cleanTranslations), len(subtitles), resp.Choices[0].Message.Content)
	}

	// update subtitle texts with translations
	result := make([]srt.Subtitle, len(subtitles))
	copy(result, subtitles)
	for i := range result {
		result[i].Text = cleanTranslations[i]
	}

	return result, nil
}

// Translate processes all subtitles in batches
func (s *Service) Translate(ctx context.Context, subtitles []srt.Subtitle, sourceLang, targetLang string) ([]srt.Subtitle, error) {
	s.logger.Debug().
		Int("total_subtitles", len(subtitles)).
		Str("source_lang", sourceLang).
		Str("target_lang", targetLang).
		Msg("starting batch translation")

	result := make([]srt.Subtitle, 0, len(subtitles))

	// process in batches
	for i := 0; i < len(subtitles); i += defaultBatchSize {
		end := i + defaultBatchSize
		if end > len(subtitles) {
			end = len(subtitles)
		}

		batch := subtitles[i:end]
		translated, err := s.translateBatch(ctx, batch, sourceLang, targetLang)
		if err != nil {
			return nil, fmt.Errorf("failed to translate batch %d-%d: %w", i, end, err)
		}

		result = append(result, translated...)

		s.logger.Debug().
			Int("batch_start", i).
			Int("batch_end", end).
			Int("processed", len(result)).
			Int("total", len(subtitles)).
			Msg("batch processed")
	}

	return result, nil
}
