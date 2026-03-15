# AWS Bedrock — MiniMax Integration

## What Changed (commits d4b5367 → 9a7ef15 → 69bd4f3)

### `d4b5367` — Switch default backend to OpenRouter + minimax/minimax-m2.5

**Files:** `internal/llm/openrouter.go` *(new)*, `internal/llm/client.go`, `internal/llm/models.go`, `internal/config/global.go`

- Added `OpenRouterProvider` — pure `net/http` OpenAI-compatible client with SSE streaming. No extra deps.
- `client.go`: `Client` type alias now points to `OpenRouterProvider`. `New(apiKey)` calls `NewOpenRouter(apiKey, "")`.
- `NewProvider()` switch: `"" | "openrouter" | "openai-compat"` → `NewOpenRouter`, `"anthropic"` → `NewAnthropic`, `"bedrock"` → `NewBedrock`.
- `models.go`: `ModelDefault = ModelMiniMax = "minimax/minimax-m2.5"`. `ModelHaiku`/`ModelSonnet` aliased to MiniMax; `ModelOpus` → `"anthropic/claude-opus-4-5"` via OpenRouter.
- `config/global.go`: `APIKey()` checks `OPENROUTER_API_KEY` first (env → Keychain), then `ANTHROPIC_API_KEY` as fallback. `ProviderConfigFor()` defaults to `"openrouter"`.
- **MiniMax quirk fix:** `content` field is a `*string` (pointer) — reasoning models return `null` when `max_tokens` is too low; enforces 512-token minimum in `Complete()`.

---

### `9a7ef15` — Correct Bedrock model ID constants

**Files:** `internal/llm/models.go`, `bedrock.md` *(new)*

Replaced the stale `eu.minimax.minimax-text-01-v1:0` constant with the correct AWS dot-notation IDs:

```go
BedrockModelMiniMaxM2  = "minimax.minimax-m2"   // GA
BedrockModelMiniMaxM21 = "minimax.minimax-m2.1" // GA — added Feb 2026
BedrockModelMiniMaxM25 = "minimax.minimax-m2.5" // not yet confirmed in console
ModelBedrockMiniMax    = BedrockModelMiniMaxM21  // safe default until M2.5 is verified
```

---

### `69bd4f3` — Native AWS Bedrock provider (Converse API)

**Files:** `internal/llm/bedrock.go` *(new)*, `internal/llm/provider.go`, `internal/llm/client.go`, `internal/config/global.go`, `internal/commands/provider.go`, `go.mod`

Added `BedrockProvider` using the AWS SDK v2 Converse API as a no-OpenRouter path to MiniMax on Bedrock.

**New Go dependencies** (`go.mod`):
```
github.com/aws/aws-sdk-go-v2/service/bedrockruntime
github.com/aws/aws-sdk-go-v2/credentials
```

**`ProviderConfig`** (both `llm` and `config` packages) gained two new fields:
```go
SecretAccessKey string `yaml:"secret_access_key,omitempty"` // AWS_SECRET_ACCESS_KEY
Region          string `yaml:"region,omitempty"`             // default: eu-central-1
```

`commands/provider.go` passes these through when constructing `llm.ProviderConfig`.

**`NewBedrock(accessKeyID, secretAccessKey, region string)`** — builds a `bedrockruntime.Client` from static IAM credentials (`credentials.NewStaticCredentialsProvider`). Default region: `eu-central-1`.

**`Complete()`** — calls `client.Converse()` → extracts text from `ConverseOutputMemberMessage.Content`.

**`Stream()`** — calls `client.ConverseStream()` → ranges over `stream.Events()`, handles `ConverseStreamOutputMemberContentBlockDelta` → `ContentBlockDeltaMemberText`.

---

## Provider Reference

### All provider types

| `type` value | Constructor | Auth | Streaming |
|---|---|---|---|
| `openrouter` (default) | `NewOpenRouter(apiKey, baseURL)` | Bearer token | SSE `data:` lines |
| `openai-compat` | `NewOpenRouter(apiKey, baseURL)` | Bearer token | SSE `data:` lines |
| `anthropic` | `NewAnthropic(apiKey)` | Anthropic SDK | Anthropic SDK events |
| `bedrock` | `NewBedrock(keyID, secret, region)` | IAM SigV4 | `ConverseStream` events |

### Model ID formats

| Provider | Format | Example |
|---|---|---|
| OpenRouter | `{provider}/{model}` | `minimax/minimax-m2.5` |
| Bedrock Converse | `{provider}.{model}` | `minimax.minimax-m2.1` |
| Anthropic | bare model ID | `claude-sonnet-4-6` |

---

## Configuration

### `~/.naqb/config.yaml` — with both OpenRouter and Bedrock

