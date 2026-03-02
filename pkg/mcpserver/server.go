package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra/pkg/client"
	"samebits.com/evidra/pkg/config"
	"samebits.com/evidra/pkg/evidence"
	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/validate"
)

type Mode string

const (
	ModeEnforce Mode = "enforce"
)

// Error codes used in ErrorSummary.Code.
const (
	ErrCodeInvalidInput   = "invalid_input"
	ErrCodePolicyFailure  = "policy_failure"
	ErrCodeEvidenceWrite  = "evidence_write_failed"
	ErrCodeChainInvalid   = "evidence_chain_invalid"
	ErrCodeNotFound       = "not_found"
	ErrCodeInternalError  = "internal_error"
	ErrCodeAPIUnreachable = "api_unreachable"
)

type Options struct {
	Name                     string
	Version                  string
	Mode                     Mode
	PolicyRef                string
	PolicyPath               string
	DataPath                 string
	BundlePath               string
	Environment              string
	EvidencePath             string
	IncludeFileResourceLinks bool
	APIClient                *client.Client
	FallbackPolicy           string
	IsOnline                 bool
}

type PolicySummary struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
	PolicyRef string `json:"policy_ref,omitempty"`
}

type ErrorSummary struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RiskLevel string `json:"risk_level,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type ValidateOutput struct {
	OK        bool                    `json:"ok"`
	EventID   string                  `json:"event_id,omitempty"`
	Source    string                  `json:"source"`
	Policy    PolicySummary           `json:"policy"`
	RuleIDs   []string                `json:"rule_ids,omitempty"`
	Hints     []string                `json:"hints,omitempty"`
	Reasons   []string                `json:"reasons,omitempty"`
	Resources []evidence.ResourceLink `json:"resources,omitempty"`
	Error     *ErrorSummary           `json:"error,omitempty"`
}

type validateHandler struct {
	service *ValidateService
}

type getEventHandler struct {
	service *ValidateService
}

type getEventInput struct {
	EventID string `json:"event_id"`
}

const initializeInstructions = "Evidra policy enforcement is active.\n" +
	"Always call `validate` before destructive or privileged operations.\n" +
	"On deny: STOP and do not retry unchanged input.\n" +
	"If hints indicate missing data, ask for required fields and re-run validate.\n" +
	"Fetch canonical contract from `evidra://prompts/agent_contract_v1`."

const (
	resourceURIDocsEngineLogicV2 = "evidra://docs/engine_logic_v2"
	resourceURIDocsProtocolError = "evidra://docs/protocol_errors"
	resourceURIPolicySummary     = "evidra://policy/summary"
	resourceURIAgentContractV1   = "evidra://prompts/agent_contract_v1"
)

const engineLogicV2ResourceMarkdown = "# Evidra Engine Logic v2 (MCP summary)\n\n" +
	"- `actor.type` is the security classifier (`human|agent|ci`).\n" +
	"- `actor.origin` is transport metadata (`mcp|cli|api`) and not a security classifier.\n" +
	"- `context.source` is optional metadata and not a security classifier.\n" +
	"- Canonicalization normalizes native and flat Kubernetes payloads before policy rules run.\n" +
	"- If MCP schema validation fails, JSON-RPC returns `-32602` and tool handlers are not invoked.\n\n" +
	"See repo docs: `docs/ENGINE_LOGIC_V2.md`."

const protocolErrorsResourceMarkdown = "# Protocol Errors\n\n" +
	"## `-32602` Invalid params\n\n" +
	"- Means request arguments failed MCP schema validation.\n" +
	"- In this path, tool handlers are not invoked.\n" +
	"- Tool-level fields like `ok`, `policy`, or `error` are not produced.\n\n" +
	"See repo docs: `docs/PROTOCOL_ERRORS.md`."

