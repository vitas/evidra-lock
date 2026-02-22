package evidra.policy

warn["WARN-AUTO-01"] = msg if {
  input.actor.type == "agent"
  input.source == "mcp"
  msg := "Autonomous execution: agent via mcp"
}
