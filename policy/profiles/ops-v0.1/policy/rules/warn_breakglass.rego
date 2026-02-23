package evidra.policy

warn["WARN-BREAKGLASS-01"] = "breakglass tag present" if {
  action := input.actions[_]
  action.risk_tags[_] == "breakglass"
}
