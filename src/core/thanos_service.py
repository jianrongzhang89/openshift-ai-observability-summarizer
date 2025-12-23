#!/usr/bin/env python3
"""
Core Thanos Query Service
Moved from metrics_api.py to separate business logic
"""


def get_metric_key(promql: str) -> str:
    """
    Generate a unique key for a PromQL query
    """
    # Remove common prefixes and suffixes for cleaner keys
    key = promql.strip()
    
    # Remove common prefixes
    prefixes_to_remove = [
        "sum(", "avg(", "count(", "rate(", "histogram_quantile(0.95, ",
        "sum(rate(", "avg(rate(", "count(rate("
    ]
    
    for prefix in prefixes_to_remove:
        if key.startswith(prefix):
            key = key[len(prefix):]
            break
    
    # Remove common suffixes
    suffixes_to_remove = [
        ")", "))", ")))"
    ]
    
    for suffix in suffixes_to_remove:
        if key.endswith(suffix):
            key = key[:-len(suffix)]
            break
    
    # Clean up the key
    key = key.replace(" ", "_").replace("{", "_").replace("}", "_").replace('"', "")
    key = key.replace("namespace=", "ns_").replace("model_name=", "model_")
    key = key.replace("phase=", "phase_")
    
    # Remove multiple underscores
    while "__" in key:
        key = key.replace("__", "_")
    
    # Remove leading/trailing underscores
    key = key.strip("_")
    
    return key or "unknown_metric" 