# Contract Tests

Contract tests validate API response structures against recorded golden files. These tests verify that the gateway correctly handles provider API responses without making actual API calls during CI.

## Golden Files Structure

```text
testdata/
├── openai/
│   ├── chat_completion.json           # Basic chat completion
│   ├── chat_completion_reasoning.json # o3-mini reasoning model
│   ├── chat_completion_stream.txt     # SSE streaming format
│   ├── chat_with_tools.json           # Function calling
│   ├── chat_json_mode.json            # Structured JSON output
│   ├── chat_with_params.json          # System prompt, temperature, stop sequences
│   ├── chat_multi_turn.json           # Multi-turn conversation
│   ├── chat_multimodal.json           # Image input
│   └── models.json                    # Models list
├── anthropic/
│   ├── messages.json                  # Basic messages
│   ├── messages_stream.txt            # SSE streaming format
│   ├── messages_with_params.json      # System prompt, temperature, stop sequences
│   ├── messages_with_tools.json       # Tool use
│   ├── messages_extended_thinking.json # Extended thinking (reasoning)
│   ├── messages_multi_turn.json       # Multi-turn conversation
│   └── messages_multimodal.json       # Image input
├── gemini/
│   ├── chat_completion.json           # Basic chat (OpenAI-compatible)
│   ├── chat_completion_stream.txt     # SSE streaming format
│   ├── chat_with_params.json          # System prompt, temperature
│   ├── chat_with_tools.json           # Function calling
│   └── models.json                    # Models list
├── xai/
│   ├── chat_completion.json           # Basic chat (OpenAI-compatible)
│   ├── chat_completion_stream.txt     # SSE streaming format
│   ├── chat_with_params.json          # System prompt, temperature
│   └── models.json                    # Models list
└── groq/
    ├── chat_completion.json           # Basic chat (OpenAI-compatible)
    ├── chat_completion_stream.txt     # SSE streaming format
    ├── chat_with_params.json          # System prompt, temperature
    ├── chat_with_tools.json           # Function calling
    └── models.json                    # Models list
```

## Running Contract Tests

```bash
# Run all contract tests
go test -v -tags=contract ./tests/contract/...

# Run specific provider tests
go test -v -tags=contract ./tests/contract/... -run TestOpenAI
go test -v -tags=contract ./tests/contract/... -run TestAnthropic
go test -v -tags=contract ./tests/contract/... -run TestGemini
go test -v -tags=contract ./tests/contract/... -run TestXAI
go test -v -tags=contract ./tests/contract/... -run TestGroq
```

## Recording Golden Files

Use the `recordapi` tool or curl commands below to record fresh golden files.

### Using recordapi Tool

```bash
# OpenAI
go run ./cmd/recordapi -provider=openai -endpoint=chat -output=tests/contract/testdata/openai/chat_completion.json
go run ./cmd/recordapi -provider=openai -endpoint=chat_stream -output=tests/contract/testdata/openai/chat_completion_stream.txt
go run ./cmd/recordapi -provider=openai -endpoint=models -output=tests/contract/testdata/openai/models.json

# Anthropic
go run ./cmd/recordapi -provider=anthropic -endpoint=chat -model=claude-sonnet-4-20250514 -output=tests/contract/testdata/anthropic/messages.json

# Gemini
go run ./cmd/recordapi -provider=gemini -endpoint=chat -model=gemini-2.5-flash-preview-09-2025 -output=tests/contract/testdata/gemini/chat_completion.json

# Groq
go run ./cmd/recordapi -provider=groq -endpoint=chat -model=llama-3.3-70b-versatile -output=tests/contract/testdata/groq/chat_completion.json

# xAI
go run ./cmd/recordapi -provider=xai -endpoint=chat -model=grok-3-mini -output=tests/contract/testdata/xai/chat_completion.json
```

### Curl Commands Reference

#### OpenAI

