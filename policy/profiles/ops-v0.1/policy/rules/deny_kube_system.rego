package evidra.policy

import data.evidra.policy.defaults as defaults

deny["POL-KUBE-01"] = msg if {
  some i
  action := input.actions[i]
  defaults.action_namespace(action) == "kube-system"
  not defaults.has_tag(action, "breakglass")
  msg := "Changes in kube-system require breakglass"
}
