from typing import Any, Dict, List, Optional
import json
import logging

from common.pylogger import get_python_logger
from core.correlation_service import CorrelationService
from core.korrel8r_client import Korrel8rClient
from core.config import (
    KORREL8R_ENABLED,
    CONSOLE_BASE_URL,
    GRAFANA_BASE_URL,
    TEMPO_BASE_URL,
    TEMPO_DATASOURCE_UID,
    LOKI_DATASOURCE_UID,
)
from .observability_vllm_tools import _resp
from mcp_server.exceptions import MCPException, MCPErrorCode


logger = get_python_logger()


def korrel8r_health() -> List[Dict[str, Any]]:
    """Probe Korrel8r health via MCP tool."""
    if not KORREL8R_ENABLED:
        return _resp(json.dumps({"status": "down", "reason": "KORREL8R_DISABLED"}))

    client = Korrel8rClient()
    try:
        result = client.health()
        status = "ok"
        try:
            if not result:
                status = "down"
        except Exception:
            status = "degraded"
        return _resp(json.dumps({"status": status, "raw": result}))
    except Exception as e:
        return _resp(json.dumps({"status": "down", "error": str(e)}))


def korrel8r_find_related(
    start: Dict[str, Any],
    targets: Optional[List[str]] = None,
    time_window_start: Optional[str] = None,
    time_window_end: Optional[str] = None,
    limit: Optional[int] = None,
    depth: Optional[int] = None,
) -> List[Dict[str, Any]]:
    """Find correlated signals for a starting object using Korrel8r."""
    if not KORREL8R_ENABLED:
        err = MCPException(
            message="Korrel8r integration is disabled",
            error_code=MCPErrorCode.FEATURE_DISABLED,
            recovery_suggestion="Enable KORREL8R_ENABLED to use this tool.",
        )
        return err.to_mcp_response()

    try:
        service = CorrelationService()
        alert_labels = start.get("labels", {}) if isinstance(start, dict) else {}
        alert_ts = start.get("timestamp") if isinstance(start, dict) else None
        normalized = service.correlate_alert(
            alert_labels=alert_labels,
            alert_timestamp_iso=alert_ts,
            window_start_iso=time_window_start,
            window_end_iso=time_window_end,
            targets=targets,
            limit=limit,
            depth=depth,
        )
        return _resp(json.dumps(normalized))
    except Exception as e:
        logger.error("korrel8r_find_related failed: %s", e)
        err = MCPException(
            message=f"Korrel8r correlation failed: {str(e)}",
            error_code=MCPErrorCode.INTERNAL_ERROR,
            recovery_suggestion="Try again later or check Korrel8r service.",
        )
        return err.to_mcp_response()


def korrel8r_build_links(
    entities_json: str,
    window_start: Optional[str] = None,
    window_end: Optional[str] = None,
) -> List[Dict[str, Any]]:
    """Populate link fields for correlated entities using configured bases.

    entities_json: JSON string of entities (heterogeneous list) to avoid overly
                   complex MCP param typing.
    """
    try:
        entities = json.loads(entities_json)
        if not isinstance(entities, list):
            raise ValueError("entities must be a JSON array")
    except Exception as e:
        err = MCPException(
            message=f"Invalid entities_json: {str(e)}",
            error_code=MCPErrorCode.INVALID_INPUT,
            recovery_suggestion="Provide entities_json as a JSON array.",
        )
        return err.to_mcp_response()

    # Build links best-effort
    for ent in entities:
        try:
            if not isinstance(ent, dict):
                continue
            # K8s objects
            if "kind" in ent and "name" in ent:
                # Simple plural map for common kinds
                plural_map = {
                    "Pod": "pods",
                    "Deployment": "deployments",
                    "StatefulSet": "statefulsets",
                    "DaemonSet": "daemonsets",
                    "ReplicaSet": "replicasets",
                    "Service": "services",
                    "Namespace": "namespaces",
                    "Node": "nodes",
                }
                kind = ent.get("kind")
                name = ent.get("name")
                ns = ent.get("namespace")
                if kind in ("Node", "Namespace"):
                    if CONSOLE_BASE_URL:
                        ent["link"] = f"{CONSOLE_BASE_URL.rstrip('/')}/k8s/cluster/{plural_map.get(kind, kind.lower()+'s')}/{name}"
                else:
                    if CONSOLE_BASE_URL and ns:
                        ent["link"] = f"{CONSOLE_BASE_URL.rstrip('/')}/k8s/ns/{ns}/{plural_map.get(kind, kind.lower()+'s')}/{name}"

            # Tempo traces
            if "traceId" in ent and TEMPO_BASE_URL:
                ent["link"] = f"{TEMPO_BASE_URL.rstrip('/')}/trace/{ent.get('traceId')}"

            # Loki logs (build Grafana Explore link when possible)
            if ent.get("type") == "loki/log" and GRAFANA_BASE_URL and LOKI_DATASOURCE_UID and ent.get("query"):
                q = ent.get("query", "").replace("'", "\\'")
                start_iso = window_start or ""
                end_iso = window_end or ""
                left = (
                    f"(datasource:'{LOKI_DATASOURCE_UID}',"
                    f"queries:!((expr:'{q}')),"
                    f"range:(from:'{start_iso}',to:'{end_iso}'))"
                )
                from urllib.parse import quote
                ent["link"] = f"{GRAFANA_BASE_URL.rstrip('/')}/explore?left={quote(left, safe='')}"

        except Exception:
            # Best-effort; skip on errors
            continue

    return _resp(json.dumps({"entities": entities}))


def korrel8r_query_objects(query: str) -> List[Dict[str, Any]]:
    """Execute a Korrel8r domain query and return objects.

    Example query strings (see docs [korrel8r#_query_8](https://korrel8r.github.io/korrel8r/#_query_8)):
      - alert:alert:{"alertname":"PodDisruptionBudgetAtLimit"}
      - k8s:Pod:{"namespace", "llm-serving", "name":"vllm-inference-*"}
      - loki:log:{"kubernetes.namespace_name":"llm-serving","kubernetes.pod_name":"p-abc"}
      - trace:span:{".k8s.namespace.name":"llm-serving"}
    """
    if not KORREL8R_ENABLED:
        err = MCPException(
            message="Korrel8r integration is disabled",
            error_code=MCPErrorCode.FEATURE_DISABLED,
            recovery_suggestion="Enable KORREL8R_ENABLED to use this tool.",
        )
        return err.to_mcp_response()

    try:
        client = Korrel8rClient()
        result = client.query_objects(query)
        return _resp(json.dumps(result))
    except Exception as e:
        logger.error("korrel8r_query_objects failed: %s", e)
        err = MCPException(
            message=f"Korrel8r query failed: {str(e)}",
            error_code=MCPErrorCode.INTERNAL_ERROR,
            recovery_suggestion="Check query syntax and Korrel8r service availability.",
        )
        return err.to_mcp_response()


