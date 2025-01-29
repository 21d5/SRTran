# SRTran

SRTran is a command-line tool for translating subtitle files (.srt) from one language to another using various AI language models.

## Features

- Translate .srt subtitle files between any language pair
- Support for multiple AI providers:
  - Google AI Studio (Gemini)
  - OpenAI (GPT-4)
  - OpenRouter (various models)
- Preserve subtitle timing and formatting
- Support for verbose output to track translation progress
- Easy-to-use command-line interface

## Installation

1. Ensure you have Go 1.16 or later installed
2. Clone the repository:
   ```bash
   git clone https://github.com/s0up4200/SRTran.git
   cd SRTran
   ```
3. Build the binary:
   ```bash
   go build
   ```

## Configuration

SRTran can be configured using a TOML configuration file or environment variables.

### Using config.toml (Recommended)

Create a `config.toml` file in one of these locations:
- `./config.toml` (current directory)
- `~/.config/srtran/config.toml`
- `~/.srtran.toml`

Example configuration:
```toml
# SRTran Configuration

# Backend can be: googleai, openai, or openrouter
backend = "googleai"

# Model depends on the backend selected
model = "gemini-2.0-flash-exp"

# API key for the selected backend
api_key = "your_api_key_here"

# Maximum requests per minute (0 for no limit)
# Check the API docs for the limits of your selected backend/model
rpm = 9
```

You can also specify a custom config file location using the `-c` flag:
```bash
srtran translate -c /path/to/config.toml -i input.srt -o output.srt -s english -t norwegian
```

### Using Environment Variables

Alternatively, you can use environment variables:
```bash
# Google AI Studio
export GOOGLE_AI_API_KEY='your-key' GOOGLE_AI_MODEL='gemini-2.0-flash-exp'

# OpenRouter
export OPENROUTER_API_KEY='your-key' OPENROUTER_MODEL='anthropic/claude-3.5-sonnet'

# OpenAI
export OPENAI_API_KEY='your-key' OPENAI_MODEL='gpt-4'
```

The tool will try API keys in this order: Google AI → OpenRouter → OpenAI

## Usage

### Basic Translation

```bash
srtran translate -i input.srt -o output.srt -s english -t spanish
```

This command translates subtitles from English to Spanish.

### Command-line Options

- `-i, --input`: Input subtitle file (required)
- `-o, --output`: Output subtitle file (required)
- `-s, --source-language`: Source language (required)
- `-t, --target-language`: Target language (required)
- `-v, --verbose`: Enable verbose output

### Examples

1. Translate from English to French with verbose output:
   ```bash
   srtran translate -i movie.srt -o movie_fr.srt -s english -t french -v
   ```

2. Translate from Spanish to German:
   ```bash
   srtran translate -i spanish.srt -o german.srt -s spanish -t german
   ```

### Version Information

To check the version of SRTran:

```bash
srtran version
```

Add `-v` for detailed version information:

```bash
srtran version -v
```

## Supported Languages

SRTran supports translation between any language pair. The supported languages depend on the AI provider being used.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
