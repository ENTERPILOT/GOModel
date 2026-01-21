# GOModel Examples

This document provides examples for using the GOModel gateway with different providers.

## Setup

1. Create a `.env` file in the project root:

```bash
PORT=8080
OPENAI_API_KEY=sk-your-openai-api-key-here
ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key-here
GEMINI_API_KEY=your-gemini-api-key-here
XAI_API_KEY=xai-your-xai-api-key-here
```

Note: You only need to provide the API key for the provider(s) you want to use.

2. Start the server:

```bash
make run
```

## OpenAI Examples

### Basic Chat Completion

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

### Chat Completion with Parameters

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Write a haiku about programming."}
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

### Streaming Response

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Tell me a short story."}
    ],
    "stream": true
  }'
```

## Anthropic Examples

### Basic Chat Completion

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

### Chat Completion with System Message

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [
      {"role": "system", "content": "You are a creative writing assistant."},
      {"role": "user", "content": "Write a haiku about the ocean."}
    ],
    "temperature": 0.8,
    "max_tokens": 200
  }'
```

### Streaming Response

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [
      {"role": "user", "content": "Explain quantum computing in simple terms."}
    ],
    "stream": true
  }'
```

### Using Claude Opus (Most Capable Model)

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus-20240229",
    "messages": [
      {"role": "user", "content": "Analyze the pros and cons of renewable energy."}
    ],
    "max_tokens": 1000
  }'
```

## Google Gemini Examples

### Basic Chat Completion

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-3-flash-preview",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ]
  }'
```

### Chat Completion with Parameters

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-1.5-pro",
    "messages": [
      {"role": "system", "content": "You are a knowledgeable science educator."},
      {"role": "user", "content": "Explain photosynthesis in simple terms."}
    ],
    "temperature": 0.7,
    "max_tokens": 500
  }'
```

### Streaming Response

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "gemini-2.0-flash",
    "messages": [
      {"role": "user", "content": "Write a short poem about AI."}
    ],
    "stream": true
  }'
```

### Using Gemini 1.5 Pro (Most Capable Gemini Model)

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-1.5-pro",
    "messages": [
      {"role": "user", "content": "Compare and contrast supervised and unsupervised learning in machine learning."}
    ],
    "max_tokens": 1000
  }'
```

## xAI Examples

### Basic Responses API Request

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-4-1-fast-non-reasoning",
    "input": "What is the capital of France?"
  }'
```

### Responses API Request with Instructions

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-4-1-fast-non-reasoning",
    "input": "Write a haiku about programming.",
    "instructions": "You are a creative AI assistant who specializes in writing poetry.",
    "temperature": 0.8,
    "max_output_tokens": 200
  }'
```

### Responses API with Structured Input

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-4-1-fast-non-reasoning",
    "input": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Explain quantum computing in simple terms."}
    ],
    "max_output_tokens": 500
  }'
```

### Streaming Responses

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "grok-4-1-fast-non-reasoning",
    "input": "Tell me a short story about AI.",
    "stream": true
  }'
```

### Using Grok 2 Mini (Faster Model)

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-4-1-fast-non-reasoning",
    "input": "What are the benefits of renewable energy?",
    "max_output_tokens": 1000
  }'
```

## List Available Models

Get a list of all available models from configured providers:

```bash
curl http://localhost:8080/v1/models
```

Example response:

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "owned_by": "openai",
      "created": 1234567890
    },
    {
      "id": "claude-3-5-sonnet-20241022",
      "object": "model",
      "owned_by": "anthropic",
      "created": 1234567890
    }
  ]
}
```

## Health Check

```bash
curl http://localhost:8080/health
```

## Python Example

Here's how to use GOModel as a drop-in replacement for the OpenAI Python client:

