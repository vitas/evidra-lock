package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra/pkg/invocation"
)

var bundleDir = filepath.Join("..", "..", "policy", "bundles", "ops-v0.1")
var guidanceDir = filepath.Join("..", "..", "prompts", "mcpserver")

func testGuidanceContent() GuidanceContent {
	return mustLoadGuidanceContent(guidanceDir)
}

func TestValidateServiceReturnsDeny(t *testing.T) {
	opts := Options{
		BundlePath:   bundleDir,
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	svc := newValidateService(opts, testGuidanceContent())

	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "tester", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"payload": map[string]interface{}{
				"namespace": "prod",
			},
		},
		Context: map[string]interface{}{},
	}

	out := svc.Validate(context.Background(), inv)
	if out.Policy.Allow {
		t.Fatalf("expected policy to deny")
	}
	if out.EventID == "" {
		t.Fatalf("missing event id")
	}
	if len(out.RuleIDs) == 0 {
		t.Fatalf("expected rule ids")
	}
	found := false
	for _, id := range out.RuleIDs {
		if id == "ops.unapproved_change" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing ops.unapproved_change in %v", out.RuleIDs)
	}
	if len(out.Hints) == 0 {
		t.Fatalf("expected hints")
	}
}

func TestValidateServiceRecordsEvidence(t *testing.T) {
	opts := Options{
		BundlePath:   bundleDir,
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	svc := newValidateService(opts, testGuidanceContent())

	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "tester", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"target":  map[string]interface{}{"namespace": "default"},
			"payload": map[string]interface{}{"namespace": "default", "resource": "pod"},
		},
		Context: map[string]interface{}{},
	}

	out := svc.Validate(context.Background(), inv)
	if !out.Policy.Allow {
		t.Fatalf("expected policy to allow; rule_ids=%v reasons=%v", out.RuleIDs, out.Reasons)
	}
	if out.EventID == "" {
		t.Fatalf("missing event id")
	}
	event := svc.GetEvent(context.Background(), out.EventID)
	if !event.OK {
		t.Fatalf("get event failed: %+v", event.Error)
	}
	if event.Record == nil {
		t.Fatalf("expected record")
	}
}

func TestServerRegistersValidateTool(t *testing.T) {
	server := newTestServer(t)
	tools := listToolNamesFromServer(t, server)
	if !containsTool(tools, "validate") {
		t.Fatalf("expected validate tool in %v", tools)
	}
	if containsTool(tools, "execute") {
		t.Fatalf("did not expect execute tool by default")
	}
}

func TestServerInitializeInstructions(t *testing.T) {
	server := newTestServer(t)
	ctx := context.Background()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	init := clientSession.InitializeResult()
	if init == nil {
		t.Fatal("expected initialize result")
	}
	if strings.TrimSpace(init.Instructions) == "" {
		t.Fatal("expected non-empty initialize instructions")
	}
	for _, snippet := range []string{
		"Always call `validate` before destructive or privileged operations",
		"STOP and do not retry unchanged input",
		"missing data",
		"evidra://prompts/agent_contract_v1",
	} {
		if !strings.Contains(init.Instructions, snippet) {
			t.Fatalf("initialize instructions missing snippet %q", snippet)
		}
	}
}