const agentContractPromptV1Markdown = "# Evidra Agent Contract v1\n\n" +
	"You are operating against an Evidra policy enforcement service.\n\n" +
	"## 1. Always Validate First\n\n" +
	"Before any destructive or privileged operation:\n\n" +
	"- Call the `validate` tool.\n" +
	"- Never execute mutations without prior validation.\n\n" +
	"## 2. On Policy Deny\n\n" +
	"If `validate` indicates deny (`ok=false` or `policy.allow=false`), you MUST:\n\n" +
	"- STOP immediately.\n" +
	"- Do NOT retry unchanged input.\n" +
	"- Do NOT attempt alternate mutations.\n" +
	"- Explain the denial.\n" +
	"- Escalate if required.\n\n" +
	"## 3. On `insufficient_context`\n\n" +
	"If hints indicate missing fields:\n\n" +
	"- Ask the user for missing data.\n" +
	"- Re-run `validate`.\n" +
	"- Do NOT guess Kubernetes fields.\n\n" +
	"## 4. Kubernetes Payload Format\n\n" +
	"You may send:\n\n" +
	"- Native Kubernetes manifest\n" +
	"- Flat schema\n\n" +
	"Server canonicalizes internally.\n\n" +
	"## 5. Actor Semantics\n\n" +
	"- `actor.type = human | agent | ci`\n" +
	"- `actor.origin = mcp | cli | api`\n\n" +
	"`actor.type` is the security classifier.\n" +
	"`context.source` is metadata only.\n\n" +
	"## 6. Protocol Errors\n\n" +
	"If you receive JSON-RPC error `-32602`:\n\n" +
	"- Schema validation failed.\n" +
	"- Tool handler was NOT executed.\n" +
	"- Fix request structure before retrying.\n\n" +
	"## 7. Large Manifests (Important)\n\n" +
	"When validating large manifests:\n\n" +
	"- Send the full manifest in ONE `validate` call.\n" +
	"- Do NOT progressively enrich partial payloads.\n" +
	"- Do NOT split across multiple attempts."

func NewServer(opts Options) *mcp.Server {
	if opts.Name == "" {
		opts.Name = "evidra-mcp"
	}
	if opts.Version == "" {
		opts.Version = "v0.1.0"
	}
	opts.Mode = ModeEnforce
	if opts.EvidencePath == "" {
		resolved, err := config.ResolveEvidencePath("")
		if err == nil {
			opts.EvidencePath = resolved
		}
	}

	svc := newValidateService(opts)
	validateTool := &validateHandler{service: svc}
	getEventTool := &getEventHandler{service: svc}

	server := mcp.NewServer(
		&mcp.Implementation{Name: opts.Name, Version: opts.Version},
		&mcp.ServerOptions{
			Instructions: initializeInstructions,
		},
	)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate",
		Title:       "Validate Tool Invocation",
		Description: "Evaluates intended infrastructure action(s) against the Evidra policy bundle.\nCall before destructive or privileged operations (for example: kubectl apply/delete, terraform apply/destroy, helm upgrade/uninstall, argocd sync).\nIf allow=false: STOP. Show reasons to the user. Do not retry unchanged input.\nIf hints indicate missing data, request required fields and re-run validate.\nKubernetes payload may be a native manifest or a flat schema (server canonicalizes).\nRead-only operations (plan/get/describe/list/show/diff) can usually skip validate.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Validate Scenario",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: mustLoadInputSchema(validateSchemaBytes, "schemas/validate.schema.json"),
	}, validateTool.Handle)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event",
		Title:       "Get Evidence Event",
		Description: "Fetch one immutable evidence record by event_id.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Evidence Lookup",
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: mustLoadInputSchema(getEventSchemaBytes, "schemas/get_event.schema.json"),
	}, getEventTool.Handle)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "evidra-event",
		Title:       "Evidence Event Record",
		Description: "Read a specific evidence record by event_id.",
		MIMEType:    "application/json",
		URITemplate: "evidra://event/{event_id}",
	}, svc.readResourceEvent)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-evidence-manifest",
		Title:       "Evidence Manifest",
		Description: "Read evidence manifest for segmented store.",
		MIMEType:    "application/json",
		URI:         "evidra://evidence/manifest",
	}, svc.readResourceManifest)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-evidence-segments",
		Title:       "Evidence Segments",
		Description: "Read sealed/current segment summary.",
		MIMEType:    "application/json",
		URI:         "evidra://evidence/segments",
	}, svc.readResourceSegments)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-docs-engine-logic-v2",
		Title:       "Engine Logic v2",
		Description: "MCP-facing summary of engine contract and classification semantics.",
		MIMEType:    "text/markdown",
		URI:         resourceURIDocsEngineLogicV2,
	}, svc.readResourceEngineLogicV2)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-docs-protocol-errors",
		Title:       "Protocol Errors",
		Description: "MCP/JSON-RPC error semantics used by Evidra tests and clients.",
		MIMEType:    "text/markdown",
		URI:         resourceURIDocsProtocolError,
	}, svc.readResourceProtocolErrors)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-policy-summary",
		Title:       "Policy Summary",
		Description: "Active policy and guidance-surface summary for MCP clients.",
		MIMEType:    "application/json",
		URI:         resourceURIPolicySummary,
	}, svc.readResourcePolicySummary)
	server.AddResource(&mcp.Resource{
		Name:        "evidra-agent-contract-v1",
		Title:       "Evidra Agent Contract v1",
		Description: "Canonical vendor-agnostic agent guidance for Evidra-hosted MCP clients.",
		MIMEType:    "text/markdown",
		URI:         resourceURIAgentContractV1,
	}, svc.readResourceAgentContractV1)
	return server
}

