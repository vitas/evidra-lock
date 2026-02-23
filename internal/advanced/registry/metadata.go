package registry

import (
	"sort"
	"strings"
)

var destructiveIndicators = []string{"delete", "destroy", "remove", "rm", "rollback"}
var longRunningIndicators = []string{"apply", "plan", "upgrade", "sync"}

func MetadataFromOperations(ops map[string]CLIOperationSpec) ToolMetadata {
	longRunning := false
	destructive := false
	labelSet := map[string]struct{}{}

	for name := range ops {
		normalized := strings.ToLower(name)
		if isLongRunningOperation(normalized) {
			longRunning = true
		}
		if isDestructiveOperation(normalized) {
			destructive = true
		}
		for _, label := range labelsForOperation(normalized) {
			labelSet[label] = struct{}{}
		}
	}

	labels := make([]string, 0, len(labelSet))
	for label := range labelSet {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	return ToolMetadata{
		LongRunning: longRunning,
		Destructive: destructive,
		Labels:      labels,
	}
}

func isDestructiveOperation(name string) bool {
	for _, indicator := range destructiveIndicators {
		if strings.Contains(name, indicator) {
			return true
		}
	}
	return false
}

func isLongRunningOperation(name string) bool {
	for _, indicator := range longRunningIndicators {
		if strings.Contains(name, indicator) {
			return true
		}
	}
	return false
}

func labelsForOperation(name string) []string {
	if isDestructiveOperation(name) {
		return []string{"destructive"}
	}
	if isLongRunningOperation(name) {
		return []string{"long-running"}
	}
	return nil
}
