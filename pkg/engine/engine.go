package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"samebits.com/evidra-mcp/pkg/core"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/outputlimit"
	"samebits.com/evidra-mcp/pkg/policy"
)

type Mode string

const (
	ModeEnforce Mode = "enforce"
	ModeObserve Mode = "observe"
)

type ToolMetadata struct {
	LongRunning bool
	Destructive bool
	Labels      []string
}

type ToolDefinition interface {
	Name() string
	Operation() string
	ValidateParams(map[string]string) error
	BuildCommand(map[string]string) ([]string, error)
	Metadata() ToolMetadata
}

type ToolResolver interface {
	Resolve(tool string, op string) (ToolDefinition, error)
}

type ValidationHit struct {
	Severity string
	Message  string
	Labels   []string
}

type Validator interface {
	Name() string
	Validate(ctx context.Context, inv invocation.ToolInvocation, tool ToolDefinition) ([]ValidationHit, error)
}

type Config struct {
	Mode           Mode
	Guarded        bool
	PolicyRef      string
	MaxOutputBytes int
	Validators     []Validator
}

type Error struct {
	Code      string
	Message   string
	RiskLevel string
	Reason    string
	Hint      string
}

type ExecutionOutput struct {
	Status          string
	ExitCode        *int
	Stdout          string
	Stderr          string
	StdoutTruncated bool
	StderrTruncated bool
}

type Result struct {
	OK         bool
	Decision   policy.Decision
	EvidenceID string
	Output     ExecutionOutput
	Error      *Error
	Hints      []string
	Advisory   bool
	Hits       []ValidationHit
}

type Reporter interface {
	Report(progress float64, message string)
}

type ReporterFunc func(progress float64, message string)

func (f ReporterFunc) Report(progress float64, message string) {
	if f == nil {
		return
	}
	f(progress, message)
}

type ExecutionEngine struct {
	resolver ToolResolver
	policy   core.PolicyEngine
	evidence core.EvidenceStore

	mode           Mode
	guarded        bool
	policyRef      string
	maxOutputBytes int
	validators     []Validator
}

func NewExecutionEngine(resolver ToolResolver, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore, cfg Config) *ExecutionEngine {
	if cfg.Mode == "" {
		cfg.Mode = ModeEnforce
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = outputlimit.DefaultMaxBytes
	}
	return &ExecutionEngine{
		resolver:       resolver,
		policy:         policyEngine,
		evidence:       evidenceStore,
		mode:           cfg.Mode,
		guarded:        cfg.Guarded,
		policyRef:      cfg.PolicyRef,
		maxOutputBytes: cfg.MaxOutputBytes,
		validators:     cfg.Validators,
	}
}

type executeStep func(*execContext) error

type execContext struct {
	ctx      context.Context
	engine   *ExecutionEngine
	inv      invocation.ToolInvocation
	reporter Reporter

	tool          ToolDefinition
	decision      policy.Decision
	output        ExecutionOutput
	errOut        *Error
	advisory      bool
	deny          bool
	validatorHits []ValidationHit
	final         Result
}

func (e *ExecutionEngine) Execute(ctx context.Context, inv invocation.ToolInvocation, reporter Reporter) (Result, error) {
	ec := &execContext{
		ctx:      ctx,
		engine:   e,
		inv:      inv,
		reporter: reporter,
		output:   ExecutionOutput{Status: "denied"},
		decision: decisionForDeny("invalid_invocation"),
	}
	steps := []executeStep{
		stepValidateInvocation,
		stepDetectBypassAttempt,
		stepResolveAndValidateTool,
		stepEvaluatePolicy,
		stepRunValidators,
		stepExecuteTool,
		stepWriteEvidence,
		stepFinalize,
	}
	for _, step := range steps {
		if err := step(ec); err != nil {
			ec.final = e.writeFinal(inv, decisionForPolicyError(), ExecutionOutput{
				Status: "failed",
				Stderr: err.Error(),
			}, false, &Error{
				Code:    "internal_error",
				Message: "execution pipeline failed",
			}, false, nil)
			return ec.final, nil
		}
	}
	if ec.final.EvidenceID == "" {
		ec.final = e.writeFinal(inv, decisionForPolicyError(), ExecutionOutput{
			Status: "failed",
			Stderr: "empty pipeline output",
		}, false, &Error{
			Code:    "internal_error",
			Message: "execution pipeline failed",
		}, false, nil)
	}
	return ec.final, nil
}

func stepValidateInvocation(ec *execContext) error {
	report(ec.reporter, 0, "received")
	if err := ec.inv.ValidateStructure(); err != nil {
		ec.setDeny("invalid_invocation", err.Error(), "Provide actor/tool/operation and non-nil params/context.")
		return nil
	}
	report(ec.reporter, 10, "validated invocation")
	return nil
}

