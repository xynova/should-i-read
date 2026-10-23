package mailreport

import (
	"context"
	"strings"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/strop/pkg/openaibatch"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
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

func essenceIntegrityAfterMiss(err error) integrity.MissAction {
	missing, _ := essenceKeysFromErr(err)
	for _, key := range missing {
		if key == essenceKeyWhy1 {
			return integrity.MissRestart
		}
	}
	return integrity.MissContinue
}

// essenceShellHooks configures one Essence integrity shell (WHY, TAGS, or KIND).
type essenceShellHooks struct {
	Validate     func(string) error
	CompleteNote func(error) string
	AfterMiss    func(error) integrity.MissAction
}

func createEssenceIntegrityShellPolypus(client *polypus.Client, essenceModel, escalateModel string, hooks essenceShellHooks) (*integrity.Shell, error) {
	const op = "mailreport.createEssenceIntegrityShellPolypus"
	if client == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil client")
	}
	essenceModel = strings.TrimSpace(essenceModel)
	if essenceModel == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty model")
	}
	if hooks.Validate == nil || hooks.CompleteNote == nil || hooks.AfterMiss == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil essence shell hooks")
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
		Validate:     hooks.Validate,
		CompleteNote: hooks.CompleteNote,
		AfterMiss:    hooks.AfterMiss,
		MaxAttempts:  essenceParseMaxAttempts,
	}
	if escalateModel != "" {
		cfg.Compensate = essenceCompensateSteerPolypus(client, escalateModel)
	}
	return integrity.CreateShell(cfg)
}

func createEssenceIntegrityShellChatLines(runner ChatLineRunner, ln openaibatch.ChatLine, escalateModel string, tagCatalog *tags.Catalog) (*integrity.Shell, error) {
	const op = "mailreport.createEssenceIntegrityShellChatLines"
	if runner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil runner")
	}
	escalateModel = EffectiveEssenceEscalateModel(ln.Model, escalateModel)
	hooks := essenceFullFormShellHooks(tagCatalog)
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
		Validate:     hooks.Validate,
		CompleteNote: hooks.CompleteNote,
		AfterMiss:    hooks.AfterMiss,
		MaxAttempts:  essenceParseMaxAttempts,
	}
	if escalateModel != "" {
		cfg.Compensate = essenceCompensateSteerChatLines(runner, ln, escalateModel)
	}
	return integrity.CreateShell(cfg)
}

func essenceCompensateSteerPolypus(client *polypus.Client, escalateModel string) integrity.CompensateFunc {
	escalateChat := func(ctx context.Context, messages []integrity.Message) (string, error) {
		return client.Chat(ctx, polypus.ChatRequest{
			Model:       escalateModel,
			Temperature: 0,
			Messages:    integrityMessagesToPolypus(messages),
		})
	}
	return func(ctx context.Context, messages []integrity.Message, lastRaw string, lastErr error) (integrity.Steer, error) {
		_ = messages
		_ = lastRaw
		_ = lastErr
		return integrity.Steer{
			Action:   integrity.SteerReenter,
			Chat:     escalateChat,
			Restart:  true,
			Attempts: essenceParseMaxAttempts,
		}, nil
	}
}

func essenceCompensateSteerChatLines(runner ChatLineRunner, ln openaibatch.ChatLine, escalateModel string) integrity.CompensateFunc {
	const op = "mailreport.essenceCompensateSteerChatLines"
	escalateChat := func(ctx context.Context, messages []integrity.Message) (string, error) {
		line := openaibatch.ChatLine{
			CustomID: ln.CustomID,
			Model:    escalateModel,
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
	}
	return func(ctx context.Context, messages []integrity.Message, lastRaw string, lastErr error) (integrity.Steer, error) {
		_ = messages
		_ = lastRaw
		_ = lastErr
		return integrity.Steer{
			Action:   integrity.SteerReenter,
			Chat:     escalateChat,
			Restart:  true,
			Attempts: essenceParseMaxAttempts,
		}, nil
	}
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
