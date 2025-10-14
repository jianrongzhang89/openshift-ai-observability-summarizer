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


# korrel8r_build_links tool removed per request


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

 
def korrel8r_list_goals(goals: List[str], start: Dict[str, Any]) -> List[Dict[str, Any]]:
    """List Korrel8r goals using explicit parameters.

    Args:
        goals: List of goal class names (see docs: https://korrel8r.github.io/korrel8r/#Goals)
        start: Start object (see docs: https://korrel8r.github.io/korrel8r/#Start)
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
        result = client.list_goals(goals=goals, start=start)
        return _resp(json.dumps(result))
    except Exception as e:
        logger.error("korrel8r_list_goals failed: %s", e)
        err = MCPException(
            message=f"Korrel8r list goals failed: {str(e)}",
            error_code=MCPErrorCode.RESOURCE_UNAVAILABLE,
            recovery_suggestion="Verify Korrel8r URL, token and service health.",
        )
        return err.to_mcp_response()


