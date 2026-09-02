#!/usr/bin/env python3
"""TheDobra analytics workers — forecast, anomaly, stats.

Jobs are pulled from Redis list `thedobra:ml:jobs` as JSON:
  {"kind":"forecast"|"anomaly"|"stats", "org_id":"...", "series":[...], "horizon":12}

This process is intentionally independent from the Go query path.
"""
from __future__ import annotations

import json
import math
import os
import statistics
import time
from typing import Any

try:
    import redis  # type: ignore
except ImportError:
    redis = None


def forecast(series: list[float], horizon: int = 12) -> dict[str, Any]:
    if len(series) < 3:
        return {"error": "insufficient history"}
    n = len(series)
    xs = list(range(n))
    xbar = sum(xs) / n
    ybar = sum(series) / n
    den = sum((x - xbar) ** 2 for x in xs) or 1.0
    slope = sum((x - xbar) * (y - ybar) for x, y in zip(xs, series)) / den
    intercept = ybar - slope * xbar
    resid = [y - (intercept + slope * x) for x, y in zip(xs, series)]
    sigma = statistics.pstdev(resid) if len(resid) > 1 else 0.0
    actual = series
    pred = [intercept + slope * (n + i) for i in range(horizon)]
    lo = [p - 1.96 * sigma for p in pred]
    hi = [p + 1.96 * sigma for p in pred]
    return {
        "actual": actual,
        "forecast": pred,
        "low": lo,
        "high": hi,
        "assumptions": [
            "Linear trend fitted by ordinary least squares",
            "95% interval assumes homoscedastic residuals",
        ],
    }


def anomalies(series: list[float], z: float = 3.0) -> dict[str, Any]:
    if len(series) < 8:
        return {"error": "insufficient history"}
    mu = statistics.mean(series)
    sd = statistics.pstdev(series) or 1e-9
    flags = []
    for i, v in enumerate(series):
        score = (v - mu) / sd
        if abs(score) >= z:
            flags.append({"index": i, "value": v, "z": score})
    return {"mean": mu, "stdev": sd, "anomalies": flags}


def stats(series: list[float]) -> dict[str, Any]:
    if not series:
        return {"error": "empty"}
    s = sorted(series)
    n = len(s)
    return {
        "count": n,
        "mean": statistics.mean(s),
        "stdev": statistics.pstdev(s) if n > 1 else 0,
        "min": s[0],
        "p50": s[n // 2],
        "p95": s[min(n - 1, math.floor(n * 0.95))],
        "max": s[-1],
    }


HANDLERS = {"forecast": forecast, "anomaly": anomalies, "stats": stats}


def handle(job: dict[str, Any]) -> dict[str, Any]:
    kind = job.get("kind", "stats")
    series = [float(x) for x in job.get("series", [])]
    if kind == "forecast":
        return forecast(series, int(job.get("horizon", 12)))
    if kind == "anomaly":
        return anomalies(series, float(job.get("z", 3)))
    return stats(series)


def main() -> None:
    addr = os.environ.get("REDIS_ADDR", "localhost:6379")
    host, _, port = addr.partition(":")
    if redis is None:
        print("redis package missing — running one-shot self test")
        print(json.dumps(forecast([10, 12, 13, 15, 18, 19, 22], 4), indent=2))
        return
    client = redis.Redis(host=host or "localhost", port=int(port or 6379), decode_responses=True)
    print("thedobra analytics worker listening on", addr)
    while True:
        item = client.blpop("thedobra:ml:jobs", timeout=5)
        if not item:
            continue
        _, raw = item
        job = json.loads(raw)
        result = handle(job)
        key = job.get("result_key") or f"thedobra:ml:result:{job.get('id', int(time.time()))}"
        client.setex(key, 3600, json.dumps(result))
        print("done", job.get("kind"), key)


if __name__ == "__main__":
    main()
