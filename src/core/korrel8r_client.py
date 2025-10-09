"""Korrel8r REST client.

Provides a minimal, resilient client for calling Korrel8r from server code and MCP tools.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

import requests
from urllib.parse import urlparse

from .config import (
    KORREL8R_URL,
    KORREL8R_TIMEOUT_SECONDS,
    VERIFY_SSL,
)
from common.pylogger import get_python_logger
from .config import THANOS_TOKEN


logger = get_python_logger()


@dataclass
class TimeWindow:
    start: str  # ISO8601
    end: str    # ISO8601


class Korrel8rClient:
    def __init__(self, base_url: Optional[str] = None, timeout_seconds: Optional[int] = None) -> None:
        self.base_url: str = (base_url or KORREL8R_URL).rstrip("/")
        self.timeout_seconds: int = timeout_seconds or KORREL8R_TIMEOUT_SECONDS

    def _post(self, path: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        if not self.base_url:
            raise RuntimeError("Korrel8r base URL not configured")

        url = f"{self.base_url}{path}"
        headers: Dict[str, str] = {"Content-Type": "application/json"}
        # Forward bearer token so Korrel8r can impersonate to stores (Prometheus, etc.)
        if THANOS_TOKEN:
            headers["Authorization"] = f"Bearer {THANOS_TOKEN}"

        # Choose verify behavior: use service CA only for in-cluster svc endpoints
        verify_param: Any = self._choose_verify_param(url)

        try:
            response = requests.post(
                url,
                data=json.dumps(payload),
                headers=headers,
                verify=verify_param,
                timeout=self.timeout_seconds,
            )
            response.raise_for_status()
            return response.json()
        except requests.exceptions.Timeout as e:
            logger.warning("Korrel8r request timed out: %s", e)
            raise
        except requests.exceptions.RequestException as e:
            logger.error("Korrel8r request failed: %s", e)
            raise

    def _get(self, path: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        if not self.base_url:
            raise RuntimeError("Korrel8r base URL not configured")

        url = f"{self.base_url}{path}"
        headers: Dict[str, str] = {}
        if THANOS_TOKEN:
            headers["Authorization"] = f"Bearer {THANOS_TOKEN}"

        verify_param: Any = self._choose_verify_param(url)

        try:
            response = requests.get(
                url,
                params=params,
                headers=headers,
                verify=verify_param,
                timeout=self.timeout_seconds,
            )
            logger.info("Korrel8r url:%s, params: %s", url, params)
            logger.info("Korrel8r headers: %s", headers)
            logger.info("Korrel8r GET response: %s", response.json())

            response.raise_for_status()
            return response.json()
        except requests.exceptions.Timeout as e:
            logger.warning("Korrel8r GET timed out: %s", e)
            raise
        except requests.exceptions.RequestException as e:
            logger.error("Korrel8r GET failed: %s", e)
            raise

    def _choose_verify_param(self, full_url: str) -> Any:
        """Use service CA bundle for in-cluster service URLs; otherwise system CAs.

        This avoids overriding public CA trust with the injected service CA bundle
        when calling external routes.
        """
        try:
            host = urlparse(full_url).hostname or ""
            if ".svc" in host or "cluster.local" in host:
                return VERIFY_SSL
            return True
        except Exception:
            return True

    def health(self) -> Dict[str, Any]:
        """Check Korrel8r liveness/readiness."""
        return self._get("/healthz")

    def find_related(
        self,
        *,
        start: Dict[str, Any],
        targets: Optional[List[str]] = None,
        time_window: Optional[TimeWindow] = None,
        limit: Optional[int] = None,
        depth: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Call Korrel8r 'graphs/neighbours' endpoint with path fallback.

        Arguments mirror the proposal; server must map to Korrel8r's actual API.
        """
        payload: Dict[str, Any] = {"start": start}
        if targets:
            payload["targets"] = targets
        if time_window:
            payload["timeWindow"] = {"start": time_window.start, "end": time_window.end}
        if limit is not None:
            payload["limit"] = limit
        if depth is not None:
            payload["depth"] = depth

        return self._post("/api/v1alpha1/graphs/neighbours", payload)

    def query_objects(self, query: str) -> Any:
        """Execute a Korrel8r domain query and return raw objects.

        GET /api/v1alpha1/objects?query=domain:class:selector
        """
        if not query or not isinstance(query, str):
            raise ValueError("query must be a non-empty string")
        return self._get("/api/v1alpha1/objects", params={"query": query})


