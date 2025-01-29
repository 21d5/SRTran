// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package translate

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/s0up4200/SRTran/srt"
	openai "github.com/sashabaranov/go-openai"
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
	// RPM is the maximum number of requests per minute
	// if set to 0, no rate limiting is applied
	RPM int
}

// Service handles the translation of subtitles using AI models
type Service struct {
	openaiClient *openai.Client
	googleClient *genai.Client
	config       ServiceConfig
	verbose      bool
	logger       zerolog.Logger
	// rate limiter fields
	rateLimiter   *time.Ticker
	rateLimiterMu sync.Mutex
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

	// initialize rate limiter if RPM is set
	if config.RPM > 0 {
		// calculate interval between requests
		interval := time.Minute / time.Duration(config.RPM)
		service.rateLimiter = time.NewTicker(interval)
		service.logger.Info().
			Int("rpm", config.RPM).
			Dur("interval", interval).
			Msg("rate limiter initialized")
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

// waitForRateLimit waits for the rate limiter if it's configured
func (s *Service) waitForRateLimit(ctx context.Context) error {
	if s.rateLimiter == nil {
		return nil
	}

	s.rateLimiterMu.Lock()
	defer s.rateLimiterMu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.rateLimiter.C:
		return nil
	}
}

// Close cleans up resources used by the service
func (s *Service) Close() {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
}

// translateBatch translates a batch of subtitles
func (s *Service) translateBatch(ctx context.Context, subtitles []srt.Subtitle, sourceLang, targetLang string) ([]srt.Subtitle, error) {
	batchSize := 20 // or whatever the current batch size is
	var translated []srt.Subtitle

	for i := 0; i < len(subtitles); i += batchSize {
		end := i + batchSize
		if end > len(subtitles) {
			end = len(subtitles)
		}

		batch := subtitles[i:end]

		// Retry logic for rate limits
		maxAttempts := 10
		baseDelay := time.Second

		for attempt := 0; attempt < maxAttempts; attempt++ {
			// Wait for rate limiter
			if err := s.waitForRateLimit(ctx); err != nil {
				return nil, fmt.Errorf("rate limit wait error: %w", err)
			}

			batchTranslated, err := s.translateBatchInternal(ctx, batch, sourceLang, targetLang)
			if err != nil {
				if strings.Contains(err.Error(), "429") ||
					strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
					delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
					s.logger.Warn().
						Int("attempt", attempt).
						Dur("backoff", delay).
						Msg("rate limit hit, backing off")

					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(delay):
						continue // retry after delay
					}
				}
				return nil, fmt.Errorf("failed to translate batch %d-%d: %w", i, end, err)
			}

			translated = append(translated, batchTranslated...)

			break // successful translation, move to next batch
		}
	}

	return translated, nil
}

// Move the actual translation logic to a separate method
func (s *Service) translateBatchInternal(ctx context.Context, subtitles []srt.Subtitle, sourceLang, targetLang string) ([]srt.Subtitle, error) {
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

	// wait for rate limit before making request
	if err := s.waitForRateLimit(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait interrupted: %w", err)
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

// rateLimitBackoff implements exponential backoff for rate limits
func (s *Service) rateLimitBackoff(ctx context.Context, attempt int) error {
	backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	s.logger.Warn().
		Int("attempt", attempt).
		Dur("backoff", backoff).
		Msg("rate limit hit, backing off")

	timer := time.NewTimer(backoff)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

	maxAttempts := 5
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// wait for rate limit before making request
		if err := s.waitForRateLimit(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait interrupted: %w", err)
		}

		result, err := s.googleClient.Models.GenerateContent(ctx, model, genai.Text(prompt), nil)
		if err != nil {
			// check for rate limit errors
			if strings.Contains(err.Error(), "quota") ||
				strings.Contains(err.Error(), "rate limit") ||
				strings.Contains(err.Error(), "resource exhausted") {
				lastErr = err
				if err := s.rateLimitBackoff(ctx, attempt); err != nil {
					return nil, fmt.Errorf("rate limit backoff interrupted: %w", err)
				}
				continue
			}
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

	return nil, fmt.Errorf("max retries exceeded due to rate limits: %w", lastErr)
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

		// Simplified progress logging
		s.logger.Info().
			Int("processed", len(result)).
			Int("remaining", len(subtitles)-len(result)).
			Int("percent", int(float64(len(result))/float64(len(subtitles))*100)).
			Msg("translation progress")
	}

	return result, nil
}
