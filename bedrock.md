# AWS Bedrock — MiniMax M2.5 Integration Guide

This guide documents how to connect **nqb** to MiniMax models running on AWS Bedrock as an alternative to OpenRouter.

---

## Why Bedrock?

| | OpenRouter | AWS Bedrock |
|---|---|---|
| **Setup** | Single API key | IAM credentials + model access grant |
| **Billing** | OpenRouter invoice | AWS invoice (pay-per-token) |
| **Latency** | Varies by routing | Consistent (in-region) |
| **Data residency** | OpenRouter infra | Stays in your AWS region |
| **MiniMax M2.5** | ✅ Available now | ⚠️ M2 / M2.1 confirmed; M2.5 pending |

---

## Available MiniMax Models on AWS Bedrock

| Model | Bedrock Model ID | Status (March 2026) |
|---|---|---|
| MiniMax M2 | `minimax.minimax-m2` | ✅ GA |
| MiniMax M2.1 | `minimax.minimax-m2.1` | ✅ GA (added Feb 2026) |
| MiniMax M2.5 | `minimax.minimax-m2.5` | ⚠️ Not yet confirmed — verify in console |

> **Note:** MiniMax M2.5 is available on OpenRouter as `minimax/minimax-m2.5`. On Bedrock, the latest confirmed model is M2.1. Check the [Bedrock model access console](https://console.aws.amazon.com/bedrock/home#/modelaccess) for the current list.

---

## How It Works

nqb uses two complementary approaches for Bedrock:

### Approach A — Converse API (AWS SDK v2) — `provider type: bedrock`

Uses the native AWS `bedrockruntime` SDK with the **Converse API**. Supports all Bedrock-listed models, proper SigV4 request signing, and ConverseStream for token-by-token output.

**Go package:** `github.com/aws/aws-sdk-go-v2/service/bedrockruntime`
**Auth:** `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` (static IAM credentials)

### Approach B — bedrock-mantle OpenAI-compatible endpoint — `provider type: openai-compat`

AWS exposes an OpenAI-compatible endpoint called **bedrock-mantle** that nqb's existing `OpenRouterProvider` can call directly. No AWS SDK needed — just a Bearer token.

**Base URL pattern:** `https://bedrock-mantle.{region}.api.aws/v1`
**Auth:** Bedrock API key from AWS console (short-lived token, not IAM keys)

---

## Setup

### Step 1 — Enable Model Access

1. Open the [AWS Bedrock console](https://console.aws.amazon.com/bedrock/home#/modelaccess)
2. Click **Modify model access**
3. Find **MiniMax** in the list → check **MiniMax M2.1** (and M2.5 when available)
4. Click **Save changes** — approval is usually instant for MiniMax

### Step 2 — Create IAM Credentials

Create an IAM user or role with this minimal policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "bedrock:InvokeModel",
        "bedrock:InvokeModelWithResponseStream",
        "bedrock:Converse",
        "bedrock:ConverseStream"
      ],
      "Resource": [
        "arn:aws:bedrock:*::foundation-model/minimax.minimax-m2",
        "arn:aws:bedrock:*::foundation-model/minimax.minimax-m2.1",
        "arn:aws:bedrock:*::foundation-model/minimax.minimax-m2.5"
      ]
    }
  ]
}
```

### Step 3 — Configure nqb

Add a `bedrock` provider block to `~/.naqb/config.yaml`:

```yaml
default_provider: openrouter          # keep OpenRouter as default

providers:
  openrouter:
    type: openrouter
    api_key: "sk-or-v1-..."
    base_url: "https://openrouter.ai/api/v1"

  bedrock:
    type: bedrock
    api_key: "AKIAIOSFODNN7EXAMPLE"           # AWS_ACCESS_KEY_ID
    secret_access_key: "wJalrXUtnFEMI/..."    # AWS_SECRET_ACCESS_KEY
    region: "us-east-1"                       # AWS region with model access
```

To use Bedrock for a specific book, add to `book.yaml`:

```yaml
llm:
  write_provider: bedrock
  write_model: minimax.minimax-m2.1
  qa_provider: bedrock
  qa_model: minimax.minimax-m2.1
```

Or override per-command:

```bash
nqb write --chapter 3 --provider bedrock
nqb fix --chapter 3 --provider bedrock
```

---

## Approach B — bedrock-mantle (OpenAI-compat, no AWS SDK)

If you prefer not to store IAM keys, AWS Bedrock provides an OpenAI-compatible endpoint via **Project Mantle**. Configure it as an `openai-compat` provider:

### Get a Bedrock API Key

```bash
aws bedrock create-api-key \
  --name "nqb-bedrock" \
  --region us-east-1
```

This returns a short-lived token (valid until rotated). Store it in your config:

```yaml
providers:
  bedrock-mantle:
    type: openai-compat
    api_key: "bdck-..."                          # Bedrock API key (not IAM)
    base_url: "https://bedrock-mantle.us-east-1.api.aws/v1"