func stepDetectBypassAttempt(ec *execContext) error {
	if !ec.engine.guarded || ec.deny {
		return nil
	}
	if !looksLikeBypass(ec.inv) {
		return nil
	}
	ec.setDeny("bypass_attempt", "guarded mode blocked bypass attempt", "Use a registered tool and operation through the registry.")
	return nil
}

func stepResolveAndValidateTool(ec *execContext) error {
	if ec.deny {
		return nil
	}
	tool, err := ec.engine.resolver.Resolve(ec.inv.Tool, ec.inv.Operation)
	if err != nil {
		code := codedErrorCode(err)
		msg := err.Error()
		switch code {
		case "tool_not_registered", "unregistered_tool":
			if code == "unregistered_tool" && ec.engine.guarded {
				code = "tool_not_registered"
			}
			ec.setDeny(code, msg, "Install/enable the corresponding tool pack.")
			return nil
		case "unsupported_operation":
			ec.setDeny(code, msg, "Check supported operations for the registered tool.")
			return nil
		default:
			return err
		}
	}
	ec.tool = tool

	if rawValidator, ok := tool.(interface {
		ValidateRawParams(map[string]interface{}) error
	}); ok {
		if err := rawValidator.ValidateRawParams(ec.inv.Params); err != nil {
			ec.setDeny("invalid_params", err.Error(), "Fix params to match the operation schema.")
			return nil
		}
	} else {
		stringParams, err := toStringParams(ec.inv.Params)
		if err != nil {
			ec.setDeny("invalid_params", err.Error(), "Fix params to match the operation schema.")
			return nil
		}
		if err := ec.tool.ValidateParams(stringParams); err != nil {
			ec.setDeny("invalid_params", err.Error(), "Fix params to match the operation schema.")
			return nil
		}
	}
	report(ec.reporter, 25, "registry ok")
	return nil
}

func stepEvaluatePolicy(ec *execContext) error {
	if ec.deny {
		return nil
	}
	decision, evalErr := ec.engine.policy.Evaluate(ec.inv)
	if evalErr != nil {
		decision = decisionForPolicyError()
		decision.Hints = []string{"Policy evaluation failed; verify policy syntax and rerun evidra validate with the same input file."}
		decision.Hint = decision.Hints[0]
	}
	ec.decision = decision
	report(ec.reporter, 40, "policy evaluated (allow/deny)")
	if ec.engine.mode == ModeEnforce && !decision.Allow {
		ec.output = ExecutionOutput{Status: "denied"}
		ec.errOut = &Error{
			Code:      decision.Reason,
			Message:   "execution denied by policy",
			RiskLevel: decision.RiskLevel,
			Reason:    decision.Reason,
			Hint:      firstHint(decision.Hints, decision.Hint),
		}
		ec.deny = true
		return nil
	}
	ec.advisory = ec.engine.mode == ModeObserve
	return nil
}

func stepRunValidators(ec *execContext) error {
	if ec.deny || len(ec.engine.validators) == 0 {
		return nil
	}
	for _, v := range ec.engine.validators {
		hits, err := v.Validate(ec.ctx, ec.inv, ec.tool)
		if err != nil {
			return fmt.Errorf("validator %s failed: %w", v.Name(), err)
		}
		ec.validatorHits = append(ec.validatorHits, hits...)
	}
	ec.decision.RiskLevel = aggregateRisk(ec.decision, ec.validatorHits)
	return nil
}

func stepExecuteTool(ec *execContext) error {
	if ec.deny {
		return nil
	}
	report(ec.reporter, 60, "execution started")
	stopHeartbeat := startProgressHeartbeat(ec.ctx, ec.reporter, ec.decision.LongRunning || ec.tool.Metadata().LongRunning)
	execOut, execErr := executeTool(ec.ctx, ec.tool, ec.inv.Params)
	stopHeartbeat()
	if errors.Is(ec.ctx.Err(), context.Canceled) || errors.Is(execErr, context.Canceled) {
		execOut.Status = "cancelled"
		if execOut.Stderr == "" {
			execOut.Stderr = "execution cancelled"
		}
	}
	if execErr != nil {
		execOut.Status = "failed"
		if execOut.Stderr == "" {
			execOut.Stderr = execErr.Error()
		}
	}
	ec.output = execOut
	report(ec.reporter, 90, "execution finished (writing evidence)")
	return nil
}

