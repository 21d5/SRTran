package translate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	openai "github.com/sashabaranov/go-openai"
	"github.com/soup/SRTran/srt"
	"google.golang.org/genai"
)

// Backend represents the AI service provider
type Backend string

const (
	BackendOpenAI     Backend = "openai"
	BackendOpenRouter Backend = "openrouter"
	BackendGoogleAI   Backend = "googleai"
)

// ServiceConfig holds the configuration for the translation service
type ServiceConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Verbose bool
	Backend Backend
}

// Service handles the translation of subtitles using AI models
type Service struct {
	openaiClient *openai.Client
	googleClient *genai.Client
	config       ServiceConfig
	verbose      bool
	logger       zerolog.Logger
}

// batch size for translations
const defaultBatchSize = 20

// NewService creates a new translation service
func NewService(config ServiceConfig) (*Service, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	service := &Service{
		config:  config,
		verbose: config.Verbose,
		logger:  zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger(),
	}

	switch config.Backend {
	case BackendOpenAI:
		clientConfig := openai.DefaultConfig(config.APIKey)
		service.openaiClient = openai.NewClientWithConfig(clientConfig)
	case BackendOpenRouter:
		clientConfig := openai.DefaultConfig(config.APIKey)
		clientConfig.BaseURL = config.BaseURL
		service.openaiClient = openai.NewClientWithConfig(clientConfig)
	case BackendGoogleAI:
		client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey:  config.APIKey,
			Backend: genai.BackendGoogleAI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Google AI client: %w", err)
		}
		service.googleClient = client
	default:
		return nil, fmt.Errorf("unsupported backend: %s", config.Backend)
	}

	return service, nil
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

	var cleanTranslations [][]string
	var err error

	switch s.config.Backend {
	case BackendOpenAI, BackendOpenRouter:
		cleanTranslations, err = s.translateWithOpenAI(ctx, batchText.String(), sourceLang, targetLang)
	case BackendGoogleAI:
		cleanTranslations, err = s.translateWithGoogleAI(ctx, batchText.String(), sourceLang, targetLang)
	default:
		return nil, fmt.Errorf("unsupported backend: %s", s.config.Backend)
	}

	if err != nil {
		return nil, err
	}

	// ensure we got the same number of translations as inputs
	if len(cleanTranslations) != len(subtitles) {
		return nil, fmt.Errorf("received %d translations for %d subtitles",
			len(cleanTranslations), len(subtitles))
	}

	// update subtitle texts with translations
	result := make([]srt.Subtitle, len(subtitles))
	copy(result, subtitles)
	for i := range result {
		result[i].Translated = cleanTranslations[i]
		if s.verbose {
			s.logger.Debug().
				Str("original", strings.Join(result[i].Text, "\n")).
				Str("translated", strings.Join(result[i].Translated, "\n")).
				Msg("translation completed")
		}
	}

	return result, nil
}

func (s *Service) translateWithOpenAI(ctx context.Context, text string, sourceLang, targetLang string) ([][]string, error) {
	// use configured model or fallback to GPT-4
	model := openai.GPT4
	if s.config.Model != "" {
		model = s.config.Model
	}

	resp, err := s.openaiClient.CreateChatCompletion(
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
					Content: text,
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to translate batch: %w", err)
	}

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

	return cleanTranslations, nil
}

func (s *Service) translateWithGoogleAI(ctx context.Context, text string, sourceLang, targetLang string) ([][]string, error) {
	model := "gemini-2.0-flash-exp"
	if s.config.Model != "" {
		model = s.config.Model
	}

	prompt := fmt.Sprintf(
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
			"===SUBTITLE=== separator between blocks\n\n"+
			"Here are the subtitles to translate:\n\n%s",
		sourceLang, targetLang, text)

	result, err := s.googleClient.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to translate batch: %w", err)
	}

	if len(result.Candidates) == 0 {
		return nil, fmt.Errorf("no response from Google AI")
	}

	candidate := result.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Google AI")
	}

	// Get the text from the first part
	responseText := candidate.Content.Parts[0].Text

	// split response by subtitle separator
	translations := strings.Split(responseText, "===SUBTITLE===")

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

	return cleanTranslations, nil
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