func (h *validateHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input invocation.ToolInvocation,
) (*mcp.CallToolResult, ValidateOutput, error) {
	output := h.service.Validate(ctx, input)
	return &mcp.CallToolResult{Content: resourceLinksToContent(output.Resources)}, output, nil
}

func (h *getEventHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input getEventInput,
) (*mcp.CallToolResult, GetEventOutput, error) {
	output := h.service.GetEvent(ctx, input.EventID)
	return &mcp.CallToolResult{Content: resourceLinksToContent(output.Resources)}, output, nil
}

func newValidateService(opts Options) *ValidateService {
	return &ValidateService{
		policyPath:               opts.PolicyPath,
		dataPath:                 opts.DataPath,
		bundlePath:               opts.BundlePath,
		environment:              opts.Environment,
		evidencePath:             opts.EvidencePath,
		policyRef:                opts.PolicyRef,
		mode:                     opts.Mode,
		includeFileResourceLinks: opts.IncludeFileResourceLinks,
		apiClient:                opts.APIClient,
		fallbackPolicy:           opts.FallbackPolicy,
		isOnline:                 opts.IsOnline,
	}
}

type ValidateService struct {
	policyPath               string
	dataPath                 string
	bundlePath               string
	environment              string
	evidencePath             string
	policyRef                string
	mode                     Mode
	includeFileResourceLinks bool
	apiClient                *client.Client
	fallbackPolicy           string
	isOnline                 bool
}

type GetEventOutput struct {
	OK        bool                    `json:"ok"`
	Record    *evidence.Record        `json:"record,omitempty"`
	Resources []evidence.ResourceLink `json:"resources,omitempty"`
	Error     *ErrorSummary           `json:"error,omitempty"`
}

