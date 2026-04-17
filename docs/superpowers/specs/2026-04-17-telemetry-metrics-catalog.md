# Telemetry Metrics Catalog

**Date:** 2026-04-17
**Status:** Draft
**Audience:** Engineering, Security/Compliance, Leadership

A comprehensive catalog of every metric, attribute, and data point available from Claude Code, Claude Chat, Claude Cowork, and the Anthropic Admin APIs — organized by privacy tier with compliance mapping.

---

## Executive Summary

### Answers to the Three Key Questions

**Can we log actual queries?**
Yes. Claude Code requires opt-in (`OTEL_LOG_USER_PROMPTS=1` for prompt text, `OTEL_LOG_RAW_API_BODIES=1` for full request/response JSON). Claude Cowork sends full prompt text by default. Claude Chat has no direct telemetry — content is only accessible via the Compliance API (Enterprise, control-plane events only, not conversation content).

**Can we detect PII in queries?**
Not natively. Anthropic provides no built-in PII detection. Options: (1) build a custom OTel processor in our collector pipeline, (2) use a third-party PII scanner (e.g., AWS Comprehend, Presidio) on telemetry data before storage, or (3) use prompt-level heuristics (regex for SSNs, emails, credit cards) in a new processor. This is a gap we should fill.

**Can we monitor file changes?**
Yes, with opt-in. When `OTEL_LOG_TOOL_DETAILS=1` is set, Claude Code emits `tool_result` events containing file paths for Read/Write/Edit operations, bash commands, and git commit IDs. Claude Cowork emits file paths accessed via MCP tools by default. You can monitor *which* files changed without logging *what* changed by enabling tool details but not tool content.

### What's Possible at Each Privacy Tier

| Tier | What You Get | PII Risk |
|------|-------------|----------|
| **Default** (no opt-in) | Session counts, token counts, cost, model usage, lines of code, commits, PRs, tool accept/reject rates | None |
| **Tool Details** | + File paths, bash commands, MCP server/tool names, git commit IDs | Low — file paths may reveal project names |
| **User Prompts** | + Full user prompt text | High — prompts may contain PII, secrets, proprietary code |
| **Tool Content** | + Full tool input/output (file contents, bash output, up to 60KB) | Very High — raw file contents, terminal output |
| **Raw API Bodies** | + Complete Messages API request/response JSON (up to 60KB) | Maximum — full conversation history including system prompts |

---

## 1. Data Sources

### 1.1 Claude Code (OTel)

**Protocol:** OTLP (HTTP port 4318, gRPC port 4317)
**Signal types:** Metrics, Events/Logs, Traces (beta)
**Configuration:** Environment variables on each developer machine or CI runner

Required env vars:
```
CLAUDE_CODE_ENABLE_TELEMETRY=1
OTEL_EXPORTER_OTLP_ENDPOINT=http://<collector>:4318
OTEL_METRICS_EXPORTER=otlp
OTEL_LOGS_EXPORTER=otlp
```

Optional opt-in flags (see Privacy Tiers below):
```
OTEL_TRACES_EXPORTER=otlp                  # Traces (beta)
CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1       # Required for traces
OTEL_LOG_USER_PROMPTS=1                     # Prompt text
OTEL_LOG_TOOL_DETAILS=1                     # File paths, commands
OTEL_LOG_TOOL_CONTENT=1                     # Full tool I/O
OTEL_LOG_RAW_API_BODIES=1                   # Full API request/response
```

### 1.2 Claude Chat (claude.ai)

**Protocol:** No direct OTel. Admin APIs only.
**Signal types:** Aggregated usage/cost data via Admin API
**Limitations:** No per-conversation telemetry. No content access. Compliance API covers control-plane events (user management, settings changes) but NOT conversation content.

### 1.3 Claude Cowork

**Protocol:** OTLP (configured via Claude Desktop Organization settings)
**Signal types:** Events/Logs (same OTel protocol as Claude Code)
**Key difference from Claude Code:** User prompt text is sent by default (no opt-in required). File access paths are included by default.

Configuration is managed centrally via Claude Desktop Organization settings (not per-machine env vars):
- OTLP endpoint, protocol, and auth headers

### 1.4 Anthropic Admin APIs

**Protocol:** REST API (`api.anthropic.com`)
**Authentication:** Admin API key with appropriate permissions

| API | Endpoint | Granularity | Freshness |
|-----|----------|-------------|-----------|
| Usage | `/v1/organizations/usage_report/messages` | 1m / 1h / 1d buckets | ~5 min |
| Cost | `/v1/organizations/cost_report` | Daily | ~5 min |
| Claude Code Analytics | `/v1/organizations/usage_report/claude_code` | Per-user daily | ~1 hour |
| Organization Management | `/v1/organizations/*` | Real-time | Immediate |
| Compliance (Enterprise) | Compliance API | Event-level | Near real-time |

