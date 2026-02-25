Что бы я добавил, чтобы стало “нужно” (roadmap без воды)
A) “One-command demo”

Сделай в README блок типа:

brew install evidra/tap/evidra
evidra-mcp --bundle policy/bundles/ops-v0.1
# в другом окне:
evidra validate examples/terraform_public_exposure_fail.json --explain

И рядом: expected output (PASS/FAIL + rule_ids + hints + evidence_id).

B) “Starter policy packs”

Сейчас policy bundle есть, тесты есть — круто.
Но скачивания растут, когда есть:

pack-kubernetes-basics

pack-terraform-basics

pack-prod-guardrails
и 5–10 правил, которые узнают все (“public exposure”, “mass delete”, “kube-system/prod namespace”, “needs approval tag”).

C) “Evidence report” как продуктовая фича

У тебя есть evidence inspect/report. Это можно поднять как killer-feature:

“сгенерируй markdown/html отчёт для change review”

“ссылка на evidence_id → покажи цепочку/контекст/параметры”

Даже локально — это прям вкусно для команд.

Идея “сделать как backend для skills AI agent” — да, это уже почти оно

Сейчас ты — policy gate + audit store. Чтобы стать прям “backend для skills” не хватает пары API/тулов:

Минимальный набор MCP tools (помимо validate)

explain_rule(rule_id) — что это за правило, примеры, как чинить

list_policies() / list_rule_ids() — чтобы агент сам находил ограничения

render_evidence_report(event_id, format=md|json) — отчёт для человека

(опционально) diff_policy(baseline, candidate) — если ты хочешь “policy-as-code review”

Тогда агент сможет:

сам себя ограничивать

сам исправляться по hints

сам прикладывать отчёт в PR/тикет

Быстрый чек-лист “что не хватает” (самое важное)

Релизить evidra-mcp так же, как evidra (goreleaser + make build).

Убрать бинарники из репо, оставить только исходники.

Добавить в README:

“who it’s for”

60-сек демо

один “реальный” сценарий (Terraform plan / kubectl delete)

GitHub Action / pre-commit пример.

1–2 дополнительных MCP tools для “agent backend” (хотя бы explain/list).