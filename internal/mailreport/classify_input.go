package mailreport

import (
	"strings"

	"github.com/behaviorengineering/strop/pkg/dspy/runner"
)

// ClassifyInput is the JobRunner generator input for one message.
type ClassifyInput struct {
	ObjectHash string
	Text       string
	version    int
}

// CreateClassifyInput builds input for mail_classify Generate.
func CreateClassifyInput(objectHash, text string) ClassifyInput {
	return ClassifyInput{
		ObjectHash: strings.TrimSpace(objectHash),
		Text:       strings.TrimSpace(text),
		version:    1,
	}
}

// ToMap implements runner.GeneratorInput.
func (in ClassifyInput) ToMap() map[string]interface{} {
	return map[string]interface{}{
		fieldObjectHash: in.ObjectHash,
		fieldText:       in.Text,
	}
}

// GetVersion implements runner.GeneratorInput.
func (in ClassifyInput) GetVersion() int {
	if in.version <= 0 {
		return 1
	}
	return in.version
}

// classifyGenerationConfig returns strop GenerationConfig for mail_classify.
func classifyGenerationConfig() runner.GenerationConfig {
	return runner.GenerationConfig{
		ModuleName:   JobMailClassify,
		JobName:      JobMailClassify,
		StepName:     StepMailClassify,
		ErrorCode:    "MAIL_CLASSIFY",
		ErrorMessage: "mail classify",
	}
}