---

## 2. Privacy Tiers

### Tier 0: Default (No Opt-In Required)

Everything in this tier is emitted automatically when `CLAUDE_CODE_ENABLE_TELEMETRY=1` is set. No PII risk.

#### Metrics

| Metric | Type | Attributes | Description |
|--------|------|-----------|-------------|
| `claude_code.session.count` | Counter | standard | Number of sessions started |
| `claude_code.lines_of_code.count` | Counter | `type` (added/removed) | Lines of code added or removed |
| `claude_code.pull_request.count` | Counter | standard | PRs created by Claude Code |
| `claude_code.commit.count` | Counter | standard | Commits created by Claude Code |
| `claude_code.cost.usage` | Counter | `model` | Estimated cost in USD |
| `claude_code.token.usage` | Counter | `type` (input/output/cacheRead/cacheCreation), `model` | Token consumption by type and model |
| `claude_code.code_edit_tool.decision` | Counter | `tool_name`, `decision`, `source`, `language` | Code edit accept/reject rates |
| `claude_code.active_time.total` | Counter | `type` (user/cli) | Active usage time |

#### Events

| Event | Key Attributes | Description |
|-------|---------------|-------------|
| `claude_code.user_prompt` | `prompt_length` | Prompt received (length only, no content) |
| `claude_code.tool_result` | `tool_name`, `success`, `duration_ms`, `error`, `decision_type`, `decision_source`, `tool_result_size_bytes` | Tool execution result |
| `claude_code.api_request` | `model`, `cost_usd`, `duration_ms`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `request_id`, `speed` | API call completed |
| `claude_code.api_error` | `model`, `error`, `status_code`, `duration_ms`, `attempt`, `request_id`, `speed` | API call failed |
| `claude_code.tool_decision` | `tool_name`, `decision`, `source` | Tool permission decision |
| `claude_code.plugin_installed` | `plugin.name`, `plugin.version`, `marketplace.name`, `install.trigger` | Plugin installed |
| `claude_code.skill_activated` | `skill.name`, `skill.source`, `plugin.name` | Skill activated |

#### Standard Attributes (All Signals)

| Attribute | Type | Description |
|-----------|------|-------------|
| `session.id` | string | Unique session identifier |
| `app.version` | string | Claude Code version |
| `organization.id` | string | Anthropic organization ID |
| `user.account_uuid` | string | User account UUID |
| `user.account_id` | string | User account ID |
| `user.id` | string | User identifier |
| `user.email` | string | User email address |
| `terminal.type` | string | Terminal type (e.g., vscode, iterm2) |
| `prompt.id` | string (UUID) | Correlates all events from one user prompt |

#### Resource Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| `service.name` | string | Always `claude-code` |
| `service.version` | string | Claude Code version |
| `os.type` | string | Operating system |
| `os.version` | string | OS version |
| `host.arch` | string | CPU architecture |
| `wsl.version` | string | WSL version (if applicable) |

### Tier 1: Tool Details (`OTEL_LOG_TOOL_DETAILS=1`)

Adds operational context to tool events. Low PII risk (file paths, command strings).

| Attribute | Event | Description |
|-----------|-------|-------------|
| `bash_command` | `tool_result` | Bash command executed |
| `full_command` | `tool_result` | Full command with arguments |
| `git_commit_id` | `tool_result` | Git commit SHA (on successful git commit) |
| `mcp_server_name` | `tool_result` | MCP server name |
| `mcp_server_scope` | `tool_result` | MCP server scope |
| `mcp_tool_name` | `tool_result` | MCP tool name |
| `skill_name` | `tool_result` | Skill name invoked |
| `tool_input` | `tool_result` | Tool input parameters (truncated ~4KB) — includes file paths for Read/Write/Edit |

### Tier 2: User Prompts (`OTEL_LOG_USER_PROMPTS=1`)

Adds the full text of user prompts. **High PII risk.**

| Attribute | Event | Description |
|-----------|-------|-------------|
| `prompt` | `claude_code.user_prompt` | Full user prompt text |

### Tier 3: Tool Content (`OTEL_LOG_TOOL_CONTENT=1`)

Adds full tool input/output to trace spans. **Very high PII risk.**

| Attribute | Location | Description |
|-----------|----------|-------------|
| Tool input content | Trace span events | Full input to tools (truncated 60KB) — includes raw file contents from Read |
| Tool output content | Trace span events | Full output from tools (truncated 60KB) — includes bash stdout, file contents |