```python
from openai import OpenAI

# Point to your GOModel instance instead of OpenAI
client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="not-needed"  # API key is configured on the server side
)

# Use OpenAI models
response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
print(response.choices[0].message.content)

# Or use Anthropic models with the same interface!
response = client.chat.completions.create(
    model="claude-3-5-sonnet-20241022",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
print(response.choices[0].message.content)

# Or use Google Gemini models!
response = client.chat.completions.create(
    model="gemini-2.0-flash",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
print(response.choices[0].message.content)

# Or use xAI Grok models!
response = client.chat.completions.create(
    model="grok-4-1-fast-non-reasoning",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
print(response.choices[0].message.content)

# Streaming works too
stream = client.chat.completions.create(
    model="claude-3-5-haiku-20241022",
    messages=[{"role": "user", "content": "Tell me a story"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

## Node.js Example

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8080/v1",
  apiKey: "not-needed", // API key is configured on the server side
});

// Use OpenAI models
const response1 = await client.chat.completions.create({
  model: "gpt-4o-mini",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response1.choices[0].message.content);

// Or use Anthropic models with the same interface!
const response2 = await client.chat.completions.create({
  model: "claude-3-5-sonnet-20241022",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response2.choices[0].message.content);

// Or use Google Gemini models!
const response3 = await client.chat.completions.create({
  model: "gemini-2.0-flash",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response3.choices[0].message.content);

// Or use xAI Grok models!
const response4 = await client.chat.completions.create({
  model: "grok-4-1-fast-non-reasoning",
  messages: [{ role: "user", content: "Hello!" }],
});
console.log(response4.choices[0].message.content);

// Streaming
const stream = await client.chat.completions.create({
  model: "claude-3-5-haiku-20241022",
  messages: [{ role: "user", content: "Tell me a story" }],
  stream: true,
});

for await (const chunk of stream) {
  if (chunk.choices[0]?.delta?.content) {
    process.stdout.write(chunk.choices[0].delta.content);
  }
}
```

## Available Models

### OpenAI

- `gpt-4o` - Most capable GPT-4 model
- `gpt-4o-mini` - Fast and efficient GPT-4 model
- `gpt-4-turbo` - Previous generation GPT-4 Turbo
- `gpt-4` - Original GPT-4 model
- `gpt-3.5-turbo` - Fast and cost-effective
- `o1-preview` - Advanced reasoning model (preview)
- `o1-mini` - Faster reasoning model

### Anthropic

- `claude-3-5-sonnet-20241022` - Latest Sonnet (best balance of speed and capability)
- `claude-3-5-sonnet-20240620` - Previous Sonnet version
- `claude-3-5-haiku-20241022` - Latest Haiku (fastest, most cost-effective)
- `claude-3-opus-20240229` - Most capable Claude model
- `claude-3-sonnet-20240229` - Previous Sonnet generation
- `claude-3-haiku-20240307` - Previous Haiku generation

### Google Gemini

- `gemini-2.0-flash` - Latest Flash model (fast and efficient)
- `gemini-1.5-pro` - Most capable Gemini model (large context window)
- `gemini-1.5-flash` - Previous generation Flash model
- `gemini-1.0-pro` - Original Gemini Pro model

### xAI

- `grok-4-1-fast-non-reasoning` - Most capable Grok model (best for complex reasoning)

## Tips

1. **Model Selection**: The gateway automatically routes requests to the correct provider based on the model name prefix:
   - `gpt-` or `o1` → OpenAI
   - `claude-` → Anthropic
   - `gemini-` → Google Gemini
   - `grok-` → xAI

2. **API Compatibility**: The gateway exposes an OpenAI-compatible API, so you can use existing OpenAI client libraries to access OpenAI, Anthropic, and Google Gemini models.

3. **Streaming**: All providers support streaming responses. The gateway automatically converts provider-specific streaming formats to match OpenAI's SSE format.

4. **System Messages**:
   - Anthropic handles system messages differently. The gateway automatically extracts system messages and sends them in Anthropic's required format.
   - Gemini uses Google's OpenAI-compatible endpoint which handles system messages natively.

5. **Max Tokens**:
   - Anthropic requires `max_tokens` to be set. If not provided, the gateway uses a default of 4096 tokens.
   - Gemini and OpenAI have optional `max_tokens` parameters.

6. **Context Windows**:
   - Gemini 1.5 Pro offers an exceptionally large context window (up to 1M tokens), making it ideal for long-form content analysis.
   - OpenAI GPT-4 models typically support 8K-128K tokens depending on the variant.
   - Anthropic Claude models support up to 200K tokens.
   - xAI Grok models support up to 128K tokens.

7. **Responses API**: The `/v1/responses` endpoint is OpenAI-compatible and provides a unified interface across all providers. xAI internally converts Responses API requests to chat completion format since it doesn't natively support the Responses API.
