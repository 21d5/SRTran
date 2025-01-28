# SRTran

SRTran is a command-line tool for translating subtitle files (.srt) from one language to another using OpenAI's language models.

## Features

- Translate .srt subtitle files between any language pair
- Preserve subtitle timing and formatting
- Support for verbose output to track translation progress
- Easy-to-use command-line interface

## Installation

1. Ensure you have Go 1.16 or later installed
2. Clone the repository:
   ```bash
   git clone https://github.com/soup/SRTran.git
   cd SRTran
   ```
3. Build the binary:
   ```bash
   go build
   ```

## Configuration

SRTran supports both OpenAI and OpenRouter for translations. You can configure the service using environment variables either directly or through a `.env` file.

1. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` with your preferred configuration:

   ### OpenRouter Configuration (Preferred)
   ```env
   OPENROUTER_API_KEY=your_openrouter_api_key_here
   OPENROUTER_MODEL=anthropic/claude-3.5-sonnet
   ```

   ### OpenAI Configuration (Alternative)
   ```env
   OPENAI_API_KEY=your_openai_api_key_here
   ```

The tool will automatically use OpenRouter if `OPENROUTER_API_KEY` is set, falling back to OpenAI if only `OPENAI_API_KEY` is available.

Note: You can also set these environment variables directly in your shell:
```bash
export OPENROUTER_API_KEY='your-openrouter-api-key-here'
export OPENROUTER_MODEL='anthropic/claude-3.5-sonnet'  # Optional
# or
export OPENAI_API_KEY='your-openai-api-key-here'
```

## Usage

### Basic Translation

```bash
srtran translate -i input.srt -o output.srt -s en -t es
```

This command translates subtitles from English (`en`) to Spanish (`es`).

### Command-line Options

- `-i, --input`: Input subtitle file (required)
- `-o, --output`: Output subtitle file (required)
- `-s, --source-language`: Source language code (required)
- `-t, --target-language`: Target language code (required)
- `-v, --verbose`: Enable verbose output

### Examples

1. Translate from English to French with verbose output:
   ```bash
   srtran translate -i movie.srt -o movie_fr.srt -s en -t fr -v
   ```

2. Translate from Spanish to German:
   ```bash
   srtran translate -i spanish.srt -o german.srt -s es -t de
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

## Language Codes

Use standard ISO 639-1 two-letter language codes:

- English: en
- Spanish: es
- French: fr
- German: de
- Italian: it
- Portuguese: pt
- And many more...

## Error Handling

SRTran includes robust error handling for common issues:

- Missing or invalid API key
- File not found or permission issues
- Invalid subtitle file format
- Network connectivity problems
- API rate limiting

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
