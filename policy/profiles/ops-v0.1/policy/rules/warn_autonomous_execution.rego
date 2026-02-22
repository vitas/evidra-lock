package evidra.policy.rules

warn["autonomous-execution"] = "autonomous execution: agent via mcp" {
  actor_type == "agent"
  input_source == "mcp"
}
