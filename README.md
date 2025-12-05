# HeavyModel

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
   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

## Endpoints

- `POST /v1/chat/completions` - OpenAI-compatible chat completions
- `GET /health` - Health check
