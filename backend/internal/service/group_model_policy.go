package service

import "strings"

const (
	GroupModelPolicyModeNone          = ""
	GroupModelPolicyModeForce         = "force"
	GroupModelPolicyDefaultForceModel = "gpt-5.3-codex-spark"
)

func NormalizeGroupModelPolicyMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case GroupModelPolicyModeForce:
		return GroupModelPolicyModeForce
	default:
		return GroupModelPolicyModeNone
	}
}

func NormalizeGroupModelPolicyModel(mode string, model string) string {
	if NormalizeGroupModelPolicyMode(mode) != GroupModelPolicyModeForce {
		return ""
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return GroupModelPolicyDefaultForceModel
	}
	return model
}

func ResolveGroupModelPolicyModel(group *Group, requestedModel string) (string, bool) {
	if group == nil {
		return requestedModel, false
	}
	mode := NormalizeGroupModelPolicyMode(group.ModelPolicyMode)
	model := NormalizeGroupModelPolicyModel(mode, group.ModelPolicyModel)
	if mode != GroupModelPolicyModeForce || model == "" {
		return requestedModel, false
	}
	if model == requestedModel {
		return requestedModel, false
	}
	return model, true
}