func (s *ValidateService) Validate(ctx context.Context, inv invocation.ToolInvocation) ValidateOutput {
	// ONLINE: try API first
	if s.isOnline && s.apiClient != nil {
		result, _, err := s.apiClient.Validate(ctx, inv)
		if err == nil {
			return ValidateOutput{
				OK:      result.Pass,
				EventID: result.EvidenceID,
				Source:  "api",
				Policy: PolicySummary{
					Allow:     result.Pass,
					RiskLevel: result.RiskLevel,
					Reason:    firstReason(result.Reasons),
					PolicyRef: result.PolicyRef,
				},
				RuleIDs: result.RuleIDs,
				Hints:   result.Hints,
				Reasons: result.Reasons,
			}
		}

		// Reachability error → check fallback policy
		if client.IsReachabilityError(err) && s.fallbackPolicy == "offline" {
			// Fall through to local evaluation below
		} else {
			// Non-recoverable: auth/validation/rate-limit, or fallback=closed
			code := ErrCodeAPIUnreachable
			if errors.Is(err, client.ErrUnauthorized) {
				code = "unauthorized"
			} else if errors.Is(err, client.ErrForbidden) {
				code = "forbidden"
			} else if errors.Is(err, client.ErrRateLimited) {
				code = "rate_limited"
			} else if errors.Is(err, client.ErrInvalidInput) {
				code = ErrCodeInvalidInput
			}
			return ValidateOutput{
				OK:     false,
				Source: "none",
				Policy: PolicySummary{
					Allow:     false,
					RiskLevel: "high",
					Reason:    code,
				},
				Error: &ErrorSummary{Code: code, Message: err.Error()},
			}
		}
	}

	// OFFLINE or FALLBACK: local evaluation
	res, err := validate.EvaluateInvocation(ctx, inv, validate.Options{
		PolicyPath:  s.policyPath,
		DataPath:    s.dataPath,
		BundlePath:  s.bundlePath,
		Environment: s.environment,
		EvidenceDir: s.evidencePath,
	})
	if err != nil {
		code, msg := validateErrCode(err)
		return ValidateOutput{
			OK:     false,
			Source: "none",
			Policy: PolicySummary{
				Allow:     false,
				RiskLevel: "high",
				Reason:    code,
				PolicyRef: s.policyRef,
			},
			Error: &ErrorSummary{Code: code, Message: msg},
		}
	}

	source := "local"
	if s.isOnline {
		source = "local-fallback"
	}

	return ValidateOutput{
		OK:      res.Pass,
		EventID: res.EvidenceID,
		Source:  source,
		Policy: PolicySummary{
			Allow:     res.Pass,
			RiskLevel: res.RiskLevel,
			Reason:    firstReason(res.Reasons),
			PolicyRef: s.policyRef,
		},
		RuleIDs:   res.RuleIDs,
		Hints:     res.Hints,
		Reasons:   res.Reasons,
		Resources: s.resourceLinks(res.EvidenceID),
	}
}

func firstReason(reasons []string) string {
	if len(reasons) == 0 {
		return "scenario_validated"
	}
	return reasons[0]
}

func (s *ValidateService) GetEvent(_ context.Context, eventID string) GetEventOutput {
	if eventID == "" {
		return GetEventOutput{OK: false, Error: &ErrorSummary{Code: ErrCodeInvalidInput, Message: "event_id is required"}}
	}
	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil {
		if errors.Is(err, evidence.ErrChainInvalid) {
			return GetEventOutput{OK: false, Error: &ErrorSummary{Code: ErrCodeChainInvalid, Message: "evidence chain validation failed"}}
		}
		return GetEventOutput{OK: false, Error: &ErrorSummary{Code: ErrCodeInternalError, Message: "failed to read evidence"}}
	}
	if !found {
		return GetEventOutput{OK: false, Error: &ErrorSummary{Code: ErrCodeNotFound, Message: "event_id not found"}}
	}
	return GetEventOutput{OK: true, Record: &rec, Resources: s.resourceLinks(rec.EventID)}
}

func (s *ValidateService) resourceLinks(eventID string) []evidence.ResourceLink {
	return evidence.ResourceLinks(eventID, s.evidencePath, s.includeFileResourceLinks)
}

