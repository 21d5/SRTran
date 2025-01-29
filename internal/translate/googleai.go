// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package translate

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

func (s *Service) translateWithGoogleAI(ctx context.Context, text string, sourceLang, targetLang string) ([][]string, error) {
	if s.config.Model == "" {
		return nil, fmt.Errorf("model must be specified for Google AI backend")
	}

	prompt := fmt.Sprintf(translationPrompt, sourceLang, targetLang, text)

	maxAttempts := 5
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// wait for rate limit before making request
		if err := s.waitForRateLimit(ctx); err != nil {
			return nil, fmt.Errorf("rate limit wait interrupted: %w", err)
		}

		result, err := s.googleClient.Models.GenerateContent(ctx, s.config.Model, genai.Text(prompt), nil)
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
