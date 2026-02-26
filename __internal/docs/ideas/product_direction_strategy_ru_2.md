🎯 Идеальный ICP (initial customer profile)

Компания уже использует:

Claude Code

Cursor

AI для IaC

У них есть страх:

“AI случайно применит что-то в prod”

Им не нужна orchestration.
Им нужен switch.

💰 Модель монетизации
Free:

10k requests/month

no evidence storage

Pro:

higher limits

managed evidence store (который ты уже продумал)

SLA

Enterprise:

private deployment

multi-key per tenant

SSO

📦 Что надо сделать для этого пути

Hosted validate-only polished

SDK (Go + TS)

GitHub Action template

Claude MCP example

Landing page с 3 примерами

Это можно сделать быстрее, чем полноценную skills platform.

🧠 Путь №2 — Если идти в orchestration (skills-first)

Тогда вы уже не guardrail.

Вы:

“AI Infrastructure Action Control Plane”

Это значит:

Skills — first-class concept

Execution history

Policy per skill

Workflow chaining

Possibly async execution

Это уже почти:

Temporal-lite

Orchestration engine

Compliance layer

Это сильно.
Но это x5 complexity.

🧨 Где skills становятся killer feature

Вот когда они реально стреляют:

1️⃣ AI change management system

Skill:

create-change-request

approve-change

deploy

Policy:

no deploy without change approved

Это enterprise gold.

2️⃣ Multi-agent governance

Agent A → propose

Agent B → approve

Agent C → execute

Skills становятся control contract.

3️⃣ Regulated industries

Auditability + deterministic signing + skill definitions.

Это банковская/финтех игра.

🔥 5 неожиданных интеграций (где вы можете красиво встроиться)

Вот интересная часть.

1️⃣ Cursor / VSCode extension

Не MCP.

А extension, который:

intercepts “apply”

делает call в evidra

показывает modal: “Denied by policy”

Это очень сильный UX.

2️⃣ Terraform Cloud Run Task

Terraform Cloud уже поддерживает pre-apply hooks.

Вы можете стать:

AI-aware policy layer before apply.

Это прямой рынок.

3️⃣ GitHub Copilot Guard

Imagine:

Copilot предлагает PR с Terraform.
GitHub Action автоматически валидирует через Evidra.

Это CI security layer.

4️⃣ Datadog / SIEM evidence forwarder

Evidence record → forward to SIEM.

Compliance teams любят это.

5️⃣ Cloudflare Workers Edge Guard

Очень неожиданный ход:

Deploy lightweight worker:

intercept API requests

validate via Evidra

Edge-level AI infra guard.

🧠 Теперь важный момент

Тебе нужно принять стратегическое решение:

Вопрос	Если ответ “да” →
Хотим быть минималистичным infra guard?	Focus on validate
Хотим быть AI infra control plane?	Build skills deeply
Хотим быть compliance engine?	Build managed evidence
Хотим быть dev-tool?	Build SDK + CLI + plugins
🎯 Мой честный совет

Сейчас:

Выпусти hosted validate-only

Сделай integration examples

Получи 10 реальных пользователей

Посмотри, что они просят

Skills ты уже продумал.
Они не пропадут.

Но validate-only — это:

чище

проще

понятнее

быстрее к adoption

Если хочешь — дальше можем сделать:

конкретный go-to-market план на 60 дней

или решить: “мы guardrail” vs “мы control plane” окончательно

или разобрать, какой из путей даст больше valuation в 3–5 лет

Тут начинается уже стратегическая игра 🙂