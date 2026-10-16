package mailreport

import (
	"context"
	"strings"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/strop/pkg/openaibatch"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// EffectiveEssenceEscalateModel returns escalate id when distinct from essence, else "".
func EffectiveEssenceEscalateModel(essenceModel, escalateModel string) string {
	essenceModel = strings.TrimSpace(essenceModel)
	escalateModel = strings.TrimSpace(escalateModel)
	if escalateModel == "" || escalateModel == essenceModel {
		return ""
	}
	return escalateModel
}

func createEssenceIntegrityShellPolypus(client *polypus.Client, essenceModel, escalateModel string) (*integrity.Shell, error) {
	const op = "mailreport.createEssenceIntegrityShellPolypus"
	if client == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	essenceModel = strings.TrimSpace(essenceModel)
	if essenceModel == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty model")
	}
	escalateModel = EffectiveEssenceEscalateModel(essenceModel, escalateModel)
	cfg := integrity.ShellConfig{
		Chat: func(ctx context.Context, messages []integrity.Message) (string, error) {
			return client.Chat(ctx, polypus.ChatRequest{
				Model:       essenceModel,
				Temperature: 0,
				Messages:    integrityMessagesToPolypus(messages),
			})
		},
		Validate:     essenceIntegrityValidate,
		CompleteNote: essenceIntegrityCompleteNote,
		MaxAttempts:  essenceParseMaxAttempts,
	}
	if escalateModel != "" {
		cfg.Compensate = essenceCompensatePolypus(client, escalateModel)
	}
	return integrity.CreateShell(cfg)
}

func createEssenceIntegrityShellChatLines(runner ChatLineRunner, ln openaibatch.ChatLine, escalateModel string) (*integrity.Shell, error) {
	const op = "mailreport.createEssenceIntegrityShellChatLines"
	if runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	escalateModel = EffectiveEssenceEscalateModel(ln.Model, escalateModel)
	cfg := integrity.ShellConfig{
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
	}
	if escalateModel != "" {
		cfg.Compensate = essenceCompensateChatLines(runner, ln, escalateModel)
	}
	return integrity.CreateShell(cfg)
}

func essenceCompensatePolypus(client *polypus.Client, escalateModel string) integrity.CompensateFunc {
	return func(ctx context.Context, messages []integrity.Message, lastRaw string, lastErr error) (string, error) {
		msgs := essenceCompensateMessages(messages, lastRaw, lastErr)
		return client.Chat(ctx, polypus.ChatRequest{
			Model:       escalateModel,
			Temperature: 0,
			Messages:    integrityMessagesToPolypus(msgs),
		})
	}
}

func essenceCompensateChatLines(runner ChatLineRunner, ln openaibatch.ChatLine, escalateModel string) integrity.CompensateFunc {
	const op = "mailreport.essenceCompensateChatLines"
	return func(ctx context.Context, messages []integrity.Message, lastRaw string, lastErr error) (string, error) {
		msgs := essenceCompensateMessages(messages, lastRaw, lastErr)
		line := openaibatch.ChatLine{
			CustomID: ln.CustomID,
			Model:    escalateModel,
			Messages: integrityMessagesToMaps(msgs),
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
	}
}

func essenceCompensateMessages(messages []integrity.Message, lastRaw string, lastErr error) []integrity.Message {
	out := append([]integrity.Message{}, messages...)
	out = append(out,
		integrity.Message{Role: integrity.RoleAssistant, Content: lastRaw},
		integrity.Message{Role: integrity.RoleUser, Content: essenceIntegrityCompleteNote(lastErr)},
	)
	return out
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
