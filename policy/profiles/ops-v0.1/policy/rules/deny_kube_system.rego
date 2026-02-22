package evidra.policy.rules

deny["kube-system-breakglass"] = "kube-system changes require breakglass" {
  some action in actions
  action_namespace(action) == "kube-system"
  not has_tag(action, "breakglass")
}
