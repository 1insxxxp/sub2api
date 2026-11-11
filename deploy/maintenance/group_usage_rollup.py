#!/usr/bin/env python3
"""Bounded daily group rollups with a durable cursor and per-date SQL audits."""

import argparse
import fcntl
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
from datetime import date, datetime, timedelta
from zoneinfo import ZoneInfo


SHANGHAI = ZoneInfo("Asia/Shanghai")
SQL_PATH = Path(__file__).resolve().with_suffix(".sql")
PSQL_COMMAND = [
    "docker", "exec", "-i", "sub2api-postgres", "sh", "-c",
    'exec psql -X -qAt -v ON_ERROR_STOP=1 -v rollup_date="$1" '
    '-U "$POSTGRES_USER" -d "$POSTGRES_DB"', "rollup",
]
ONE_DAY = timedelta(days=1)


def parse_date(value):
    try:
        parsed = date.fromisoformat(value)
    except (TypeError, ValueError):
        raise ValueError("expected a calendar date in YYYY-MM-DD format") from None
    if parsed.isoformat() != value:
        raise ValueError("expected a calendar date in YYYY-MM-DD format")
    return parsed


def plan_dates(next_date, today):
    if next_date > today:
        raise ValueError("next_date must not be later than today in Asia/Shanghai")
    backlog_count = min(7, (today - next_date).days)
    backlog = {next_date + offset * ONE_DAY for offset in range(backlog_count)}
    recent = {today - ONE_DAY, today - 2 * ONE_DAY}
    return sorted(backlog | recent)


def load_next_date(state_file, start_date):
    try:
        state = json.loads(state_file.read_text(encoding="utf-8"))
    except FileNotFoundError:
        if start_date is None:
            raise ValueError("missing state requires --start-date YYYY-MM-DD to bootstrap") from None
        return start_date
    if not isinstance(state, dict):
        raise ValueError("invalid state, expected a JSON object with next_date")
    return parse_date(state.get("next_date"))


def sync_directory(path):
    descriptor = os.open(path, os.O_RDONLY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def write_atomic(destination, content):
    descriptor, temporary = tempfile.mkstemp(prefix=f".{destination.name}.", dir=destination.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, destination)
        sync_directory(destination.parent)
    finally:
        Path(temporary).unlink(missing_ok=True)


def run_day(day):
    try:
        result = subprocess.run(
            [*PSQL_COMMAND, day.isoformat()],
            input=SQL_PATH.read_text(encoding="utf-8"),
            text=True, encoding="utf-8", capture_output=True, check=True, timeout=45,
        )
    except subprocess.SubprocessError as error:
        raise RuntimeError(f"rollup failed for {day} ({type(error).__name__})") from None
    if len(result.stdout.splitlines()) != 1:
        raise ValueError(f"expected one SQL JSON audit line for {day}")
    audit = json.loads(result.stdout)
    required = {"source_rows", "group_count", "actual_cost", "commission_excluded_cost", "before", "after"}
    if not isinstance(audit, dict) or audit.get("day") != day.isoformat() or not required <= audit.keys():
        raise ValueError(f"invalid SQL audit for {day}")
    return result.stdout


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--state-dir", type=Path, default=Path("/var/lib/sub2api-group-usage-rollup"))
    parser.add_argument("--start-date", type=parse_date, help="bootstrap date, used only when state is missing")
    parser.add_argument("--dry-run", action="store_true", help="print planned dates without running SQL")
    args = parser.parse_args(argv)
    try:
        args.state_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
        sync_directory(args.state_dir.parent)
        with (args.state_dir / "runner.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            state_file = args.state_dir / "state.json"
            next_date = load_next_date(state_file, args.start_date)
            days = plan_dates(next_date, datetime.now(SHANGHAI).date())
            if args.dry_run:
                for day in days:
                    print(day.isoformat())
                return 0
            runs = args.state_dir / "runs"
            runs.mkdir(mode=0o700, exist_ok=True)
            sync_directory(args.state_dir)
            for day in days:
                raw_audit = run_day(day)
                write_atomic(runs / f"{day}.json", raw_audit)
                # Recent recomputes must never skip unfinished backlog.
                if day == next_date:
                    next_date += ONE_DAY
                write_atomic(state_file, json.dumps({"next_date": next_date.isoformat()}) + "\n")
                summary = json.loads(raw_audit, parse_float=str)
                print(f"{day} source_rows={summary['source_rows']} group_count={summary['group_count']} "
                      f"gross={summary['actual_cost']} excluded={summary['commission_excluded_cost']}", flush=True)
        return 0
    except BlockingIOError:
        print("rollup runner lock is already held", file=sys.stderr)
    except (OSError, ValueError, RuntimeError) as error:
        print(f"rollup error — {error}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