```yaml
default_provider: openrouter

providers:
  openrouter:
    type: openrouter
    api_key: "sk-or-v1-..."
    base_url: "https://openrouter.ai/api/v1"

  bedrock:
    type: bedrock
    api_key: "AKIAIOSFODNN7EXAMPLE"        # AWS_ACCESS_KEY_ID
    secret_access_key: "wJalrXUtnFEMI/..."  # AWS_SECRET_ACCESS_KEY
    region: "us-east-1"
```

### `book.yaml` — route specific stages to Bedrock

```yaml
llm:
  write_provider: bedrock
  write_model: minimax.minimax-m2.1
  qa_provider: bedrock
  qa_model: minimax.minimax-m2.1
```

### CLI override (any command with `--provider`)

```bash
nqb write --chapter 3 --provider bedrock
nqb fix   --chapter 3 --provider bedrock
nqb qa    --chapter 3 --provider bedrock
```

### bedrock-mantle (OpenAI-compat, no IAM keys)

AWS also exposes an OpenAI-compatible endpoint. Use `openai-compat` type — it routes through `OpenRouterProvider` with a custom base URL, no AWS SDK involved:

```yaml
providers:
  bedrock-mantle:
    type: openai-compat
    api_key: "bdck-..."   # Bedrock API key from console, not IAM
    base_url: "https://bedrock-mantle.us-east-1.api.aws/v1"
```

---

## Model ID Constants

Defined in `internal/llm/models.go`:

```go
// OpenRouter (default provider)
llm.ModelDefault         // "minimax/minimax-m2.5"  — used by all stages
llm.ModelMiniMax         // "minimax/minimax-m2.5"
llm.ModelHaiku           // "minimax/minimax-m2.5"  (aliased)
llm.ModelSonnet          // "minimax/minimax-m2.5"  (aliased)
llm.ModelOpus            // "anthropic/claude-opus-4-5" — heavy reasoning

// Anthropic native (provider type: "anthropic")
llm.ModelAnthropicHaiku  // "claude-haiku-4-5-20251001"
llm.ModelAnthropicSonnet // "claude-sonnet-4-6"
llm.ModelAnthropicOpus   // "claude-opus-4-6"

// AWS Bedrock (provider type: "bedrock")
llm.BedrockModelMiniMaxM2   // "minimax.minimax-m2"   — GA
llm.BedrockModelMiniMaxM21  // "minimax.minimax-m2.1" — GA (Feb 2026)
llm.BedrockModelMiniMaxM25  // "minimax.minimax-m2.5" — not yet confirmed in console
llm.ModelBedrockMiniMax     // alias → BedrockModelMiniMaxM21 (safe default)
```

---

## Bedrock Setup

### 1. Enable model access

[AWS Bedrock console → Model access](https://console.aws.amazon.com/bedrock/home#/modelaccess) → Modify → enable **MiniMax M2** and **MiniMax M2.1**.

### 2. IAM policy (minimal)

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": [
      "bedrock:Converse",
      "bedrock:ConverseStream"
    ],
    "Resource": [
      "arn:aws:bedrock:*::foundation-model/minimax.minimax-m2",
      "arn:aws:bedrock:*::foundation-model/minimax.minimax-m2.1"
    ]
  }]
}
```

### 3. Supported regions

`us-east-1` · `us-east-2` · `us-west-2` · `eu-west-1` · `eu-west-2` · `eu-south-1` · `ap-northeast-1` · `ap-south-1` · `ap-southeast-2` · `sa-east-1`

Default region in `NewBedrock()`: `eu-central-1`.

---

## MiniMax M2.5 Availability

| Route | Model ID | Status |
|---|---|---|
| **OpenRouter** | `minimax/minimax-m2.5` | ✅ Available now |
| **AWS Bedrock** | `minimax.minimax-m2.5` | ⚠️ Not yet confirmed in console (March 2026) |
| **AWS Bedrock** | `minimax.minimax-m2.1` | ✅ GA (Feb 2026) |

Until `minimax.minimax-m2.5` appears in the Bedrock model access console, `ModelBedrockMiniMax` points to `BedrockModelMiniMaxM21`. Switch it to `BedrockModelMiniMaxM25` once enabled.

---

## Troubleshooting

**`AccessDeniedException`** — model access not enabled in the Bedrock console.

**`ValidationException: model not found`** — model ID wrong or not available in your region. Use `minimax.minimax-m2.1` which is confirmed GA.

**Empty / null content** — `max_tokens` too low. MiniMax reasoning models spend tokens on a thinking trace before output. `OpenRouterProvider.Complete()` enforces a 512-token floor; `BedrockProvider` defaults to 8192.

**Region mismatch** — model may not be available in `eu-central-1` (the `NewBedrock` default). Switch to `us-east-1`.
