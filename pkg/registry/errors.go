package registry

import "fmt"

type ErrToolNotFound struct {
	Tool string
}

func (e ErrToolNotFound) Error() string {
	return fmt.Sprintf("tool %q is not registered", e.Tool)
}

func (ErrToolNotFound) Code() string {
	return "unregistered_tool"
}

type ErrOperationNotFound struct {
	Tool      string
	Operation string
}

func (e ErrOperationNotFound) Error() string {
	return fmt.Sprintf("operation %q is not supported for tool %q", e.Operation, e.Tool)
}

func (ErrOperationNotFound) Code() string {
	return "unsupported_operation"
}

type ErrInvalidParams struct {
	Tool      string
	Operation string
	Param     string
	Reason    string
}

func (e ErrInvalidParams) Error() string {
	if e.Param == "" {
		return e.Reason
	}
	return fmt.Sprintf("param %s: %s", e.Param, e.Reason)
}

func (ErrInvalidParams) Code() string {
	return "invalid_params"
}

func newInvalidParam(tool, operation, param, reason string) ErrInvalidParams {
	return ErrInvalidParams{
		Tool:      tool,
		Operation: operation,
		Param:     param,
		Reason:    reason,
	}
}
