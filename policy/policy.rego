package evidra.policy

default allow := false
default reason := "denied: default deny"

# Allow example: echo tool execution.
allow if {
	input.tool.name == "echo"
	input.operation == "execute"
}

# Allow example: git status only.
allow if {
	input.tool.name == "git"
	input.operation == "execute"
	input.params.args == ["status"]
}

reason := "allowed: echo execute" if {
	input.tool.name == "echo"
	input.operation == "execute"
}

reason := "allowed: git status execute" if {
	input.tool.name == "git"
	input.operation == "execute"
	input.params.args == ["status"]
}