### Tier 4: Raw API Bodies (`OTEL_LOG_RAW_API_BODIES=1`)

Full Messages API request/response JSON. **Maximum PII risk — contains entire conversation history.**

| Event | Description |
|-------|-------------|
| `claude_code.api_request_body` | Full JSON request body sent to Anthropic API |
| `claude_code.api_response_body` | Full JSON response body from Anthropic API |

Both truncated at 60KB.

---

## 3. Claude Cowork Telemetry

Cowork uses the same OTel protocol but has different defaults and additional data.

### Default Emissions (No Opt-In)

| Data | Description | Notes |
|------|-------------|-------|
| User prompts | Full prompt text | **Sent by default** (unlike Claude Code) |
| Tool/MCP invocations | Server name, tool name, parameters, success/failure, duration | Default |
| File access paths | Files read/modified/touched, including via MCPs | Default |
| Skills and plugins | Which ones Claude invokes | Default |
| Human approval decisions | Approved, rejected, or automatic | Default |
| API requests | Model, token counts, estimated cost, duration | Default |
| API errors | Error type, status code, duration | Default |
| User email | User's email address | Always included |
| `prompt.id` | Correlation ID for all events from one prompt | Same as Claude Code |

### Key Differences from Claude Code

| Aspect | Claude Code | Claude Cowork |
|--------|------------|---------------|
| Prompt content | Opt-in (`OTEL_LOG_USER_PROMPTS=1`) | **Default** |
| File paths | Opt-in (`OTEL_LOG_TOOL_DETAILS=1`) | **Default** |
| Configuration | Per-machine env vars | Centralized org settings |
| MCP tool details | Opt-in | **Default** |

---

## 4. Anthropic Admin APIs

### 4.1 Usage API

**Endpoint:** `GET /v1/organizations/usage_report/messages`
**Granularity:** 1-minute (up to 1440), 1-hour (up to 168), 1-day (up to 31)

| Field | Description |
|-------|-------------|
| `uncached_input_tokens` | Input tokens not from cache |
| `cached_input_tokens` | Input tokens served from cache |
| `cache_creation_tokens` | Tokens used creating cache entries |
| `output_tokens` | Output tokens generated |
| `model` | Model identifier |
| `workspace_id` | Workspace |
| `api_key_id` | API key used |
| `service_tier` | Priority tier |
| `context_window` | Context window size |
| `inference_geo` | Geographic region of inference |
| `speed` | fast/normal (fast mode beta) |
| Server tool usage | Web search, code execution token counts |

### 4.2 Cost API

**Endpoint:** `GET /v1/organizations/cost_report`
**Granularity:** Daily only

| Field | Description |
|-------|-------------|
| `cost_usd` | Total USD cost |
| `workspace_id` | Workspace breakdown |
| `description` | Cost category |
| Token/web search/code execution costs | Itemized cost components |

Note: Priority Tier costs are NOT included — use the Usage API instead.

### 4.3 Claude Code Analytics API

**Endpoint:** `GET /v1/organizations/usage_report/claude_code`
**Granularity:** Per-user daily aggregated

| Field | Description |
|-------|-------------|
| `num_sessions` | Sessions per user per day |
| `lines_of_code.added` | Lines added |
| `lines_of_code.removed` | Lines removed |
| `commits_by_claude_code` | Commits created |
| `pull_requests_by_claude_code` | PRs created |
| `edit_tool` (accepted/rejected) | Edit tool usage |
| `multi_edit_tool` (accepted/rejected) | Multi-edit tool usage |
| `write_tool` (accepted/rejected) | Write tool usage |
| `notebook_edit_tool` (accepted/rejected) | Notebook edit tool usage |
| Per-model token breakdown | input/output/cache_read/cache_creation per model |
| `estimated_cost` | Per-user cost estimate |
| `terminal_type` | Terminal used |
| `customer_type` | api/subscription |

**Freshness:** ~1 hour

### 4.4 Compliance API (Enterprise Only)

Launched March 2026. Control-plane audit events — NOT content audit.

| Event Category | Examples |
|---------------|----------|
| Admin/system events | Adding/removing workspace members, creating API keys, changing settings |
| Resource events | Creating, downloading, deleting files and skills |

**Does NOT log:** Inference activity, conversation content, prompts.
**No retroactive data** — logging starts at enablement.

---

## 5. Derived Metrics

These can be computed from the raw telemetry in our collector or downstream analytics:

