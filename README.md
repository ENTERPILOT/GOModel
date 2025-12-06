# GOModel

A high-performance LLM gateway written in Go.

## Quick Start

1. Set your OpenAI API key:

   ```bash
   export OPENAI_API_KEY="your-api-key"
   ```

2. Run the server:

   ```bash
   make run
   ```

3. Test it:
   ```bash
   curl http://localhost:8088/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

## Running with Docker

You can use the official `golang:1.21-alpine` image to run the project in a container:

```bash
docker run --rm -it \
  -v $(pwd):/app \
  -w /app \
  -p 8088:8088 \
  -e OPENAI_API_KEY="your-api-key" \
  golang:1.21-alpine \
  go run ./cmd/gomodel
```

## Endpoints

- `POST /v1/chat/completions` - OpenAI-compatible chat completions
- `GET /v1/models` - List available models
- `GET /health` - Health check
