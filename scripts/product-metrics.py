#!/usr/bin/env python3
"""Aggregate privacy-preserving Patchlog workflow evidence."""

from __future__ import annotations

import json
import math
import statistics
from collections import Counter
from datetime import datetime
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
EVIDENCE_DIR = ROOT / "docs" / "evidence"


def load_evidence() -> list[dict]:
    records: list[dict] = []
    for path in sorted(EVIDENCE_DIR.glob("*.json")):
        if path.name in {"metrics.json", "validation.example.json"}:
            continue
        with path.open(encoding="utf-8") as handle:
            record = json.load(handle)
        records.append(record)
    return records


def ratio(numerator: int, denominator: int) -> dict:
    percentage = 0.0 if denominator == 0 else round(100 * numerator / denominator, 1)
    return {
        "numerator": numerator,
        "denominator": denominator,
        "percentage": percentage,
    }


def time_to_first_plan(record: dict) -> int | None:
    explicit = record.get("time_to_first_successful_plan_seconds")
    if explicit is not None:
        return int(explicit)
    started = record.get("validation_started_at")
    completed = record.get("first_successful_plan_at")
    if not started or not completed:
        return None
    start_time = datetime.fromisoformat(str(started).replace("Z", "+00:00"))
    completed_time = datetime.fromisoformat(str(completed).replace("Z", "+00:00"))
    seconds = int((completed_time - start_time).total_seconds())
    if seconds < 0:
        raise ValueError("first successful plan precedes validation start")
    return seconds


def main() -> None:
    records = load_evidence()
    hosted = [
        record
        for record in records
        if record.get("validation_scope")
        in {"hosted-protected-mirror", "hosted-protected-repository"}
    ]
    durations = sorted(
        seconds
        for record in hosted
        if (seconds := time_to_first_plan(record)) is not None
    )
    successes = sum(
        record.get("plan_to_release_success") is True
        or record.get("release_verification") == "success"
        for record in hosted
    )
    rejections = Counter(
        rejection
        for record in records
        for rejection in record.get("preflight_rejections", [])
    )
    recovery_executions = sum(int(record.get("recovery_count", 0)) for record in hosted)
    releases_requiring_recovery = sum(
        int(record.get("recovery_count", 0)) > 0 for record in hosted
    )
    manual_git_free = sum(
        (
            record.get("plan_to_release_success") is True
            or record.get("release_verification") == "success"
        )
        and record.get("manual_git_intervention") is False
        for record in hosted
    )
    unrelated_repositories = {
        record.get("source_repository")
        for record in hosted
        if record.get("validation_scope") == "hosted-protected-repository"
        and record.get("repository_controller_relationship") == "unrelated-maintainer"
        and record.get("maintainer_authorization_record")
        and record.get("release_verification") == "success"
        and record.get("source_repository")
    }
    p90_index = max(0, math.ceil(0.9 * len(durations)) - 1) if durations else 0

    result = {
        "schema": "https://patchlog.dev/schemas/product-metrics/v1",
        "schema_version": 1,
        "evidence_class": "hosted-protected-workflows",
        "hosted_sample_size": len(hosted),
        "all_validation_attempts": len(records),
        "time_to_first_successful_plan_seconds": {
            "median": statistics.median(durations) if durations else None,
            "p90_nearest_rank": durations[p90_index] if durations else None,
        },
        "plan_to_release_success_rate": ratio(successes, len(hosted)),
        "preflight_rejection_reasons": dict(sorted(rejections.items())),
        "preflight_rejection_observation_sample_size": len(records),
        "recovery_rate": ratio(releases_requiring_recovery, len(hosted)),
        "recovery_execution_count": recovery_executions,
        "releases_without_manual_git_intervention": ratio(manual_git_free, successes),
        "unrelated_maintainer_validation": {
            "required_repositories": 3,
            "completed_repositories": len(unrelated_repositories),
            "launch_gate_satisfied": len(unrelated_repositories) >= 3,
        },
        "collection": {
            "telemetry_sent_by_patchlog": False,
            "individual_or_proxy_metrics_used_as_release_gates": False,
        },
    }
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
