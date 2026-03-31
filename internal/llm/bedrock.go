package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// bedrockRegions lists AWS regions known to support Bedrock.
var bedrockRegions = []string{
	"us-east-1",
	"us-east-2",
	"us-west-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-central-1",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-south-1",
	"ca-central-1",
	"sa-east-1",
}

// BedrockProvider implements Provider for AWS Bedrock using the Converse API.
type BedrockProvider struct {
	client *bedrockruntime.Client
	region string
}

// NewBedrockProvider creates a BedrockProvider using the AWS credential chain.
// It validates the region supports Bedrock before creating the client.
func NewBedrockProvider(opts ProviderOptions) (Provider, error) {
	region := opts.Region
	if region == "" {
		region = "us-east-1"
	}

	if !isBedrockRegion(region) {
		return nil, &ProviderError{
			Provider: "bedrock",
			Message:  fmt.Sprintf("region %q does not support Bedrock", region),
		}
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if opts.Profile != "" {
		loadOpts = append(loadOpts, config.WithSharedConfigProfile(opts.Profile))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, &ProviderError{
			Provider: "bedrock",
			Message:  "failed to load AWS config: " + err.Error(),
			Err:      err,
		}
	}

	client := bedrockruntime.NewFromConfig(cfg)
	return &BedrockProvider{
		client: client,
		region: region,
	}, nil
}

// Name returns the provider identifier.
func (p *BedrockProvider) Name() string { return "bedrock" }

// Invoke sends a prompt to the model via the Bedrock Converse API and returns
// the text response. It retries once on throttling or timeout errors.
func (p *BedrockProvider) Invoke(ctx context.Context, prompt, modelID string) (string, error) {
	input := &bedrockruntime.ConverseInput{
		ModelId: &modelID,
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{Value: prompt},
				},
			},
		},
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			slog.Debug("bedrock: retrying after error", slog.Int("attempt", attempt+1), slog.String("error", lastErr.Error()))
			select {
			case <-ctx.Done():
				return "", &ProviderError{
					Provider: "bedrock",
					Message:  "context cancelled during retry",
					Err:      ctx.Err(),
				}
			case <-time.After(1 * time.Second):
			}
		}

		output, err := p.client.Converse(ctx, input)
		if err != nil {
			lastErr = err
			if isRetryableBedrockError(err) && attempt == 0 {
				continue
			}
			return "", &ProviderError{
				Provider: "bedrock",
				Message:  fmt.Sprintf("invocation failed after %d attempt(s): %s", attempt+1, err.Error()),
				Err:      err,
			}
		}

		text, ok := extractConverseText(output)
		if !ok || strings.TrimSpace(text) == "" {
			return "", &ProviderError{
				Provider: "bedrock",
				Message:  "empty response from model",
			}
		}
		return text, nil
	}

	return "", &ProviderError{
		Provider: "bedrock",
		Message:  fmt.Sprintf("retries exhausted: %s", lastErr),
		Err:      lastErr,
	}
}

// extractConverseText pulls the text from the first content block of a Converse response.
func extractConverseText(output *bedrockruntime.ConverseOutput) (string, bool) {
	if output == nil || output.Output == nil {
		return "", false
	}
	msg, ok := output.Output.(*types.ConverseOutputMemberMessage)
	if !ok || msg == nil {
		return "", false
	}
	if len(msg.Value.Content) == 0 {
		return "", false
	}
	textBlock, ok := msg.Value.Content[0].(*types.ContentBlockMemberText)
	if !ok {
		return "", false
	}
	return textBlock.Value, true
}

// isRetryableBedrockError checks if the error is a throttling or timeout error.
func isRetryableBedrockError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "ThrottlingException") ||
		strings.Contains(msg, "TooManyRequestsException") ||
		strings.Contains(msg, "ServiceUnavailableException") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "Timeout")
}

// isBedrockRegion checks if the given region is in the known Bedrock regions list.
func isBedrockRegion(region string) bool {
	for _, r := range bedrockRegions {
		if r == region {
			return true
		}
	}
	return false
}
