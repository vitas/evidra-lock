package evidra.policy

import rego.v1

# ── terraform dynamic hint ──────────────────────────────

test_insufficient_context_terraform_apply if {
	inp := {
		"tool": "terraform",
		"operation": "apply",
		"actions": [{
			"kind": "terraform.apply",
			"target": {},
			"risk_tags": [],
			"payload": {},
		}],
	}
	d := data.evidra.policy.decision with input as inp
	not d.allow
	"ops.insufficient_context" in d.hits
	some r in d.reasons
	contains(r, "terraform.apply")
	contains(r, "resource_types")
}

# ── kubectl delete dynamic hint ─────────────────────────

test_insufficient_context_kubectl_delete if {
	inp := {
		"tool": "kubectl",
		"operation": "delete",
		"actions": [{
			"kind": "kubectl.delete",
			"target": {},
			"risk_tags": [],
			"payload": {},
		}],
	}
	d := data.evidra.policy.decision with input as inp
	not d.allow
	"ops.insufficient_context" in d.hits
	some r in d.reasons
	contains(r, "kubectl.delete")
	contains(r, "namespace")
}

# ── helm wildcard match ─────────────────────────────────

test_insufficient_context_helm_upgrade if {
	inp := {
		"tool": "helm",
		"operation": "upgrade",
		"actions": [{
			"kind": "helm.upgrade",
			"target": {},
			"risk_tags": [],
			"payload": {},
		}],
	}
	d := data.evidra.policy.decision with input as inp
	not d.allow
	"ops.insufficient_context" in d.hits
	some r in d.reasons
	contains(r, "helm.upgrade")
	contains(r, "namespace")
}

# ── skeleton in hints ───────────────────────────────────

test_insufficient_context_skeleton_in_hints if {
	inp := {
		"tool": "terraform",
		"operation": "destroy",
		"actions": [{
			"kind": "terraform.destroy",
			"target": {},
			"risk_tags": [],
			"payload": {},
		}],
	}
	d := data.evidra.policy.decision with input as inp
	not d.allow
	some h in d.hints
	contains(h, "destroy_count")
}

# ── argocd.project fallback (no exact match, no wildcard) ──

test_insufficient_context_argocd_project_fallback if {
	inp := {
		"tool": "argocd",
		"operation": "project",
		"actions": [{
			"kind": "argocd.project",
			"target": {},
			"risk_tags": [],
			"payload": {},
		}],
	}
	d := data.evidra.policy.decision with input as inp
	not d.allow
	"ops.insufficient_context" in d.hits
	some r in d.reasons
	contains(r, "has_sufficient_context")
}

# ── sufficient context passes ────────────────────────────

test_sufficient_context_kubectl_delete_passes if {
	inp := {
		"tool": "kubectl",
		"operation": "delete",
		"actions": [{
			"kind": "kubectl.delete",
			"target": {"namespace": "default"},
			"risk_tags": [],
			"payload": {"namespace": "default", "resource_count": 1},
		}],
	}
	d := data.evidra.policy.decision with input as inp
	d.allow
	not "ops.insufficient_context" in d.hits
}
