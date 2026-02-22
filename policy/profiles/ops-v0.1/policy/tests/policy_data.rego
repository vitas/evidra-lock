package evidra.policy

policy_test_data := {
  "rule_hints": {
    "autonomous-execution": [
      "Agent-originated MCP action detected; require explicit human review."
    ],
    "kube-system-breakglass": [
      "Add risk_tags=[\"breakglass\"] for controlled kube-system changes."
    ],
    "mass-delete": [
      "Reduce delete scope below threshold or add risk_tags=[\"breakglass\"]."
    ],
    "prod-requires-approval": [
      "Add risk_tags=[\"change-approved\"] before targeting prod namespace."
    ],
    "public-exposure-requires-approval": [
      "Add risk_tags=[\"approved_public\"] before allowing public exposure."
    ]
  },
  "thresholds": {
    "mass_delete_max": 5
  }
}