```bash
# Basic chat completion
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50}'

# Streaming
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50,"stream":true}'

# Models list
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"

# Reasoning model (o3-mini)
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"o3-mini","messages":[{"role":"user","content":"What is 2+2?"}],"max_completion_tokens":100}'

# Function calling
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"What is the weather in Paris?"}],
    "tools":[{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object","properties":{"location":{"type":"string"}}}}}],
    "tool_choice":"auto"
  }'

# JSON mode
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{"role":"user","content":"List 3 colors as JSON array"}],
    "response_format":{"type":"json_object"}
  }'

# With parameters (system prompt, temperature, stop sequences)
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[
      {"role":"system","content":"You are a helpful assistant. Be concise."},
      {"role":"user","content":"Count from 1 to 10"}
    ],
    "temperature":0.7,
    "top_p":0.9,
    "max_tokens":100,
    "stop":["5","five"]
  }'

# Multi-turn conversation
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[
      {"role":"system","content":"You are a math tutor."},
      {"role":"user","content":"What is 5+3?"},
      {"role":"assistant","content":"5+3 equals 8."},
      {"role":"user","content":"And if I add 2 more?"}
    ],
    "max_tokens":50
  }'

# Multimodal (image input)
curl https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gpt-4o-mini",
    "messages":[{
      "role":"user",
      "content":[
        {"type":"text","text":"What colors do you see in this image?"},
        {"type":"image_url","image_url":{"url":"https://www.google.com/images/branding/googlelogo/2x/googlelogo_color_272x92dp.png"}}
      ]
    }],
    "max_tokens":100
  }'
```

#### Anthropic

```bash
# Basic messages
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","max_tokens":50,"messages":[{"role":"user","content":"Say Hello World"}]}'

# Streaming
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","max_tokens":50,"stream":true,"messages":[{"role":"user","content":"Say Hello World"}]}'

# With parameters (system prompt, temperature, stop sequences)
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-20250514",
    "system":"You are a helpful assistant. Be concise.",
    "messages":[{"role":"user","content":"Count from 1 to 5"}],
    "max_tokens":100,
    "temperature":0.7,
    "top_p":0.9,
    "stop_sequences":["3"]
  }'

# Tool use
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-20250514",
    "messages":[{"role":"user","content":"What is the weather in Paris?"}],
    "tools":[{"name":"get_weather","description":"Get weather for a location","input_schema":{"type":"object","properties":{"location":{"type":"string"}},"required":["location"]}}],
    "max_tokens":200
  }'

# Extended thinking (reasoning)
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-20250514",
    "messages":[{"role":"user","content":"Solve: If x + 5 = 12, what is x?"}],
    "max_tokens":2000,
    "thinking":{"type":"enabled","budget_tokens":1024}
  }'

# Multi-turn conversation
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-20250514",
    "messages":[
      {"role":"user","content":"What is 5+3?"},
      {"role":"assistant","content":"8"},
      {"role":"user","content":"Multiply that by 2"}
    ],
    "max_tokens":50
  }'

# Multimodal (image input)
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"claude-sonnet-4-20250514",
    "messages":[{
      "role":"user",
      "content":[
        {"type":"text","text":"What colors do you see in this logo?"},
        {"type":"image","source":{"type":"url","url":"https://www.google.com/images/branding/googlelogo/2x/googlelogo_color_272x92dp.png"}}
      ]
    }],
    "max_tokens":100
  }'
```

#### Gemini (OpenAI-compatible endpoint)

```bash
# Basic chat completion
curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
  -H "Authorization: Bearer $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-2.5-flash-preview-09-2025","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50}'

# Streaming
curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
  -H "Authorization: Bearer $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gemini-2.5-flash-preview-09-2025","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50,"stream":true}'

# Models list
curl "https://generativelanguage.googleapis.com/v1beta/openai/models" \
  -H "Authorization: Bearer $GEMINI_API_KEY"

# With parameters
curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
  -H "Authorization: Bearer $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gemini-2.5-flash-preview-09-2025",
    "messages":[
      {"role":"system","content":"You are concise."},
      {"role":"user","content":"Name 3 planets"}
    ],
    "temperature":0.5,
    "max_tokens":50
  }'

# Function calling
curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
  -H "Authorization: Bearer $GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"gemini-2.5-flash-preview-09-2025",
    "messages":[{"role":"user","content":"What is the weather in Tokyo?"}],
    "tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object","properties":{"location":{"type":"string"}}}}}],
    "max_tokens":200
  }'
```

