"""
Data analysis and statistical functions for metrics processing.

This module contains pure functions for analyzing metrics data,
detecting anomalies, computing health scores, and trend analysis.
All functions are framework-agnostic and operate on pandas DataFrames.
"""

import pandas as pd
from scipy.stats import linregress


def detect_anomalies(df: pd.DataFrame, label: str) -> str:
    """
    Detect anomalies in metrics data using statistical analysis.
    
    Args:
        df: DataFrame with 'value' column containing metric values
        label: Human-readable label for the metric
        
    Returns:
        String description of anomaly status (stable, spike, or unusually low)
    """
    if df.empty:
        return "No data"
    
    mean = df["value"].mean()
    std = df["value"].std()
    p90 = df["value"].quantile(0.9)
    latest_val = df["value"].iloc[-1]
    
    if latest_val > p90:
        return f"⚠️ {label} spike (latest={latest_val:.2f}, >90th pct)"
    elif latest_val < (mean - std):
        return f"⚠️ {label} unusually low (latest={latest_val:.2f}, mean={mean:.2f})"
    
    return f"{label} stable (latest={latest_val:.2f}, mean={mean:.2f})"


def describe_trend(df: pd.DataFrame) -> str:
    """
    Analyze the trend direction of metrics data using linear regression.
    
    Args:
        df: DataFrame with 'timestamp' and 'value' columns
        
    Returns:
        String describing trend: "increasing", "decreasing", "stable", "flat", or "not enough data"
    """
    if df.empty or len(df) < 2:
        return "not enough data"
    
    df = df.sort_values("timestamp")
    x = (df["timestamp"] - df["timestamp"].min()).dt.total_seconds()
    y = df["value"]
    
    if x.nunique() <= 1:
        return "flat"
    
    slope, *_ = linregress(x, y)
    
    if slope > 0.01:
        return "increasing"
    elif slope < -0.01:
        return "decreasing"
    
    return "stable"