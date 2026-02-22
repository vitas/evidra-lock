package evidra.policy.rules

warn["autonomous-execution"] = "autonomous execution: agent via mcp" if {
  actor_type == "agent"
  input_source == "mcp"
}