func TestValidateToolDescriptionAndSchemaGuidance(t *testing.T) {
	server := newTestServer(t)
	tool, ok := findToolByName(listToolsFromServer(t, server), "validate")
	if !ok {
		t.Fatal("validate tool not found")
	}
	for _, snippet := range []string{
		"Evaluates intended infrastructure action(s) against the Evidra-Lock policy bundle",
		"Kubernetes payload may be a native manifest or a flat schema",
		"If allow=false: STOP",
		"Do not retry unchanged input",
		"If hints indicate missing data, request required fields and re-run validate",
	} {
		if !strings.Contains(tool.Description, snippet) {
			t.Fatalf("validate tool description missing snippet %q", snippet)
		}
	}

	schema := requireMap(t, tool.InputSchema, "validate.inputSchema")
	properties := requireMap(t, schema["properties"], "validate.inputSchema.properties")
	params := requireMap(t, properties["params"], "validate.inputSchema.properties.params")
	paramProps := requireMap(t, params["properties"], "validate.inputSchema.properties.params.properties")
	payload := requireMap(t, paramProps["payload"], "validate.inputSchema.properties.params.properties.payload")
	desc, ok := payload["description"].(string)
	if !ok || desc == "" {
		t.Fatal("validate payload description missing")
	}
	for _, snippet := range []string{
		"native manifest",
		"flat internal shape",
		"canonicalizes",
	} {
		if !strings.Contains(desc, snippet) {
			t.Fatalf("payload description missing snippet %q; got %q", snippet, desc)
		}
	}

	examples, ok := payload["examples"].([]interface{})
	if !ok || len(examples) < 2 {
		t.Fatalf("expected payload examples with native and flat snippets, got %T len=%d", payload["examples"], len(examples))
	}
	native, ok := examples[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected native example object, got %T", examples[0])
	}
	if kind, _ := native["kind"].(string); kind != "Deployment" {
		t.Fatalf("expected native example kind Deployment, got %v", native["kind"])
	}
	flat, ok := examples[1].(map[string]interface{})
	if !ok {
		t.Fatalf("expected flat example object, got %T", examples[1])
	}
	if resource, _ := flat["resource"].(string); resource != "deployment" {
		t.Fatalf("expected flat example resource deployment, got %v", flat["resource"])
	}
}

func TestGetEventToolSchema(t *testing.T) {
	server := newTestServer(t)
	tool, ok := findToolByName(listToolsFromServer(t, server), "get_event")
	if !ok {
		t.Fatal("get_event tool not found")
	}

	schema := requireMap(t, tool.InputSchema, "get_event.inputSchema")
	required, ok := schema["required"].([]interface{})
	if !ok || len(required) == 0 {
		t.Fatalf("expected non-empty required array, got %T", schema["required"])
	}
	if !containsSchemaString(required, "event_id") {
		t.Fatalf("expected required field event_id in get_event schema, got %v", required)
	}

	properties := requireMap(t, schema["properties"], "get_event.inputSchema.properties")
	eventID := requireMap(t, properties["event_id"], "get_event.inputSchema.properties.event_id")
	if typ, ok := eventID["type"].(string); !ok || typ != "string" {
		t.Fatalf("expected get_event event_id type string, got %T %v", eventID["type"], eventID["type"])
	}
}

func TestServerRegistersDocumentationResources(t *testing.T) {
	server := newTestServer(t)
	resources := listResourceURIsFromServer(t, server)

	for _, uri := range []string{
		resourceURIDocsEngineLogicV2,
		resourceURIDocsProtocolError,
		resourceURIPolicySummary,
		resourceURIAgentContractV1,
	} {
		if !containsResourceURI(resources, uri) {
			t.Fatalf("expected resource URI %q in %v", uri, resources)
		}
	}
}

func TestAgentContractResourceContainsRequiredClauses(t *testing.T) {
	server := newTestServer(t)
	text := readResourceTextFromServer(t, server, resourceURIAgentContractV1)

	for _, snippet := range []string{
		"Evidra-Lock Agent Contract v1",
		"Always Validate First",
		"STOP immediately",
		"-32602",
		"Large Manifests",
		"ONE `validate` call",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("agent contract resource missing snippet %q; got: %q", snippet, text)
		}
	}
}

func TestProtocolErrorsResourceMentions32602(t *testing.T) {
	server := newTestServer(t)
	text := readResourceTextFromServer(t, server, resourceURIDocsProtocolError)

	for _, snippet := range []string{
		"-32602",
		"Invalid params",
		"handlers are not invoked",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("protocol errors resource missing snippet %q; got: %q", snippet, text)
		}
	}
}

func TestPolicySummaryResourceIncludesGuidanceFlags(t *testing.T) {
	server := newTestServer(t)
	raw := readResourceTextFromServer(t, server, resourceURIPolicySummary)

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("policy summary JSON decode: %v; body=%q", err, raw)
	}

	if payload["policy_ref"] == "" {
		t.Fatalf("expected non-empty policy_ref in %v", payload)
	}

	guidance, ok := payload["guidance"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected guidance object, got %T", payload["guidance"])
	}
	if enabled, _ := guidance["initialize_instructions"].(bool); !enabled {
		t.Fatalf("expected initialize_instructions=true in %v", guidance)
	}
	if uri, _ := guidance["initialize_points_to"].(string); uri != resourceURIAgentContractV1 {
		t.Fatalf("expected initialize_points_to=%q in %v", resourceURIAgentContractV1, guidance)
	}
}

