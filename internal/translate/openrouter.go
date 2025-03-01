// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// OpenRouterKeyInfo represents the response from the OpenRouter key info endpoint
type OpenRouterKeyInfo struct {
	Data struct {
		Label      string   `json:"label"`
		Usage      float64  `json:"usage"`
		Limit      *float64 `json:"limit"`
		IsFreeTier bool     `json:"is_free_tier"`
		RateLimit  struct {
			Requests int    `json:"requests"`
			Interval string `json:"interval"`
		} `json:"rate_limit"`
	} `json:"data"`
}

// OpenRouterError represents the error response from OpenRouter
type OpenRouterError struct {
	Error struct {
		Code     int                    `json:"code"`
		Message  string                 `json:"message"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	} `json:"error"`
}

// ModerationErrorMetadata represents moderation error details
type ModerationErrorMetadata struct {
	Reasons      []string `json:"reasons"`
	FlaggedInput string   `json:"flagged_input"`
	ProviderName string   `json:"provider_name"`
	ModelSlug    string   `json:"model_slug"`
}

// ProviderErrorMetadata represents provider error details
type ProviderErrorMetadata struct {
	ProviderName string      `json:"provider_name"`
	Raw          interface{} `json:"raw"`
}

func (s *Service) translateWithOpenRouter(ctx context.Context, text string, sourceLang, targetLang string) ([][]string, error) {
	if s.config.Model == "" {
		return nil, fmt.Errorf("model must be specified for OpenRouter backend")
	}

	maxRetries := 10
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := s.waitForRateLimit(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait interrupted: %w", err)
		}

		// Check key info and credits before proceeding
		keyInfo, err := s.getOpenRouterKeyInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check OpenRouter key info: %w", err)
		}

		// Log key info if verbose
		if s.verbose {
			s.logger.Debug().
				Float64("credits_used", keyInfo.Data.Usage).
				Interface("rate_limit", keyInfo.Data.RateLimit).
				Bool("is_free_tier", keyInfo.Data.IsFreeTier).
				Msg("OpenRouter key info")
		}

		resp, err := s.openaiClient.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model: s.config.Model,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: fmt.Sprintf(translationPrompt, sourceLang, targetLang, text),
					},
				},
			},
		)

		// Handle OpenRouter-specific errors
		if err != nil {
			var openRouterErr OpenRouterError
			if strings.Contains(err.Error(), "error code: 429") {
				lastErr = fmt.Errorf("rate limited: %w", err)
			} else if strings.Contains(err.Error(), "error code: 402") {
				return nil, fmt.Errorf("insufficient credits: %w", err) // Fatal error, don't retry
			} else if strings.Contains(err.Error(), "error code: 403") {
				if err := json.Unmarshal([]byte(err.Error()), &openRouterErr); err == nil {
					if metadata, ok := openRouterErr.Error.Metadata["moderation"].(map[string]interface{}); ok {
						return nil, fmt.Errorf("content moderation error: %v", metadata) // Fatal error, don't retry
					}
				}
				lastErr = fmt.Errorf("moderation error: %w", err)
			} else if strings.Contains(err.Error(), "error code: 502") {
				if err := json.Unmarshal([]byte(err.Error()), &openRouterErr); err == nil {
					if metadata, ok := openRouterErr.Error.Metadata["provider"].(map[string]interface{}); ok {
						lastErr = fmt.Errorf("provider error: %v", metadata)
					}
				}
				lastErr = fmt.Errorf("provider error: %w", err)
			} else {
				lastErr = fmt.Errorf("OpenRouter API error: %w", err)
			}

			if attempt < maxRetries-1 {
				s.logger.Warn().
					Err(lastErr).
					Int("attempt", attempt+1).
					Msg("translation failed, retrying")
				continue
			}
			return nil, lastErr
		}

		// Validate response
		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("empty response from OpenRouter")
			if attempt < maxRetries-1 {
				s.logger.Warn().
					Int("attempt", attempt+1).
					Msg("received empty response, retrying")
				continue
			}
			return nil, lastErr
		}

		// Handle no content case
		if resp.Choices[0].Message.Content == "" {
			lastErr = fmt.Errorf("model generated no content (possibly warming up)")
			if attempt < maxRetries-1 {
				s.logger.Warn().
					Int("attempt", attempt+1).
					Msg("model generated no content, retrying")
				continue
			}
			return nil, lastErr
		}

		// Split response by subtitle separator
		translations := strings.Split(resp.Choices[0].Message.Content, "===SUBTITLE===")

		// Count expected translations by counting "[N]" markers in input
		expectedCount := strings.Count(text, "[")

		var cleanTranslations [][]string
		for _, t := range translations {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}

			// Remove the [N] prefix
			if idx := strings.Index(t, "]\n"); idx != -1 {
				t = strings.TrimSpace(t[idx+2:])
			}

			// Split by natural line breaks
			lines := strings.Split(t, "\n")
			if len(lines) > 0 {
				cleanTranslations = append(cleanTranslations, lines)
			}
		}

		// Validate translation count
		if len(cleanTranslations) != expectedCount {
			lastErr = fmt.Errorf("received %d translations, expected %d", len(cleanTranslations), expectedCount)
			if attempt < maxRetries-1 {
				s.logger.Warn().
					Int("expected", expectedCount).
					Int("received", len(cleanTranslations)).
					Int("attempt", attempt+1).
					Msg("translation count mismatch, retrying")
				continue
			}
			return nil, lastErr
		}

		return cleanTranslations, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// getOpenRouterKeyInfo fetches information about the current API key
func (s *Service) getOpenRouterKeyInfo(ctx context.Context) (*OpenRouterKeyInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/auth/key", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get key info: %s", string(body))
	}

	var keyInfo OpenRouterKeyInfo
	if err := json.Unmarshal(body, &keyInfo); err != nil {
		return nil, fmt.Errorf("failed to parse key info: %w", err)
	}

	return &keyInfo, nil
}
