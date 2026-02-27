# From Metrics to Meaning: Designing an AI Observability Summarizer

**How we built an LLM-powered layer that turns thousands of Prometheus metrics into plain-English insights**

*Author: [Your Name/Team], Red Hat*

---

## The Dashboard Fatigue Problem

Picture this: Your model latency just spiked. You open Prometheus, stare at 30+ dashboard panels, manually correlate CPU, memory, GPU utilization, and request queues. You write custom PromQL queries, screenshot the relevant charts, paste values into Slack, and try to explain what's happening to your team.

**This happens every day, for every incident, every performance regression, every capacity review.**

We built the AI Observability Summarizer to turn this multi-hour investigation into a 60-second question-and-answer conversation.

---

## Architecture: From Signals to Insights

At its core, the system is a **metrics-aware AI layer** that sits between your observability stack and your engineering team. Here's the high-level flow:

```
Prometheus/Thanos → Curated Metrics Catalog → LLM → Plain-English Summary
         ↓                                              ↑
    GPU Metrics (DCGM)                            Report Export
         ↓                                              ↑
    Tempo Traces ────────────────────────────────────→ Optional Context
         ↓
    Loki Logs
```

### The Three Critical Layers

**1. Signal Selection (The Catalog)**

We started with ~50,000 available Prometheus metrics. The first challenge: **which metrics actually matter?**

Our solution: a **curated, dynamically validated metrics catalog** (~1,800 metrics) organized by:
- **Categories**: OpenShift fleet, GPU/accelerators, vLLM model serving, networking
- **Priority levels**: High (always show), Medium (context-dependent), Low (edge cases)
- **Runtime discovery**: GPU metrics are discovered at startup for vendor-agnostic support (NVIDIA, AMD, Intel)

```python
# Example from metrics_catalog.py
@dataclass
class MetricInfo:
    name: str
    category_id: str
    priority: str  # High, Medium, Low
    description: str
```

This catalog-first approach means:
- ✅ **No hallucinated metric names** - every query is validated against live Prometheus
- ✅ **Semantic search** - rank metrics by relevance to user questions
- ✅ **Noise reduction** - only surface metrics that matter for the context

**2. Prompt Engineering (The Brain)**

Once we have the right metrics, we need to ask the LLM the right question. Here's our prompt structure from `llm_summary_service.py`:

```python
prompt = f"""You are a senior Site Reliability Engineer (SRE) analyzing metrics for namespace: {namespace}.

Question: {question}

Metrics Data:
{context}

Provide ONLY a structured summary in this exact format:
Current value: [value]
Meaning: [brief explanation]
Immediate concern: [None or specific concern]
Key insight: [one key observation]
"""
```

**Why this works:**
- **Role definition** - "senior SRE" → generates operationally-focused insights
- **Structured output** - enforces consistent, parseable format
- **Constraints** - "ONLY" and "exact format" reduce hallucination and fluff
- **Deterministic temperature (0)** - same inputs = same outputs

We also clean LLM responses aggressively (see `_clean_llm_summary_string()`) to remove meta-commentary, markdown artifacts, and formatting instructions that models sometimes include.

**3. Multi-Provider LLM Support**

The system supports:
- **Local models** (vLLM, Llama Stack) - deployed by default
- **External APIs** - OpenAI, Google Gemini, Anthropic Claude, Meta Llama

This flexibility matters because:
- Internal teams might prefer on-prem models (data locality, cost)
- External teams might want cutting-edge hosted models (speed, quality)
- Different models excel at different tasks (chat vs analysis vs report generation)

---

## Why Summarization Beats Dashboards

Dashboards are great for **monitoring**. But for **investigating** and **communicating**, they fall short:

| Dashboards | AI Summarizer |
|-----------|--------------|
| Show you what happened | **Explain why it might matter** |
| Require you to correlate signals | **Automatically correlates across metrics** |
| Static views | **Adapts to your question** |
| Copy/paste screenshots | **Export HTML/PDF reports** |
| "CPU is at 80%" | **"CPU usage is elevated (80% vs 45% baseline). Likely cause: recent model deployment increased batch sizes."** |