```

Then set model as the Bedrock dot-notation ID:

```yaml
llm:
  write_provider: bedrock-mantle
  write_model: minimax.minimax-m2.1
```

The `openai-compat` type routes through `OpenRouterProvider` with the custom base URL — no code changes needed.

---

## Supported AWS Regions

MiniMax models are available in these Bedrock regions (in-region inference):

| Region | Name |
|---|---|
| `us-east-1` | US East (N. Virginia) |
| `us-east-2` | US East (Ohio) |
| `us-west-2` | US West (Oregon) |
| `eu-west-1` | Europe (Ireland) |
| `eu-west-2` | Europe (London) |
| `eu-south-1` | Europe (Milan) |
| `ap-northeast-1` | Asia Pacific (Tokyo) |
| `ap-south-1` | Asia Pacific (Mumbai) |
| `ap-southeast-2` | Asia Pacific (Sydney) |
| `sa-east-1` | South America (São Paulo) |

---

## Code Reference

### Go — direct Bedrock provider usage

```go
import "github.com/amr/naqb/internal/llm"

// Converse API (AWS SDK v2)
provider := llm.NewBedrock(
    "AKIAIOSFODNN7EXAMPLE",   // access key ID
    "wJalrXUtnFEMI/...",      // secret access key
    "us-east-1",              // region
)

resp, err := provider.Complete(ctx,
    llm.BedrockModelMiniMaxM21,   // "minimax.minimax-m2.1"
    "You are an expert author.",
    []llm.Message{{Role: "user", Content: "Write the intro."}},
    4096,
)
```

### Go — bedrock-mantle (OpenAI-compat, no IAM keys)

```go
import "github.com/amr/naqb/internal/llm"

provider := llm.NewOpenRouter(
    "bdck-YOUR_BEDROCK_API_KEY",
    "https://bedrock-mantle.us-east-1.api.aws/v1",
)

resp, err := provider.Complete(ctx,
    "minimax.minimax-m2.1",
    "You are an expert author.",
    []llm.Message{{Role: "user", Content: "Write the intro."}},
    4096,
)
```

### Model ID constants

```go
// In internal/llm/models.go
llm.BedrockModelMiniMaxM2   // "minimax.minimax-m2"   — GA
llm.BedrockModelMiniMaxM21  // "minimax.minimax-m2.1" — GA (Feb 2026)
llm.BedrockModelMiniMaxM25  // "minimax.minimax-m2.5" — pending confirmation
llm.ModelBedrockMiniMax     // alias → BedrockModelMiniMaxM25 (update when GA)
```

---

## API Comparison — Converse vs bedrock-mantle

| | Converse API (SDK) | bedrock-mantle (OpenAI-compat) |
|---|---|---|
| **Auth** | IAM SigV4 | Bearer token (Bedrock API key) |
| **Go deps** | `aws-sdk-go-v2/service/bedrockruntime` | `net/http` only |
| **Streaming** | ConverseStream (typed events) | SSE `data:` lines |
| **System prompt** | `System []SystemContentBlock` | `{"role":"system","content":"..."}` |
| **Model ID format** | `minimax.minimax-m2.1` | `minimax.minimax-m2.1` (same) |
| **nqb provider type** | `bedrock` | `openai-compat` |

---

## Troubleshooting

**`AccessDeniedException` on Converse call**
→ Model access not enabled. Go to Bedrock console → Model access → enable MiniMax.

**`ValidationException: model not found`**
→ Model ID typo or model not yet in your region. Use `minimax.minimax-m2.1` (confirmed GA) instead of `minimax.minimax-m2.5`.

**`minimax.minimax-m2.5` not listed in console**
→ M2.5 is only confirmed on OpenRouter as of March 2026. Use OpenRouter (`minimax/minimax-m2.5`) or use `minimax.minimax-m2.1` on Bedrock.

**Empty response / null content**
→ `max_tokens` too low. MiniMax reasoning models use tokens for the thinking trace before producing output. The `BedrockProvider` enforces a 512-token minimum automatically.

**Credential expiry on bedrock-mantle**
→ Bedrock API keys are short-lived. Rotate with `aws bedrock create-api-key` and update `~/.naqb/config.yaml`.

---

## Sources

- [MiniMax M2 — Amazon Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/model-card-minimax-minimax-m2.html)
- [MiniMax models — Amazon Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/model-cards-minimax.html)
- [Amazon Bedrock adds six open-weight models (Feb 2026)](https://aws.amazon.com/about-aws/whats-new/2026/02/amazon-bedrock-adds-support-six-open-weights-models/)
- [Supported foundation models — Amazon Bedrock](https://docs.aws.amazon.com/bedrock/latest/userguide/models-supported.html)
- [bedrockruntime Go package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/bedrockruntime)
