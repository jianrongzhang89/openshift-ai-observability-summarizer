## Proposal: Enhance PromQL selection with semantic relevance

### Background
- **Current approach**: `select_queries_directly` in `src/core/promql_service.py` uses keyword/pattern matching to map NL questions to PromQL.
- **Pros**: deterministic, fast, test-friendly.
- **Cons**: brittle to phrasing, limited coverage, does not leverage the live metric catalog and descriptions.

### Goals
- **Improve recall and relevance** for questions that don’t match the existing keyword branches.
- **Preserve determinism and current behavior** when patterns are matched (tests keep passing).
- **Avoid new heavy dependencies** and reuse existing code paths.

### High‑level design
- **Keep keyword/pattern matching as Pass 1**: if a specific pattern is detected, return those queries (current behavior).
- **Add a semantic-ish fallback as Pass 2** when either:
  - No pattern is detected, or
  - 0 queries were generated.

Pass 2 pipeline:
- **Discover**: `discover_available_metrics_from_thanos(namespace, model_name, is_fleet_wide)` to list actual metric names.
- **Select**: `intelligent_metric_selection(question, available_metrics)` to filter by relevance (uses names/descriptions and existing categorization functions).
- **Generate PromQL**: `generate_promql_from_discovered_metric(metric, namespace, model_name, rate_interval, is_fleet_wide)` to produce final queries.
- **Post-process**: de-duplicate, cap result count (e.g., top 3–6).

### Semantic scoring (lightweight)
- Augment `intelligent_metric_selection` with a tiny relevance score (still pure-Python):
  - **Keyword features**: existing checks (GPU, vLLM, K8s, alerts, etc.).
  - **Token overlap**: Jaccard overlap between question tokens and metric name/description tokens.
  - **Priority boosts**: family/metric priority (already present for latency).
  - Optional future: embedding similarity (only if a provider is available); otherwise skip.

### Fleet‑wide vs namespace
- Reuse existing label helpers so fleet‑wide queries omit namespace labels, namespace‑specific queries include them.

### Time ranges
- Continue to parse time hints with `extract_time_period_from_question` and pass computed intervals into generated queries.

### Caching and performance
- `discover_available_metrics_from_thanos` can be expensive. Add a short‑lived cache (e.g., 60–120 seconds, keyed by namespace/fleet flag) to avoid repeated catalog fetches during a session.

### Failure modes and safe fallback
- If discovery fails or returns no metrics, retain current default queries (unchanged behavior).

### Testing strategy
- Unit tests:
  - Ensure Pass 1 (pattern matching) still returns the same queries.
  - Ensure Pass 2 returns sensible queries for questions without explicit patterns.
  - Verify label scoping and rate intervals are applied correctly.
- Integration tests:
  - Mock Thanos metric list and assert semantic fallback generates relevant PromQL.

### Rollout plan
- Behind a small feature flag (e.g., env var `ENABLE_SEMANTIC_PROMQL=1`).
- Default off initially; enable in CI or local dev to iterate safely.

### Summary
- This proposal adds a semantic fallback that reuses existing code and data, improving coverage without changing the successful fast path. It remains deterministic, testable, and dependency‑light.


