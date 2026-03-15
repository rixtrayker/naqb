package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/amr/naqb/internal/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// BedrockProvider implements Provider using AWS Bedrock's Converse API.
// It supports all models available via Bedrock cross-region inference profiles,
// including MiniMax M2.5 (eu.minimax.minimax-text-01-v1:0).
type BedrockProvider struct {
	client *bedrockruntime.Client
	region string
}

// NewBedrock creates a BedrockProvider using explicit AWS credentials.
// region should be the primary region (e.g. "eu-central-1").
func NewBedrock(accessKeyID, secretAccessKey, region string) *BedrockProvider {
	if region == "" {
		region = "eu-central-1"
	}
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
	cfg := aws.Config{
		Region:      region,
		Credentials: creds,
	}
	client := bedrockruntime.NewFromConfig(cfg)
	return &BedrockProvider{client: client, region: region}
}

// Complete sends a non-streaming Converse request to Bedrock.
func (p *BedrockProvider) Complete(ctx context.Context, model, system string, messages []Message, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	log.Debug("LLM complete", "provider", "bedrock", "model", model, "max_tokens", maxTokens)

	input := p.buildConverseInput(model, system, messages, maxTokens)
	out, err := p.client.Converse(ctx, input)
	if err != nil {
		return "", fmt.Errorf("bedrock converse: %w", err)
	}

	result, err := extractConverseText(out.Output)
	if err != nil {
		return "", err
	}
	log.Debug("LLM complete done", "provider", "bedrock", "model", model, "chars", len(result))
	return result, nil
}

// Stream sends a streaming ConverseStream request to Bedrock.
func (p *BedrockProvider) Stream(ctx context.Context, model, system string, messages []Message, maxTokens int, onDelta StreamFunc) (string, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	log.Debug("LLM stream start", "provider", "bedrock", "model", model)

	input := p.buildConverseInput(model, system, messages, maxTokens)
	out, err := p.client.ConverseStream(ctx, &bedrockruntime.ConverseStreamInput{
		ModelId:  input.ModelId,
		Messages: input.Messages,
		System:   input.System,
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(int32(maxTokens)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("bedrock converse stream: %w", err)
	}

	var full strings.Builder
	stream := out.GetStream()
	defer stream.Close()

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if delta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
				full.WriteString(delta.Value)
				if onDelta != nil {
					if err := onDelta(delta.Value); err != nil {
						return full.String(), err
					}
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return full.String(), fmt.Errorf("bedrock stream error: %w", err)
	}

	result := full.String()
	log.Debug("LLM stream done", "provider", "bedrock", "model", model, "chars", len(result))
	return result, nil
}

func (p *BedrockProvider) buildConverseInput(model, system string, messages []Message, maxTokens int) *bedrockruntime.ConverseInput {
	var sysBlocks []types.SystemContentBlock
	if system != "" {
		sysBlocks = append(sysBlocks, &types.SystemContentBlockMemberText{Value: system})
	}

	var msgs []types.Message
	for _, m := range messages {
		role := types.ConversationRoleUser
		if m.Role == "assistant" {
			role = types.ConversationRoleAssistant
		}
		msgs = append(msgs, types.Message{
			Role: role,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: m.Content},
			},
		})
	}

	return &bedrockruntime.ConverseInput{
		ModelId:  aws.String(model),
		Messages: msgs,
		System:   sysBlocks,
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(int32(maxTokens)),
		},
	}
}

func extractConverseText(output types.ConverseOutput) (string, error) {
	msg, ok := output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return "", fmt.Errorf("bedrock: unexpected output type %T", output)
	}
	var sb strings.Builder
	for _, block := range msg.Value.Content {
		if t, ok := block.(*types.ContentBlockMemberText); ok {
			sb.WriteString(t.Value)
		}
	}
	return sb.String(), nil
}


