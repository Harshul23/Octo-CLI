package intelligence

import (
	"context"
	"fmt"
	"sort"
)

// DecisionOption is an evidence-backed candidate for a bounded decision.
// Providers may select an option, but they must not invent a value outside
// the candidate set.
type DecisionOption struct {
	ID         string     `json:"id" yaml:"id"`
	Value      string     `json:"value" yaml:"value"`
	Confidence float64    `json:"confidence" yaml:"confidence"`
	Evidence   []Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// DecisionRequest describes one bounded decision Octo needs resolved.
type DecisionRequest struct {
	Name    string           `json:"name" yaml:"name"`
	Options []DecisionOption `json:"options" yaml:"options"`
}

// DecisionResult records the selected evidence-backed option.
type DecisionResult struct {
	OptionID   string  `json:"option_id" yaml:"option_id"`
	Value      string  `json:"value" yaml:"value"`
	Confidence float64 `json:"confidence" yaml:"confidence"`
	Reason     string  `json:"reason" yaml:"reason"`
}

// DecisionProvider resolves bounded ambiguity without owning repository
// discovery or execution.
type DecisionProvider interface {
	Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error)
}

// DeterministicDecisionProvider selects the strongest evidence-backed option.
// It is the offline baseline and requires no external service.
type DeterministicDecisionProvider struct{}

func (DeterministicDecisionProvider) Decide(ctx context.Context, request DecisionRequest) (DecisionResult, error) {
	if err := ctx.Err(); err != nil {
		return DecisionResult{}, err
	}
	if request.Name == "" {
		return DecisionResult{}, fmt.Errorf("decision name is required")
	}
	if len(request.Options) == 0 {
		return DecisionResult{}, fmt.Errorf("decision %q has no options", request.Name)
	}

	options := append([]DecisionOption(nil), request.Options...)
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Confidence != options[j].Confidence {
			return options[i].Confidence > options[j].Confidence
		}
		return options[i].ID < options[j].ID
	})

	selected := options[0]
	if selected.ID == "" {
		return DecisionResult{}, fmt.Errorf("decision %q contains an option without an id", request.Name)
	}
	if selected.Value == "" {
		return DecisionResult{}, fmt.Errorf("decision %q selected an empty value", request.Name)
	}

	return DecisionResult{
		OptionID:   selected.ID,
		Value:      selected.Value,
		Confidence: selected.Confidence,
		Reason:     "Selected the strongest evidence-backed candidate deterministically.",
	}, nil
}