func newTestServer(t *testing.T) *mcp.Server {
	t.Helper()
	opts := Options{
		BundlePath:   bundleDir,
		ContentDir:   filepath.Join("..", "..", "prompts", "mcpserver"),
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}
	return NewServer(opts)
}

func listToolNamesFromServer(t *testing.T, srv *mcp.Server) []string {
	t.Helper()
	tools := listToolsFromServer(t, srv)
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func listToolsFromServer(t *testing.T, srv *mcp.Server) []*mcp.Tool {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()
	res, err := clientSession.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools RPC: %v", err)
	}
	return res.Tools
}

func listResourceURIsFromServer(t *testing.T, srv *mcp.Server) []string {
	t.Helper()
	resources := listResourcesFromServer(t, srv)
	var uris []string
	for _, r := range resources {
		uris = append(uris, r.URI)
	}
	return uris
}

func listResourcesFromServer(t *testing.T, srv *mcp.Server) []*mcp.Resource {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	res, err := clientSession.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		t.Fatalf("ListResources RPC: %v", err)
	}
	return res.Resources
}

func readResourceTextFromServer(t *testing.T, srv *mcp.Server, uri string) string {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	res, err := clientSession.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("ReadResource RPC for %s: %v", uri, err)
	}
	if len(res.Contents) == 0 {
		t.Fatalf("expected non-empty resource contents for %s", uri)
	}
	return res.Contents[0].Text
}

func containsTool(list []string, name string) bool {
	for _, tool := range list {
		if tool == name {
			return true
		}
	}
	return false
}

func containsResourceURI(list []string, uri string) bool {
	for _, item := range list {
		if item == uri {
			return true
		}
	}
	return false
}

func findToolByName(tools []*mcp.Tool, name string) (*mcp.Tool, bool) {
	for _, tool := range tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return nil, false
}

func requireMap(t *testing.T, v interface{}, name string) map[string]interface{} {
	t.Helper()
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("%s: expected map[string]interface{}, got %T", name, v)
	}
	return m
}

// denyInvocation returns a ToolInvocation that triggers a deny (kubectl delete in prod).
func denyInvocation(actorType string) invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: actorType, ID: "test-agent", Origin: "mcp"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"payload": map[string]interface{}{
				"namespace": "prod",
			},
		},
		Context: map[string]interface{}{},
	}
}

func TestValidate_DenyCacheDisabled_NeverBlocks(t *testing.T) {
	t.Parallel()

	svc := newValidateService(Options{
		BundlePath:       bundleDir,
		EvidencePath:     t.TempDir(),
		Mode:             ModeEnforce,
		DenyCacheEnabled: false,
	}, testGuidanceContent())

	inv := denyInvocation("agent")

	// First call: expect deny with full OPA evaluation
	out1 := svc.Validate(context.Background(), inv)
	if out1.Policy.Allow {
		t.Fatal("expected deny on first call")
	}
	if out1.Error != nil && out1.Error.Code == ErrCodeStopAfterDeny {
		t.Fatal("should not get stop_after_deny when cache disabled")
	}

	// Second call: same intent, should still run full evaluation (no stop_after_deny)
	out2 := svc.Validate(context.Background(), inv)
	if out2.Policy.Allow {
		t.Fatal("expected deny on second call")
	}
	if out2.Error != nil && out2.Error.Code == ErrCodeStopAfterDeny {
		t.Fatal("should not get stop_after_deny when cache disabled")
	}
	// Should have rule IDs from full eval
	if len(out2.RuleIDs) == 0 {
		t.Fatal("expected rule_ids from full OPA evaluation")
	}
}

