# GOModel Examples

This document provides examples for using the GOModel gateway with different providers.

## Setup

1. Create a `.env` file in the project root:

```bash
PORT=8080
OPENAI_API_KEY=sk-your-openai-api-key-here
ANTHROPIC_API_KEY=sk-ant-your-anthropic-api-key-here
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
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:8080/v1',
  apiKey: 'not-needed' // API key is configured on the server side
});

// Use OpenAI models
const response1 = await client.chat.completions.create({
  model: 'gpt-4o-mini',
  messages: [{ role: 'user', content: 'Hello!' }]
});
console.log(response1.choices[0].message.content);

// Or use Anthropic models with the same interface!
const response2 = await client.chat.completions.create({
  model: 'claude-3-5-sonnet-20241022',
  messages: [{ role: 'user', content: 'Hello!' }]
});
console.log(response2.choices[0].message.content);

// Streaming
const stream = await client.chat.completions.create({
  model: 'claude-3-5-haiku-20241022',
  messages: [{ role: 'user', content: 'Tell me a story' }],
  stream: true
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

## Tips

1. **Model Selection**: The gateway automatically routes requests to the correct provider based on the model name prefix (`gpt-` or `o1` → OpenAI, `claude-` → Anthropic).

2. **API Compatibility**: The gateway exposes an OpenAI-compatible API, so you can use existing OpenAI client libraries to access both OpenAI and Anthropic models.

3. **Streaming**: Both providers support streaming responses. The gateway automatically converts Anthropic's streaming format to match OpenAI's format.

4. **System Messages**: Anthropic handles system messages differently. The gateway automatically extracts system messages and sends them in Anthropic's required format.

5. **Max Tokens**: Anthropic requires `max_tokens` to be set. If not provided, the gateway uses a default of 4096 tokens.