func resourceLinksToContent(links []evidence.ResourceLink) []mcp.Content {
	if len(links) == 0 {
		return nil
	}
	out := make([]mcp.Content, 0, len(links))
	for _, l := range links {
		out = append(out, &mcp.ResourceLink{URI: l.URI, Name: l.Name, MIMEType: l.MIMEType})
	}
	return out
}

func (s *ValidateService) readResourceEvent(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	eventID := strings.TrimPrefix(req.Params.URI, "evidra://event/")
	if eventID == "" || eventID == req.Params.URI {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	rec, found, err := evidence.FindByEventID(s.evidencePath, eventID)
	if err != nil || !found {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/json", Text: string(b)}}}, nil
}

func (s *ValidateService) readResourceManifest(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	m, err := evidence.LoadManifest(s.evidencePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "evidra://evidence/manifest", MIMEType: "application/json", Text: string(b)}}}, nil
}

func (s *ValidateService) readResourceSegments(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	m, err := evidence.LoadManifest(s.evidencePath)
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	payload := map[string]any{"sealed_segments": m.SealedSegments, "current_segment": m.CurrentSegment, "count": len(m.SealedSegments)}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: "evidra://evidence/segments", MIMEType: "application/json", Text: string(b)}}}, nil
}

func (s *ValidateService) readResourceEngineLogicV2(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return staticResourceText(req.Params.URI, resourceURIDocsEngineLogicV2, "text/markdown", engineLogicV2ResourceMarkdown)
}

func (s *ValidateService) readResourceProtocolErrors(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return staticResourceText(req.Params.URI, resourceURIDocsProtocolError, "text/markdown", protocolErrorsResourceMarkdown)
}

func (s *ValidateService) readResourcePolicySummary(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if req.Params.URI != resourceURIPolicySummary {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	summary := map[string]any{
		"policy_ref":  s.effectivePolicyRef(),
		"mode":        string(s.mode),
		"bundle_path": s.bundlePath,
		"environment": s.environment,
		"guidance": map[string]any{
			"initialize_instructions": true,
			"initialize_points_to":    resourceURIAgentContractV1,
			"validate_tool_description": map[string]any{
				"present": true,
				"name":    "validate",
			},
			"agent_contract": map[string]any{
				"latest_stable": "v1",
				"uri":           resourceURIAgentContractV1,
			},
			"resources": []string{
				resourceURIDocsEngineLogicV2,
				resourceURIDocsProtocolError,
				resourceURIPolicySummary,
				resourceURIAgentContractV1,
			},
		},
	}

	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: resourceURIPolicySummary, MIMEType: "application/json", Text: string(b)}}}, nil
}

func (s *ValidateService) readResourceAgentContractV1(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return staticResourceText(req.Params.URI, resourceURIAgentContractV1, "text/markdown", agentContractPromptV1Markdown)
}

func staticResourceText(requestURI, expectedURI, mimeType, text string) (*mcp.ReadResourceResult, error) {
	if requestURI != expectedURI {
		return nil, mcp.ResourceNotFoundError(requestURI)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      expectedURI,
			MIMEType: mimeType,
			Text:     text,
		}},
	}, nil
}

func (s *ValidateService) effectivePolicyRef() string {
	if s.policyRef != "" {
		return s.policyRef
	}
	return "ops-v0.1.0-dev"
}

func boolPtr(v bool) *bool {
	return &v
}

// validateErrCode maps a validate package error to an ErrorSummary code and
// a safe-to-expose message. Internal details are never surfaced to callers.
func validateErrCode(err error) (code, message string) {
	switch {
	case errors.Is(err, validate.ErrInvalidInput):
		return ErrCodeInvalidInput, err.Error()
	case errors.Is(err, validate.ErrPolicyFailure):
		return ErrCodePolicyFailure, "policy evaluation failed"
	case errors.Is(err, validate.ErrEvidenceWrite):
		return ErrCodeEvidenceWrite, "evidence write failed"
	default:
		return ErrCodeInternalError, "internal error"
	}
}