func TestValidate_DenyCacheEnabled_BlocksRetry(t *testing.T) {
	t.Parallel()

	svc := newValidateService(Options{
		BundlePath:       bundleDir,
		EvidencePath:     t.TempDir(),
		Mode:             ModeEnforce,
		DenyCacheEnabled: true,
	}, testGuidanceContent())

	inv := denyInvocation("agent")

	// First call: full evaluation, deny
	out1 := svc.Validate(context.Background(), inv)
	if out1.Policy.Allow {
		t.Fatal("expected deny on first call")
	}
	if out1.Error != nil && out1.Error.Code == ErrCodeStopAfterDeny {
		t.Fatal("first call should never be stop_after_deny")
	}

	// Second call: identical intent, should be blocked by cache
	out2 := svc.Validate(context.Background(), inv)
	if out2.Policy.Allow {
		t.Fatal("expected deny on second call")
	}
	if out2.Error == nil || out2.Error.Code != ErrCodeStopAfterDeny {
		t.Fatalf("expected stop_after_deny on retry, got error=%+v", out2.Error)
	}
	if len(out2.Hints) == 0 {
		t.Fatal("expected hints on stop_after_deny")
	}
}

func TestValidate_DenyCacheEnabled_HumanNotGated(t *testing.T) {
	t.Parallel()

	svc := newValidateService(Options{
		BundlePath:       bundleDir,
		EvidencePath:     t.TempDir(),
		Mode:             ModeEnforce,
		DenyCacheEnabled: true,
	}, testGuidanceContent())

	inv := denyInvocation("human")

	// First call: deny
	out1 := svc.Validate(context.Background(), inv)
	if out1.Policy.Allow {
		t.Fatal("expected deny on first call")
	}

	// Second call: same intent but human actor, should still run full eval
	out2 := svc.Validate(context.Background(), inv)
	if out2.Policy.Allow {
		t.Fatal("expected deny on second call")
	}
	if out2.Error != nil && out2.Error.Code == ErrCodeStopAfterDeny {
		t.Fatal("human actors must never be gated by deny cache")
	}
	if len(out2.RuleIDs) == 0 {
		t.Fatal("expected rule_ids from full OPA evaluation for human")
	}
}

func TestValidate_DenyCacheEnabled_ChangedIntent_Reevaluates(t *testing.T) {
	t.Parallel()

	svc := newValidateService(Options{
		BundlePath:       bundleDir,
		EvidencePath:     t.TempDir(),
		Mode:             ModeEnforce,
		DenyCacheEnabled: true,
	}, testGuidanceContent())

	// First call: deny with prod namespace
	inv1 := denyInvocation("agent")
	out1 := svc.Validate(context.Background(), inv1)
	if out1.Policy.Allow {
		t.Fatal("expected deny on first call")
	}

	// Second call: different namespace (intent key changes), should get fresh eval
	inv2 := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "agent", ID: "test-agent", Origin: "mcp"},
		Tool:      "kubectl",
		Operation: "delete",
		Params: map[string]interface{}{
			"payload": map[string]interface{}{
				"namespace": "default",
				"resource":  "pod",
			},
		},
		Context: map[string]interface{}{},
	}
	out2 := svc.Validate(context.Background(), inv2)
	// Regardless of allow/deny, it should NOT be stop_after_deny
	if out2.Error != nil && out2.Error.Code == ErrCodeStopAfterDeny {
		t.Fatal("changed intent should get fresh evaluation, not stop_after_deny")
	}
}

func TestValidateServiceBadPolicyReturnsCode(t *testing.T) {
	svc := newValidateService(Options{
		PolicyPath:   "nonexistent.rego",
		DataPath:     "nonexistent.json",
		EvidencePath: t.TempDir(),
		Mode:         ModeEnforce,
	}, testGuidanceContent())
	inv := invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "human", ID: "u1", Origin: "test"},
		Tool:      "kubectl",
		Operation: "apply",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	}
	out := svc.Validate(context.Background(), inv)
	if out.OK {
		t.Fatal("expected OK=false when policy load fails")
	}
	if out.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if out.Error.Code != ErrCodePolicyFailure {
		t.Errorf("expected error code %q, got %q", ErrCodePolicyFailure, out.Error.Code)
	}
}

func TestValidateServiceInvalidInputReturnsCode(t *testing.T) {
	svc := newValidateService(Options{Mode: ModeEnforce}, testGuidanceContent())
	out := svc.Validate(context.Background(), invocation.ToolInvocation{})
	if out.OK {
		t.Fatal("expected OK=false for invalid input")
	}
	if out.Error == nil {
		t.Fatal("expected Error to be set")
	}
	if out.Error.Code != ErrCodeInvalidInput {
		t.Errorf("expected error code %q, got %q", ErrCodeInvalidInput, out.Error.Code)
	}
}