**Real example from the codebase:**

```python
# Instead of showing raw alert counts, we analyze them:
if alert_infos:
    alert_analysis = generate_alert_analysis_with_llm(alert_infos, namespace)
    return f"🚨 **TOTAL OF {len(alert_infos)} ALERT(S)**\n\n{alert_analysis}"
```

The LLM doesn't just count alerts - it groups them, identifies patterns, and suggests root causes.

---

## Design Decisions That Made It Work

### 1. **Catalog Validation Over Dynamic Discovery**

Early versions dynamically queried all available metrics. This was slow and unreliable. The catalog approach:
- Pre-validates metrics at build time
- Runtime validation checks catalog against live Prometheus
- Graceful fallback to dynamic discovery if catalog is stale

### 2. **Priority-Based Filtering**

Not all metrics are equally important. When a user asks about GPU health, we show:
- **High priority**: `gpu_utilization`, `gpu_temperature`, `gpu_memory_used`
- **Medium priority**: `gpu_power_usage`, `gpu_ecc_errors`
- **Low priority**: `gpu_fan_speed`, `gpu_clock_speed`

This reduces LLM input size (cheaper, faster) and improves answer quality (signal vs noise).

### 3. **Semantic Metric Search**

Users don't speak Prometheus. They ask "why is my model slow?" not "show me `vllm_request_processing_time_p95_seconds`".

Our search function (from `chat_with_prometheus.py`):
```python
def rank_metrics_by_relevance(pattern: str, all_metrics: List[str]) -> List[str]:
    """Rank metrics by semantic relevance to user question."""
    # Scores metrics based on keyword matching, category affinity,
    # and priority level
```

### 4. **Response Validation and Error Handling**

LLMs fail in creative ways. We built:
- **Response validators** - check for expected structure before returning to user
- **Error wrapping** - convert HTTP 429/401/500 into human-friendly messages
- **Retry logic** - with exponential backoff for transient failures

---

## What We Learned

**1. Curated data > raw API access**

LLMs are great at reasoning, but terrible at finding needles in haystacks. Give them pre-filtered, high-signal data.

**2. Structured prompts > open-ended**

"Explain this metric" produces vague essays. "Current value, meaning, concern, insight" produces actionable summaries.

**3. Context matters**

Same metrics mean different things in different namespaces. A 40% GPU utilization might be:
- ✅ Good - for a batch job that's supposed to run at night
- ⚠️ Concerning - for a real-time inference endpoint

We pass namespace context into every prompt.

**4. Users need escape hatches**

When AI summaries aren't enough, users can:
- Click through to raw Prometheus queries
- Export detailed reports with all supporting data
- Switch to manual dashboard analysis

---

## Try It Yourself

The AI Observability Summarizer is open source and designed for OpenShift + OpenShift AI environments.

**Quick start:**
```bash
make install NAMESPACE=your-namespace
```

Access it via the OpenShift Console → **AI Observability** menu.

**Repo**: [github.com/rh-ai-quickstart/openshift-ai-observability-summarizer](https://github.com/rh-ai-quickstart/openshift-ai-observability-summarizer)

---

## What's Next

We're actively working on:
- **Automated incident reports** - generate timeline + root cause analysis from alerts
- **Trend detection** - proactive warnings for degrading performance
- **Cost analysis** - correlate resource usage with billing data
- **Custom metrics catalogs** - support for non-OpenShift environments

---

## Closing Thoughts

Observability isn't about collecting more metrics - it's about understanding what they mean, faster.

By combining **curated metric selection**, **structured prompting**, and **flexible LLM backends**, we turned Prometheus from a queryable database into a conversational analysis partner.

The architecture is simple: filter signal from noise, ask focused questions, validate responses, and get out of the user's way.

If you're building similar systems, our key lessons:
1. Invest in data curation before LLM integration
2. Design prompts like APIs - structured, versioned, testable
3. Support multiple LLM backends from day one
4. Users trust summaries more when they can verify the underlying data

Questions? Feedback? Open an issue or ping us on the Red Hat AI community.

---

**Tags:** #OpenShift #AI #Observability #LLM #Prometheus #SRE