#### xAI (Grok)

```bash
# Basic chat completion
curl https://api.x.ai/v1/chat/completions \
  -H "Authorization: Bearer $XAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"grok-3-mini","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50}'

# Streaming
curl https://api.x.ai/v1/chat/completions \
  -H "Authorization: Bearer $XAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"grok-3-mini","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50,"stream":true}'

# Models list
curl https://api.x.ai/v1/models \
  -H "Authorization: Bearer $XAI_API_KEY"

# With parameters
curl https://api.x.ai/v1/chat/completions \
  -H "Authorization: Bearer $XAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"grok-3-mini",
    "messages":[
      {"role":"system","content":"You are concise."},
      {"role":"user","content":"Name 3 planets"}
    ],
    "temperature":0.5,
    "max_tokens":50
  }'
```

#### Groq

```bash
# Basic chat completion
curl https://api.groq.com/openai/v1/chat/completions \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-3.3-70b-versatile","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50}'

# Streaming
curl https://api.groq.com/openai/v1/chat/completions \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"llama-3.3-70b-versatile","messages":[{"role":"user","content":"Say Hello World"}],"max_tokens":50,"stream":true}'

# Models list
curl https://api.groq.com/openai/v1/models \
  -H "Authorization: Bearer $GROQ_API_KEY"

# With parameters
curl https://api.groq.com/openai/v1/chat/completions \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"llama-3.3-70b-versatile",
    "messages":[
      {"role":"system","content":"You are concise."},
      {"role":"user","content":"Name 3 planets"}
    ],
    "temperature":0.5,
    "max_tokens":50
  }'

# Function calling
curl https://api.groq.com/openai/v1/chat/completions \
  -H "Authorization: Bearer $GROQ_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model":"llama-3.3-70b-versatile",
    "messages":[{"role":"user","content":"What is the weather in London?"}],
    "tools":[{"type":"function","function":{"name":"get_weather","parameters":{"type":"object","properties":{"location":{"type":"string"}}}}}],
    "max_tokens":200
  }'
```

## Models Used

| Provider  | Model                            | Notes                                |
| --------- | -------------------------------- | ------------------------------------ |
| OpenAI    | gpt-4o-mini                      | Standard chat model                  |
| OpenAI    | o3-mini                          | Reasoning model with thinking tokens |
| Anthropic | claude-sonnet-4-20250514         | Latest Claude model                  |
| Gemini    | gemini-2.5-flash-preview-09-2025 | Preview flash model                  |
| xAI       | grok-3-mini                      | Latest Grok model                    |
| Groq      | llama-3.3-70b-versatile          | Fast inference Llama model           |

## Test Coverage

| Feature        | OpenAI | Anthropic | Gemini | xAI | Groq |
| -------------- | ------ | --------- | ------ | --- | ---- |
| Basic chat     | ✅     | ✅        | ✅     | ✅  | ✅   |
| Streaming      | ✅     | ✅        | ✅     | ✅  | ✅   |
| Models list    | ✅     | -         | ✅     | ✅  | ✅   |
| Tool calling   | ✅     | ✅        | ✅     | -   | ✅   |
| System prompt  | ✅     | ✅        | ✅     | ✅  | ✅   |
| Temperature    | ✅     | ✅        | ✅     | ✅  | ✅   |
| Stop sequences | ✅     | ✅        | -      | -   | -    |
| Multi-turn     | ✅     | ✅        | -      | -   | -    |
| Multimodal     | ✅     | ✅        | -      | -   | -    |
| JSON mode      | ✅     | -         | -      | -   | -    |
| Reasoning      | ✅     | ✅        | -      | -   | -    |