| Derived Metric | Source Data | Description |
|---------------|-------------|-------------|
| Cost per user per day | `claude_code.api_request` × user | Aggregate cost attribution |
| Cost per team per day | Above + team mapping | Team-level cost allocation |
| Cache hit ratio | `cache_read_tokens` / (`cache_read_tokens` + `uncached_input_tokens`) | Cache efficiency |
| Tool acceptance rate | `code_edit_tool.decision` (accepted vs rejected) | Code quality signal |
| Average tokens per session | `token.usage` / `session.count` | Usage intensity |
| Error rate by model | `api_error` / (`api_request` + `api_error`) | Model reliability |
| Files modified per session | Count of Write/Edit `tool_result` events per session | Scope of changes |
| Prompt-to-completion latency | `api_request.duration_ms` | Performance tracking |
| Active time per user | `active_time.total` by user | Productivity signal |
| Lines of code per dollar | `lines_of_code.count` / `cost.usage` | ROI metric |

---

## 6. Answers to Key Questions (Detailed)

### 6.1 Can We Log Actual Queries?

**Yes, with caveats by product:**

| Product | Can Log Queries? | How | Default? |
|---------|-----------------|-----|----------|
| Claude Code | Yes | `OTEL_LOG_USER_PROMPTS=1` for prompt text; `OTEL_LOG_RAW_API_BODIES=1` for full request/response | No — opt-in |
| Claude Cowork | Yes | Prompt text sent via OTel by default | **Yes** |
| Claude Chat | No | No telemetry API. Compliance API covers admin actions only | N/A |

**Recommendation:** Enable `OTEL_LOG_USER_PROMPTS=1` for Claude Code. Route prompts to S3 (via contentfilter in `keep` mode) for audit, strip before DataDog. This matches our V2 architecture plan.

### 6.2 Can We Detect PII in Queries?

**Not natively. Anthropic provides no built-in PII detection.**

Options ranked by implementation effort:

| Option | Effort | Coverage | Description |
|--------|--------|----------|-------------|
| **Regex-based OTel processor** | Low | Basic | Custom processor matching patterns (SSN, email, credit card, phone). Add to our collector pipeline. |
| **LLM-based classification** | Medium | Good | Use a small model (Haiku) to classify prompts for PII before storage. Higher latency and cost. |
| **Third-party PII scanner** | Medium | Good | AWS Comprehend, Microsoft Presidio, or Google DLP on telemetry data before S3 storage. |
| **Full DLP pipeline** | High | Best | Dedicated data loss prevention system scanning all telemetry. Enterprise-grade but complex. |

**Recommendation:** Start with a regex-based OTel processor (new `internal/piidetector/` package) that tags spans with `claude.pii.detected=true` and `claude.pii.types=["email","ssn"]` without modifying the content. This lets us alert on PII presence and route PII-containing spans differently, without building a full DLP system upfront.

### 6.3 Can We Monitor File Changes?

**Yes. Multiple levels of visibility:**

| Level | What You See | Config Required |
|-------|-------------|----------------|
| **Counts only** | Lines added/removed, commit count, PR count | Default (Tier 0) |
| **File paths** | Which files were read/written/edited, bash commands, git SHAs | `OTEL_LOG_TOOL_DETAILS=1` (Tier 1) |
| **File contents** | Full file contents read/written (truncated 60KB) | `OTEL_LOG_TOOL_CONTENT=1` (Tier 3) |

For Cowork, file paths are available by default (no opt-in).

**Recommendation:** Enable Tier 1 (`OTEL_LOG_TOOL_DETAILS=1`) across the org. This gives file path visibility without exposing file contents — the right balance for monitoring without excessive PII risk. Build a dashboard showing:
- Files modified per session (scope tracking)
- Most-frequently-edited files (hotspot analysis)
- Tool usage patterns (which tools are used most)

---

## 7. Compliance Mapping

### 7.1 SOC 2

| Control Area | Available Data | Gaps |
|-------------|---------------|------|
| Access control | `user.email`, `organization.id`, `user.account_uuid` on all events | Need to correlate with IdP for role-based access verification |
| Change management | `commit.count`, `pull_request.count`, file paths (Tier 1) | No approval workflow tracking within telemetry |
| Audit trail | All events timestamped with `session.id` and `prompt.id` correlation | Compliance API (Enterprise) adds admin action audit trail |
| Availability monitoring | `api_error` events, `health` extension at `:13133/ping` | Add uptime monitoring on collector itself |

### 7.2 GDPR / CCPA