func stepWriteEvidence(ec *execContext) error {
	ec.final = ec.engine.writeFinal(ec.inv, ec.decision, ec.output, !ec.deny, ec.errOut, ec.advisory, ec.validatorHits)
	return nil
}

func stepFinalize(ec *execContext) error {
	if ec.advisory && !ec.decision.Allow {
		ec.final.Hints = append(ec.final.Hints, "observe mode: policy denied but execution was allowed")
	}
	if ec.output.Status == "cancelled" {
		report(ec.reporter, 100, "cancelled")
		return nil
	}
	if ec.deny {
		report(ec.reporter, 100, "denied (evidence written)")
		return nil
	}
	report(ec.reporter, 100, "done")
	return nil
}

func (e *ExecutionEngine) writeFinal(inv invocation.ToolInvocation, decision policy.Decision, out ExecutionOutput, ok bool, errOut *Error, advisory bool, hits []ValidationHit) Result {
	out = e.applyOutputLimit(out)
	record := evidence.EvidenceRecord{
		EventID:   fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		PolicyRef: e.policyRef,
		Actor:     inv.Actor,
		Tool:      inv.Tool,
		Operation: inv.Operation,
		Params:    inv.Params,
		PolicyDecision: evidence.PolicyDecision{
			Allow:     decision.Allow,
			RiskLevel: decision.RiskLevel,
			Reason:    decision.Reason,
			Advisory:  advisory,
		},
		ExecutionResult: evidence.ExecutionResult{
			Status:          out.Status,
			ExitCode:        out.ExitCode,
			Stdout:          out.Stdout,
			Stderr:          out.Stderr,
			StdoutTruncated: out.StdoutTruncated,
			StderrTruncated: out.StderrTruncated,
		},
	}
	if errOut != nil && errOut.Code == "bypass_attempt" {
		if record.Params == nil {
			record.Params = map[string]interface{}{}
		}
		record.Params["violation_type"] = "bypass_attempt"
	}

	appendErr := e.evidence.Append(record)
	if appendErr != nil {
		mapped := mapEvidenceAppendError(appendErr)
		return Result{
			OK:         false,
			Decision:   decision,
			EvidenceID: record.EventID,
			Output: ExecutionOutput{
				Status:          "failed",
				Stdout:          out.Stdout,
				Stderr:          appendErr.Error(),
				StdoutTruncated: out.StdoutTruncated,
				StderrTruncated: out.StderrTruncated,
			},
			Error: mapped,
			Hints: []string{"evidence write failed; result treated as failed"},
			Hits:  hits,
		}
	}

	return Result{
		OK:         ok,
		Decision:   decision,
		EvidenceID: record.EventID,
		Output:     out,
		Error:      errOut,
		Hints:      combineHints(decision.Hints, hintsForExecution(out.Status, decision.RiskLevel)),
		Advisory:   advisory,
		Hits:       hits,
	}
}

func (e *ExecutionEngine) applyOutputLimit(in ExecutionOutput) ExecutionOutput {
	stdout, stdoutTruncated := outputlimit.Truncate(in.Stdout, e.maxOutputBytes)
	stderr, stderrTruncated := outputlimit.Truncate(in.Stderr, e.maxOutputBytes)
	in.Stdout = stdout
	in.Stderr = stderr
	in.StdoutTruncated = in.StdoutTruncated || stdoutTruncated
	in.StderrTruncated = in.StderrTruncated || stderrTruncated
	return in
}

