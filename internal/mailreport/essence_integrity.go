package mailreport

import (
	"context"
	"strings"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/strop/pkg/openaibatch"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func createEssenceIntegrityShellPolypus(client *polypus.Client, model string) (*integrity.Shell, error) {
	const op = "mailreport.createEssenceIntegrityShellPolypus"
	if client == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty model")
	}
	return integrity.CreateShell(integrity.ShellConfig{
		Chat: func(ctx context.Context, messages []integrity.Message) (string, error) {
			return client.Chat(ctx, polypus.ChatRequest{
				Model:       model,
				Temperature: 0,
				Messages:    integrityMessagesToPolypus(messages),
			})
		},
		Validate:     essenceIntegrityValidate,
		CompleteNote: essenceIntegrityCompleteNote,
		MaxAttempts:  essenceParseMaxAttempts,
	})
}

func createEssenceIntegrityShellChatLines(runner ChatLineRunner, ln openaibatch.ChatLine) (*integrity.Shell, error) {
	const op = "mailreport.createEssenceIntegrityShellChatLines"
	if runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	return integrity.CreateShell(integrity.ShellConfig{
		Chat: func(ctx context.Context, messages []integrity.Message) (string, error) {
			line := openaibatch.ChatLine{
				CustomID: ln.CustomID,
				Model:    ln.Model,
				Messages: integrityMessagesToMaps(messages),
			}
			results, err := runner.RunChatLines(ctx, []openaibatch.ChatLine{line})
			if err != nil {
				return "", err
			}
			lr, ok := results[ln.CustomID]
			if !ok {
				return "", sirerr.New(sirerr.CodeAI, op, "missing chat line result")
			}
			if strings.TrimSpace(lr.ErrMessage) != "" {
				return "", sirerr.New(sirerr.CodeAI, op, lr.ErrMessage)
			}
			return lr.Content, nil
		},
		Validate:     essenceIntegrityValidate,
		CompleteNote: essenceIntegrityCompleteNote,
		MaxAttempts:  essenceParseMaxAttempts,
	})
}

func integrityMessagesToPolypus(messages []integrity.Message) []polypus.ChatMessage {
	out := make([]polypus.ChatMessage, 0, len(messages))
	for _, m := range messages {
		out = append(out, polypus.ChatMessage{Role: m.Role, Content: m.Content})
	}
	return out
}

func integrityMessagesToMaps(messages []integrity.Message) []map[string]string {
	out := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, map[string]string{chatRoleKey: m.Role, chatContentKey: m.Content})
	}
	return out
}

func chatLineToIntegrityMessages(ln openaibatch.ChatLine) []integrity.Message {
	out := make([]integrity.Message, 0, len(ln.Messages))
	for _, m := range ln.Messages {
		out = append(out, integrity.Message{
			Role:    m[chatRoleKey],
			Content: m[chatContentKey],
		})
	}
	return out
}
