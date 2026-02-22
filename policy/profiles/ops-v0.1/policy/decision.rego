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
  hits := concat_labels(denies, warnings)
  reasons := [entry.message | entry := denies[_]]
  reason := reason_value(allow, reasons, base_reason)
  hints := dedup([hint | label := hits[_]; hint := rule_hint(label)])
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

rule_hint(label) := hint if {
  policy_data := object.get(input, "policy_data", {})
  hints := object.get(policy_data, "rule_hints", {})
  entries := object.get(hints, label, [])
  hint := entries[_]
  hint != ""
}

dedup(arr) := out if {
  uniq := {v | some v in arr; v != ""}
  out := [v | v := uniq[_]]
}

reason_value(allow, reasons, base_reason) := reasons[0] if {
  not allow
  count(reasons) > 0
}

reason_value(_, _, base_reason) := base_reason

allow_condition(base_allow, denies) := true if {
  base_allow
  count(denies) == 0
}

allow_condition(base_allow, denies) := false if {
  base_allow
  count(denies) > 0
}

allow_condition(base_allow, _) := false if {
  not base_allow
}

allow_value(base_allow, denies) := true if {
  allow_condition(base_allow, denies)
}

allow_value(base_allow, denies) := false if {
  not allow_condition(base_allow, denies)
}

aggregate_risk(base_risk, allow, _) := base_risk if {
  not allow
}

aggregate_risk(base_risk, allow, warnings) := "high" if {
  allow
  has_autonomous_warning(warnings)
}

aggregate_risk(base_risk, allow, warnings) := "high" if {
  allow
  not has_autonomous_warning(warnings)
  rules.has_breakglass
}

aggregate_risk(base_risk, allow, warnings) := base_risk if {
  allow
  not rules.has_breakglass
  not has_autonomous_warning(warnings)
}

has_autonomous_warning(warnings) := true if {
  some warning in warnings
  warning.label == "autonomous-execution"
}

has_autonomous_warning(_) := false
