// Copyright (c) 2025, soup and the SRTran contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package translate

// Backend represents the AI service provider
type Backend string

const (
	BackendOpenAI     Backend = "openai"
	BackendOpenRouter Backend = "openrouter"
	BackendGoogleAI   Backend = "googleai"
	BackendLMStudio   Backend = "lmstudio"
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

// translationPrompt is the standard prompt template for all translation models
const translationPrompt = `You are a professional subtitle translator. Translate exactly %s to %s following these rules:
1. Preserve exact timing by keeping text length similar
2. Maintain original line breaks and formatting symbols (e.g., <i>, [music])
3. Never split or merge subtitle blocks
4. Keep proper nouns/technical terms in original language when no direct translation exists
5. Use colloquial speech matching the source register
6. Handle idioms with culturally equivalent expressions
7. Preserve numbers, measurements, and codes exactly
8. Maintain capitalization style for on-screen text
9. Keep placeholder markers like [%%1] unchanged
10. Use contractions where natural for spoken language

Format:
[N] (subtitle number)
Translated text (same line breaks)
===SUBTITLE=== separator between blocks

Here are the subtitles to translate:

%s`
