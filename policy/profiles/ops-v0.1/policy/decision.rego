package evidra.policy

import data.evidra.policy.rules as rules

default decision := {
  "allow": false,
  "risk_level": "critical",
  "reason": "policy_denied_default",
  "reasons": [],
  "hints": [],
  "hits": [],
}

decision := make_decision

make_decision := {
  "allow": allow,
  "risk_level": risk_level,
  "reason": reason,
  "reasons": reasons,
  "hints": hints,
  "hits": hits,
  "denies": denies,
  "warnings": warnings,
} if {
  denies := build_denies
  warnings := build_warnings
  allow := count(denies) == 0
  hits := build_hits(denies, warnings)
  reasons := [entry.message | entry := denies[_]]
  reason := decision_reason(allow, reasons)
  hints := dedup([hint | label := hits[_]; hint := rule_hint(label)])
  risk_level := aggregate_risk(allow, warnings)
}

build_denies := [entry {
  some label
  msg := rules.deny[label]
  entry := {"label": label, "message": msg}
}]

build_warnings := [entry {
  some label
  msg := rules.warn[label]
  entry := {"label": label, "message": msg}
}]

build_hits(denies, warnings) := array.concat([
  entry.label | entry := denies[_]
], [
  entry.label | entry := warnings[_]
])

rule_hint(label) := hint {
  hints := object.get(data, "rule_hints", {})
  entries := object.get(hints, label, [])
  hint := entries[_]
  hint != ""
}

dedup(arr) := out {
  uniq := {v | some v in arr; v != ""}
  out := [v | v := uniq[_]]
}

decision_reason(allow, reasons) := reasons[0] if {
  not allow
  count(reasons) > 0
}

decision_reason(allow, _) := default_allow_reason if {
  allow
}

default_allow_reason := object.get(object.get(data, "allow_reasons", {}), "default", "allowed_operation")

aggregate_risk(allow, warnings) := "high" if {
  not allow
}

aggregate_risk(allow, warnings) := "high" if {
  allow
  some warning in warnings
  warning.label == "autonomous-execution"
}

aggregate_risk(allow, _) := "high" if {
  allow
  has_breakglass
}

aggregate_risk(allow, warnings) := "normal" if {
  allow
  count(warnings) == 0
  not has_breakglass
}

has_breakglass {
  some action in rules.actions
  rules.has_tag(action, "breakglass")
}