func executeTool(ctx context.Context, tool ToolDefinition, rawParams map[string]interface{}) (ExecutionOutput, error) {
	if ex, ok := tool.(interface {
		Execute(context.Context, map[string]interface{}) (ExecutionOutput, error)
	}); ok {
		return ex.Execute(ctx, rawParams)
	}
	stringParams, err := toStringParams(rawParams)
	if err != nil {
		return ExecutionOutput{Status: "failed"}, err
	}
	argv, err := tool.BuildCommand(stringParams)
	if err != nil {
		return ExecutionOutput{Status: "failed"}, err
	}
	if len(argv) == 0 {
		return ExecutionOutput{Status: "failed"}, fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		code := 0
		return ExecutionOutput{Status: "success", ExitCode: &code, Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		code := exitErr.ExitCode()
		return ExecutionOutput{Status: "failed", ExitCode: &code, Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}
	return ExecutionOutput{Status: "failed", Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func toStringParams(in map[string]interface{}) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch x := v.(type) {
		case string:
			out[k] = x
		case bool:
			out[k] = strconv.FormatBool(x)
		case int:
			out[k] = strconv.Itoa(x)
		case int64:
			out[k] = strconv.FormatInt(x, 10)
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		default:
			return nil, fmt.Errorf("param %s cannot be converted to string", k)
		}
	}
	return out, nil
}

func aggregateRisk(decision policy.Decision, hits []ValidationHit) string {
	max := riskRank(decision.RiskLevel)
	for _, h := range hits {
		if rank := riskRank(h.Severity); rank > max {
			max = rank
		}
	}
	return riskFromRank(max)
}

func riskRank(v string) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func riskFromRank(v int) string {
	switch {
	case v >= 4:
		return "critical"
	case v == 3:
		return "high"
	case v == 2:
		return "medium"
	default:
		return "low"
	}
}

func decisionForDeny(reason string) policy.Decision {
	return policy.Decision{Allow: false, RiskLevel: "critical", Reason: reason}
}

func decisionForPolicyError() policy.Decision {
	return policy.Decision{
		Allow:     false,
		RiskLevel: "critical",
		Reason:    "policy_evaluation_failed",
		Reasons:   []string{"policy_evaluation_failed"},
	}
}

func mapEvidenceAppendError(err error) *Error {
	if evidence.IsStoreBusyError(err) {
		return &Error{
			Code:    evidence.ErrorCodeStoreBusy,
			Message: "evidence store is busy",
			Hint:    "Wait for the active writer to finish and retry.",
		}
	}
	if errors.Is(err, evidence.ErrChainInvalid) {
		return &Error{
			Code:    "evidence_chain_invalid",
			Message: "evidence chain validation failed",
			Hint:    "Verify evidence integrity before retrying.",
		}
	}
	return &Error{
		Code:    "internal_error",
		Message: "failed to write evidence",
		Hint:    "Check evidence path permissions and disk state.",
	}
}

func report(r Reporter, progress float64, message string) {
	if r == nil {
		return
	}
	r.Report(progress, message)
}

func startProgressHeartbeat(ctx context.Context, reporter Reporter, enabled bool) func() {
	if reporter == nil || !enabled {
		return func() {}
	}
	done := make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				report(reporter, 75, "still running...")
			}
		}
	}()
	return stop
}

func hintsForExecution(status, risk string) []string {
	hints := []string{}
	switch status {
	case "denied":
		hints = append(hints, "call get_event with event_id for full evidence details")
	case "failed":
		hints = append(hints, "inspect stderr and policy.reason for triage")
	case "success":
		hints = append(hints, "call get_event with event_id for immutable audit record")
	}
	if risk == "critical" {
		hints = append(hints, "critical risk operation; require explicit review in production workflows")
	}
	return hints
}

func combineHints(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, group := range groups {
		for _, h := range group {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			if _, exists := seen[h]; exists {
				continue
			}
			seen[h] = struct{}{}
			out = append(out, h)
		}
	}
	return out
}

func firstHint(hints []string, fallback string) string {
	if len(hints) > 0 && strings.TrimSpace(hints[0]) != "" {
		return hints[0]
	}
	return strings.TrimSpace(fallback)
}

func (ec *execContext) setDeny(reason, msg, hint string) {
	ec.decision = decisionForDeny(reason)
	ec.output = ExecutionOutput{Status: "denied"}
	ec.errOut = &Error{
		Code:      reason,
		Message:   msg,
		RiskLevel: "critical",
		Reason:    reason,
		Hint:      hint,
	}
	ec.deny = true
}

func looksLikeBypass(inv invocation.ToolInvocation) bool {
	tool := strings.ToLower(strings.TrimSpace(inv.Tool))
	op := strings.ToLower(strings.TrimSpace(inv.Operation))
	if isShellToken(tool) || isShellToken(op) {
		return true
	}
	if strings.Contains(tool, "/") || strings.Contains(tool, "\\") || strings.HasPrefix(tool, ".") {
		return true
	}
	for k, raw := range inv.Params {
		key := strings.ToLower(strings.TrimSpace(k))
		v, ok := raw.(string)
		if !ok {
			continue
		}
		val := strings.ToLower(strings.TrimSpace(v))
		if val == "" {
			continue
		}
		if key == "command" || key == "cmd" || key == "script" || key == "shell" || key == "binary" || key == "executable" {
			return true
		}
		if (key == "path" || key == "binary_path") && (strings.Contains(val, "/") || strings.Contains(val, "\\") || strings.HasPrefix(val, ".")) {
			return true
		}
	}
	return false
}

func isShellToken(v string) bool {
	switch v {
	case "sh", "bash", "zsh", "shell", "cmd", "powershell", "pwsh":
		return true
	default:
		return false
	}
}

type codedError interface {
	error
	Code() string
}

func codedErrorCode(err error) string {
	var ce codedError
	if errors.As(err, &ce) {
		return ce.Code()
	}
	return ""
}
