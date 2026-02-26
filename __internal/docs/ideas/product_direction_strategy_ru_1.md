🧠 Чем /skills реально хороши
1️⃣ Они превращают Evidra из “policy engine” в “AI action control plane”
Без skills вы:
•	принимаете ToolInvocation
•	отвечаете allow/deny
Со skills вы:
•	регистрируете именованные действия
•	даёте схему
•	закрепляете risk_tags
•	делаете idempotency
•	создаёте execution metadata
Это уже orchestration.
 
2️⃣ Они решают главную боль AI-агентов
LLM плохо умеют:
•	правильно формировать сложные invocation
•	помнить правильные risk_tags
•	не эскалировать privileges
Skills дают:
•	schema enforcement
•	immutable risk_tags
•	controlled surface
Это очень сильная модель.
 
3️⃣ Это abstraction layer над ToolInvocation
Вы уходите от:
“вот raw terraform apply”
к:
“deploy-service”, “scale-deployment”, “rotate-secret”
Это UX-победа.
Куда ещё можно встроиться (и возможно лучше, чем skills)
Вот это важнее.
 
1️⃣ GitHub Actions Guardrail
Перед terraform apply:
- name: Validate via Evidra
  run: |
    curl https://evidra.rest/v1/validate ...
Это CI gate.
Это очень понятный use-case.
Это massive market.
 
2️⃣ Terraform Cloud Run Task
Terraform Cloud поддерживает run tasks.
Evidra как:
“AI policy gate before apply”
Это прямой рынок.
 
3️⃣ Kubernetes Admission Controller (через Hosted API)
OPA Gatekeeper — after kubectl.
Evidra — before kubectl.
Но можно сделать и:
•	lightweight validating webhook → calls evidra-api
Тогда вы:
•	и AI gate
•	и infra gate
 
4️⃣ Claude / OpenAI Tool Guard
Evidra как remote MCP server:
•	Claude Code
•	Cursor
•	Continue.dev
•	VSCode agents
Это ваш натуральный рынок.
 
5️⃣ Pulumi / CDK plugin
Очень сильная ниша:
AI + IaC + policy.
 
Вот честная стратегическая мысль
Skills — это не “куда встроиться”.
Это способ:
стать workflow orchestration layer для AI infra operations.
Это более крупная игра.
Но и более тяжёлая.
 
Если бы я выбирал порядок
1️⃣ Выпустить hosted validate-only
2️⃣ Сделать SDK / thin client
3️⃣ Интеграции (GitHub, Terraform Cloud, Claude MCP)
4️⃣ Только потом skills
 
Когда skills станут killer feature
Когда появится:
"Multi-agent infrastructure operations"
Например:
•	AI планирует
•	AI применяет
•	AI откатывает
•	AI делает scaling
•	AI создаёт PR
И всё через skills.
 
Очень важный вопрос
Что вы хотите быть?
A) Stripe for AI infrastructure policy
(Stateless, clean, simple, scalable)
B) Control plane for AI infra actions
(Workflow, state, skills, orchestration)
Оба пути валидны.
Но это разные продукты.
 
Моя честная рекомендация
Сейчас:
•	/validate — ваш продукт
•	/skills — ваша опция
Не наоборот.

