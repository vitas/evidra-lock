package evidra.policy

import data.evidra.policy.defaults as defaults
import data.evidra.policy.rules as rules

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
  "reasons": [],
  "hints": [],
  "hits": [],
  "denies": [],
  "warnings": [],
}

decision := build_decision

build_decision := {
  "allow": allow,
  "risk_level": risk_level,
  "reason": reason,
  "reasons": reasons,
  "hints": hints,
  "hits": hits,
  "denies": denies,
  "warnings": warnings,
} if {
  base := defaults.decision
  base_allow := object.get(base, "allow", false)
  base_risk := object.get(base, "risk_level", "critical")
  base_reason := object.get(base, "reason", "policy_denied_default")

  denies := build_results(rules.deny)
  warnings := build_results(rules.warn)

  allow := allow_value(base_allow, denies)
  reasons := [entry.message | entry := denies[_]]
  reason := reason_value(base_reason, allow, reasons)
  hits := concat_labels(denies, warnings)
  policy_hints := [
    hint |
    label := hits[_]
    policy_data := object.get(input, "policy_data", {})
    hints_map := object.get(policy_data, "rule_hints", {})
    entries := object.get(hints_map, label, [])
    hint := entries[_]
    hint != ""
  ]
  hints := dedup(policy_hints)
  risk_level := aggregate_risk(base_risk, allow, warnings)
}

build_results(mapping) := results if {
  results := [{"label": label, "message": message} |
    some label
    message := mapping[label]
    message != ""
    label != ""
  ]
}

concat_labels(denies, warnings) := array.concat([
  entry.label | entry := denies[_]
], [
  entry.label | entry := warnings[_]
])

dedup(arr) := out if {
  uniq := {v | some v in arr; v != ""}
  out := [v | v := uniq[_]]
}

reason_value(base_reason, allow, reasons) := reasons[0] if {
  not allow
  count(reasons) > 0
}

reason_value(base_reason, allow, _) := base_reason if {
  allow
}

reason_value(base_reason, _, reasons) := base_reason if {
  count(reasons) == 0
}

allow_value(base_allow, denies) := true if {
  base_allow
  count(denies) == 0
}

allow_value(_, denies) := false if {
  count(denies) > 0
}

allow_value(base_allow, _) := false if {
  not base_allow
}

aggregate_risk(base_risk, allow, warnings) := base_risk if {
  not allow
}

aggregate_risk(_, allow, warnings) := "high" if {
  allow
  has_autonomous_warning(warnings)
}

aggregate_risk(_, allow, warnings) := "high" if {
  allow
  not has_autonomous_warning(warnings)
  rules.has_breakglass
}

aggregate_risk(base_risk, allow, warnings) := base_risk if {
  allow
  not has_autonomous_warning(warnings)
  not rules.has_breakglass
}

has_autonomous_warning(warnings) := count([warning | warning := warnings[_]; warning.label == "autonomous-execution"]) > 0
