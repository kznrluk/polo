package chat

import (
	"context"
	"fmt"
	"github.com/fatih/color"
	"github.com/kznrluk/aski/config"
	"github.com/kznrluk/aski/conv"
	"github.com/kznrluk/go-anthropic"
	"github.com/sashabaranov/go-openai"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func RetrieveResponse(isRestMode bool, cfg config.Config, conv conv.Conversation) (string, error) {
	cancelCtx, cancelFunc := createCancellableContext()
	defer cancelFunc()

	if isClaude(conv.GetProfile().Model) {
		ac := anthropic.NewClient(cfg.AnthropicAPIKey)
		return StreamClaudeExperimental(cancelCtx, ac, conv)
	}

	oc := openai.NewClient(cfg.OpenAIAPIKey)
	if isRestMode {
		return Rest(cancelCtx, oc, conv)
	}
	return Stream(cancelCtx, oc, conv)
}

func isClaude(model string) bool {
	return model == string(anthropic.Claude3Opus20240229)
}

func GetSummary(cfg config.Config, conv conv.Conversation) string {
	oc := openai.NewClient(cfg.OpenAIAPIKey)

	c := ""
	messages := conv.GetMessages()
	start := len(messages) - 2
	if start < 0 {
		start = 0
	}

	for i := start; i < len(messages); i++ {
		msg := messages[i]
		if msg.Role == openai.ChatMessageRoleSystem {
			continue
		}

		c += msg.Role + " says :" + msg.Content + "\n"
	}

	stream, err := oc.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: c,
					Name:    "User",
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Give only this conversation a short title in the language of the conversation in one line, without symbols.",
					Name:    "Aski",
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return ""
	}

	blue := color.New(color.FgHiBlue).SprintFunc()

	data := ""
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return ""
			}
		}

		fmt.Printf("%s", blue(resp.Choices[0].Delta.Content))
		data += resp.Choices[0].Delta.Content
	}

	data = strings.ReplaceAll(data, ".", "")
	data = strings.ReplaceAll(data, "\"", "")

	return data
}

func Rest(ctx context.Context, oc *openai.Client, conv conv.Conversation) (string, error) {
	profile := conv.GetProfile()
	customParams := profile.CustomParameters
	resp, err := oc.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:            profile.Model,
			Messages:         conv.ToChatCompletionMessage(),
			ResponseFormat:   profile.GetResponseFormat(),
			MaxTokens:        customParams.MaxTokens,
			Temperature:      customParams.Temperature,
			TopP:             customParams.TopP,
			Stop:             customParams.Stop,
			PresencePenalty:  customParams.PresencePenalty,
			FrequencyPenalty: customParams.FrequencyPenalty,
			LogitBias:        customParams.LogitBias,
		},
	)

	if err != nil {
		return "", err
	}
	fmt.Printf("%s", resp.Choices[0].Message.Content)
	return resp.Choices[0].Message.Content, nil
}

func Stream(ctx context.Context, oc *openai.Client, conv conv.Conversation) (string, error) {
	profile := conv.GetProfile()
	customParams := profile.CustomParameters
	stream, err := oc.CreateChatCompletionStream(
		ctx,
		openai.ChatCompletionRequest{
			Model:            profile.Model,
			Messages:         conv.ToChatCompletionMessage(),
			ResponseFormat:   profile.GetResponseFormat(),
			MaxTokens:        customParams.MaxTokens,
			Temperature:      customParams.Temperature,
			TopP:             customParams.TopP,
			Stop:             customParams.Stop,
			PresencePenalty:  customParams.PresencePenalty,
			FrequencyPenalty: customParams.FrequencyPenalty,
			LogitBias:        customParams.LogitBias,
		},
	)

	if err != nil {
		return "", err
	}

	data := ""
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return "", err
			}
		}

		fmt.Printf("%s", resp.Choices[0].Delta.Content)
		data += resp.Choices[0].Delta.Content
	}
	return data, nil
}

func createCancellableContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT)

		select {
		case <-sigChan:
			println()
			cancel()
		case <-ctx.Done():
		}

		signal.Stop(sigChan)
	}()

	return ctx, cancel
}

// this will be refactored in the future
func StreamClaudeExperimental(ctx context.Context, ac *anthropic.Client, conv conv.Conversation) (string, error) {
	stream, err := ac.CreateMessageStream(anthropic.MessageRequest{
		MaxTokens: 4096,
		Model:     anthropic.Claude3Opus20240229,
		System:    conv.GetProfile().SystemContext,
		Messages:  conv.ToAnthropicMessage(),
	})

	if err != nil {
		return "", err
	}

	data := ""
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return "", err
			}
		}

		fmt.Printf("%s", resp.Delta.Text)
		data += resp.Delta.Text
	}
	return data, nil
}
