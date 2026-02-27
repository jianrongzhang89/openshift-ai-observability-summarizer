# From Metrics to Meaning: An AI Observability Summarizer for SREs on OpenShift

**Turn Prometheus metrics (and optionally traces/logs) into evidence-backed, operator-ready summaries—without context switching.**

*Author: [Your Name/Team], Red Hat*

---

## TL;DR

- **Problem**: Modern AI platforms generate huge volumes of telemetry; on-call time is burned on tool-hopping and manual correlation.
- **Solution**: The **AI Observability Summarizer** is an OpenShift Console plugin + service that lets SREs ask questions in plain language and get back **structured answers with supporting evidence** (PromQL results, trace samples, log excerpts).
- **Why it’s trustworthy**: It uses a curated metric catalog, schema-validated outputs, and “show your work” evidence links—so operators can verify fast.

---

## The SRE problem: too many signals, not enough time

When an inference SLA breaks at 2AM, the workflow is familiar:

- Find the metric spike in Prometheus/Thanos
- Pivot to GPU/infra metrics
- Jump to Tempo traces to locate the slow hop
- Search Loki logs to confirm the failure mode
- Stitch it all into an incident update

The failure isn’t lack of data; it’s **time-to-understanding** under pressure.

---

## What the summarizer does (in one sentence)

It compresses multi-tool investigation into a single guided interaction: **ask a question → fetch the right signals → correlate them → return a structured summary with evidence and next actions**.

---

## What SREs get out of it

- **Faster triage**: “What changed?” and “what’s most likely broken?” becomes a first-class workflow, not a manual side quest.
- **Less PromQL memorization**: you can still get PromQL, but you don’t have to start there.
- **Incident communication**: export a report with the evidence already attached (no screenshot stitching).
- **A safer AI experience**: answers are constrained, validated, and anchored to source data.

---

## How it works (high level)

At query time, the summarizer:

1. **Classifies intent** (alerts triage vs performance regression vs capacity vs GPU issues)
2. **Selects signals** using a curated metrics catalog (to avoid token blowups and metric-name hallucinations)
3. **Fetches in parallel** from Prometheus/Thanos (and optionally Tempo/Loki)
4. **Correlates** signals (metrics ↔ traces ↔ logs) using shared Kubernetes context (namespace/pod/container labels; trace IDs when available)
5. **Synthesizes** a structured response (current state, likely cause, recommended actions, evidence)
6. **Validates** the response format; if the model fails the contract, it retries or falls back to raw data

If you want the deeper implementation details (catalog design, scoring, validation strategies), see the deep dive:
`docs/blog-from-metrics-to-meaning.md`.

---

## Trust & operational guardrails (what makes this usable on-call)

- **Evidence-first**: summaries must cite the underlying query results / trace samples / log excerpts.
- **Structured output contracts**: prompts are treated like APIs (schema-validated, versionable, testable).
- **Graceful degradation**: if the model/provider is unavailable, return raw results + an explanation—don’t fail silently.
- **Data locality controls**: use local models where telemetry can’t leave the cluster.
- **No write actions by default**: the tool accelerates investigation and communication; remediation remains explicit and human-controlled.

---

## Example questions SREs actually ask

- “Are there any alerts firing right now, and which are symptoms vs root cause?”
- “Why did P95 latency spike in the last 30 minutes?”
- “GPU utilization is low but request queue is high—what’s the bottleneck?”
- “What changed around the time errors started?”

---

## Try it yourself

The AI Observability Summarizer is open source and designed for OpenShift + OpenShift AI environments.

```bash
make install NAMESPACE=your-namespace
```

Then open the OpenShift Console and look for the **AI Observability** entry in the navigation.

**Repo**: [github.com/rh-ai-quickstart/openshift-ai-observability-summarizer](https://github.com/rh-ai-quickstart/openshift-ai-observability-summarizer)

---

## Limitations (and when not to use it)

- **Simple lookups**: if you already know the exact query, deterministic tooling is faster.
- **Correlation quality depends on telemetry hygiene**: missing labels and broken trace propagation reduce cross-signal accuracy.
- **AI doesn’t replace verification**: treat outputs as triage acceleration; confirm decisions against the evidence.

---

## Get involved

We welcome contributions from SRE teams running AI workloads:

- Metric catalog contributions (new categories, priority tuning)
- Prompt templates for investigation workflows
- Correlation patterns for your environments
- Console/plugin UX feedback

Start here: [github.com/rh-ai-quickstart/openshift-ai-observability-summarizer](https://github.com/rh-ai-quickstart/openshift-ai-observability-summarizer)