| Requirement | Status | Notes |
|-------------|--------|-------|
| Data minimization | Configurable via privacy tiers | Default tier collects no PII beyond user identifiers |
| Right to erasure | Gap | No mechanism to purge a user's telemetry from DataDog/S3. Need retention policies. |
| Consent | Partial | Opt-in flags act as technical consent, but need legal consent framework |
| Data processing records | Available | Collector config documents exactly what is processed and where it goes |
| Cross-border transfer | Configurable | `inference_geo` attribute tracks where inference happens; S3 bucket region is configurable |
| PII identification | Gap | No automated PII detection (see Section 6.2) |

### 7.3 Internal Policy

| Policy Area | Available Data | Recommendation |
|------------|---------------|----------------|
| Cost governance | `cost.usage`, Cost API, per-user analytics | Set up per-team cost alerts in DataDog |
| Usage monitoring | Sessions, tokens, active time, tool decisions | Dashboard per team/user |
| Security review | File paths, bash commands (Tier 1), tool decisions | Alert on sensitive file access patterns |
| Model governance | `model` attribute on all API requests | Track model usage distribution, enforce allowed models |

---

## 8. Gap Analysis & Recommendations

### Gaps

| Gap | Impact | Priority |
|-----|--------|----------|
| **No PII detection** | Cannot flag sensitive data in prompts/responses | High |
| **No Claude Chat telemetry** | Blind spot for non-Code, non-Cowork usage | Medium |
| **No right-to-erasure mechanism** | GDPR non-compliance for stored telemetry | High |
| **No V2 claudereceiver** | Admin API data not flowing through collector pipeline | Medium |
| **No S3 exporter** | Raw content not archived for audit/compliance | Medium |
| **No alerting on sensitive file access** | Security monitoring gap | Medium |
| **No session-level aggregation** | Cannot easily answer "what happened in session X?" | Low |

### Recommended Next Steps

1. **Build PII detector processor** — `internal/piidetector/` that tags spans with PII presence/types. Enables alerting and routing without content modification.
2. **Enable Tier 1 org-wide** — `OTEL_LOG_TOOL_DETAILS=1` gives file path and command visibility with minimal PII risk.
3. **Implement V2 claudereceiver** — Poll Admin APIs (Usage, Cost, Analytics) to bring all data into the collector pipeline.
4. **Add S3 exporter** — Archive raw telemetry (with content) for compliance audit trail with configurable retention.
5. **Define data retention policies** — Per-tier retention (e.g., Tier 0 forever, Tier 4 for 90 days) to support right-to-erasure.
6. **Build DataDog dashboards** — Cost per team, tool acceptance rates, file hotspots, error rates.

---

## Appendix: Complete Attribute Reference

### All `claude.*` Attributes (Current Collector)

| Attribute | Type | Set By | Description |
|-----------|------|--------|-------------|
| `claude.model` | string | Claude Code/Cowork | Model identifier |
| `claude.tokens.input` | int | Claude Code/Cowork | Input token count |
| `claude.tokens.output` | int | Claude Code/Cowork | Output token count |
| `claude.user` | string | Claude Code/Cowork | User identifier |
| `claude.cost.usd` | double | claudeprocessor | Calculated cost |
| `claude.team` | string | claudeprocessor | Team attribution (from mapping) |
| `claude.content.prompt` | string | Claude Code/Cowork | Full prompt text (Tier 2+) |
| `claude.content.response` | string | Claude Code/Cowork | Full response text (Tier 3+) |
| `claude.product` | string | V2 planned | `code`, `chat`, `cowork` |
| `claude.conversation_id` | string | V2 planned | Conversation identifier |
| `claude.tools` | string | V2 planned | Tools used |
| `claude.error` | string | V2 planned | Error type |

### Environment Variable Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_CODE_ENABLE_TELEMETRY` | `0` | Master telemetry switch |
| `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA` | `0` | Enable trace spans |
| `OTEL_METRICS_EXPORTER` | `none` | Metrics exporter (otlp/prometheus/console/none) |
| `OTEL_LOGS_EXPORTER` | `none` | Logs/events exporter (otlp/console/none) |
| `OTEL_TRACES_EXPORTER` | `none` | Traces exporter (otlp/console/none) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | Collector endpoint URL |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` | OTLP protocol |
| `OTEL_EXPORTER_OTLP_HEADERS` | — | Auth headers |
| `OTEL_LOG_USER_PROMPTS` | `0` | Opt-in: user prompt text |
| `OTEL_LOG_TOOL_DETAILS` | `0` | Opt-in: file paths, commands |
| `OTEL_LOG_TOOL_CONTENT` | `0` | Opt-in: full tool I/O |
| `OTEL_LOG_RAW_API_BODIES` | `0` | Opt-in: full API request/response |
