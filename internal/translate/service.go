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
	"github.com/s0up4200/SRTran/internal/srt"
	openai "github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

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
	// API key is required for all backends except LM Studio
	if config.APIKey == "" && config.Backend != BackendLMStudio {
		return nil, fmt.Errorf("API key is required for %s backend", config.Backend)
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
	case BackendLMStudio:
		if config.BaseURL == "" {
			config.BaseURL = "http://localhost:1234/v1"
		}
		clientConfig := openai.DefaultConfig("") // Empty API key is fine for LM Studio
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

// translateBatchInternal handles the actual translation of a batch of subtitles
func (s *Service) translateBatchInternal(ctx context.Context, subtitles []srt.Subtitle, sourceLang, targetLang string) ([]srt.Subtitle, error) {
	if len(subtitles) == 0 {
		return subtitles, nil
	}

	maxRetries := 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// combine subtitle texts with numbered markers
		var batchText strings.Builder
		for i, sub := range subtitles {
			if i > 0 {
				batchText.WriteString("\n===SUBTITLE===\n")
			}
			batchText.WriteString(fmt.Sprintf("[%d]\n", i+1))
			batchText.WriteString(strings.Join(sub.Text, "\n"))
			batchText.WriteString("\n")
		}

		var cleanTranslations [][]string
		var err error

		switch s.config.Backend {
		case BackendOpenAI:
			cleanTranslations, err = s.translateWithOpenAI(ctx, batchText.String(), sourceLang, targetLang)
		case BackendOpenRouter:
			cleanTranslations, err = s.translateWithOpenRouter(ctx, batchText.String(), sourceLang, targetLang)
		case BackendLMStudio:
			cleanTranslations, err = s.translateWithLMStudio(ctx, batchText.String(), sourceLang, targetLang)
		case BackendGoogleAI:
			cleanTranslations, err = s.translateWithGoogleAI(ctx, batchText.String(), sourceLang, targetLang)
		default:
			return nil, fmt.Errorf("unsupported backend: %s", s.config.Backend)
		}

		if err != nil {
			if attempt < maxRetries {
				s.logger.Warn().
					Int("attempt", attempt+1).
					Int("max_retries", maxRetries).
					Err(err).
					Msg("translation attempt failed, retrying")
				continue
			}
			return nil, err
		}

		// If we got fewer translations than expected but not zero
		if len(cleanTranslations) > 0 && len(cleanTranslations) < len(subtitles) {
			if attempt < maxRetries {
				s.logger.Warn().
					Int("expected", len(subtitles)).
					Int("received", len(cleanTranslations)).
					Int("attempt", attempt+1).
					Msg("received partial translations, retrying")
				continue
			}
			// On final attempt, abort with error
			return nil, fmt.Errorf("failed to get complete translations after %d attempts: expected %d, got %d",
				maxRetries, len(subtitles), len(cleanTranslations))
		}

		// Success case - we got the expected number of translations
		if len(cleanTranslations) == len(subtitles) {
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
	}

	return nil, fmt.Errorf("failed to get complete translations after %d attempts", maxRetries)
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
