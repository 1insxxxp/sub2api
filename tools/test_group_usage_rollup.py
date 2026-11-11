#!/usr/bin/env python3
"""Small, standard-library-only tests for the operational rollup runner."""

import contextlib
import fcntl
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import unittest
from datetime import date, datetime, timedelta, timezone
from unittest import mock
from zoneinfo import ZoneInfo


RUNNER_PATH = Path(__file__).resolve().parents[1] / "deploy/maintenance/group_usage_rollup.py"
SPEC = importlib.util.spec_from_file_location("group_usage_rollup", RUNNER_PATH)
rollup = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(rollup)
TODAY = date(2026, 11, 9)
AUDIT_TEMPLATE = (
    '{"day": "%s", "source_rows": 3, "group_count": 1, '
    '"actual_cost": 1234567890.123456789012345678, '
    '"commission_excluded_cost": 0.000000000000000001, '
    '"before": [], "after": [{"actual_cost": 1234567890.123456789012345678}]}\n'
)


def audit_text(day):
    return AUDIT_TEMPLATE % day.isoformat()


class PlanningTests(unittest.TestCase):
    def test_backlog_is_bounded_and_recent_dates_are_sorted_and_unique(self):
        start = date(2026, 10, 20)
        self.assertEqual(
            rollup.plan_dates(start, TODAY),
            [start + timedelta(days=offset) for offset in range(7)]
            + [date(2026, 11, 7), date(2026, 11, 8)],
        )
        self.assertEqual(
            rollup.plan_dates(date(2026, 11, 5), TODAY),
            [date(2026, 11, day) for day in range(5, 9)],
        )

    def test_caught_up_runner_recomputes_two_closed_dates(self):
        self.assertEqual(
            rollup.plan_dates(TODAY, TODAY),
            [date(2026, 11, 7), date(2026, 11, 8)],
        )

    def test_plans_never_include_open_or_future_dates(self):
        for age in range(32):
            days = rollup.plan_dates(TODAY - timedelta(days=age), TODAY)
            self.assertLessEqual(len(days), 9)
            self.assertEqual(days, sorted(set(days)))
            self.assertTrue(all(day < TODAY for day in days))

    def test_future_cursor_is_rejected(self):
        with self.assertRaises(ValueError):
            rollup.plan_dates(TODAY + timedelta(days=1), TODAY)

    def test_only_canonical_calendar_dates_are_accepted(self):
        self.assertEqual(rollup.parse_date("2024-02-29"), date(2024, 2, 29))
        for value in ("2026-02-29", "2026-13-01", "20261107", "2026-W45-6",
                      "2026-11-7", "2026-11-07T00:00:00", "", None, 123, True):
            with self.subTest(value=value), self.assertRaises(ValueError):
                rollup.parse_date(value)


class RunnerTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.state_dir = Path(temporary.name) / "state"
        self.state_file = self.state_dir / "state.json"
        self.clock = self.enterContext(mock.patch.object(rollup, "datetime", wraps=datetime))
        self.clock.now.return_value = datetime(2026, 11, 9, 12, tzinfo=ZoneInfo("Asia/Shanghai"))
        self.database = self.enterContext(mock.patch.object(rollup, "run_day", side_effect=audit_text))

    def write_state(self, next_date):
        self.state_dir.mkdir(parents=True, exist_ok=True)
        self.state_file.write_text(json.dumps({"next_date": next_date}), encoding="utf-8")

    def cursor(self):
        return json.loads(self.state_file.read_text(encoding="utf-8"))["next_date"]

    def invoke(self, *arguments):
        output, errors = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
            result = rollup.main(["--state-dir", str(self.state_dir), *arguments])
        return result, output.getvalue(), errors.getvalue()

    def test_planning_switches_at_shanghai_midnight(self):
        self.write_state("2026-11-08")
        for instant, expected in (
            (datetime(2026, 11, 8, 15, 59, 59, tzinfo=timezone.utc), ["2026-11-06", "2026-11-07"]),
            (datetime(2026, 11, 8, 16, 0, tzinfo=timezone.utc), ["2026-11-07", "2026-11-08"]),
        ):
            self.clock.now.side_effect = lambda zone, instant=instant: instant.astimezone(zone)
            result, output, errors = self.invoke("--dry-run")
            self.assertEqual((result, errors), (0, ""))
            self.assertEqual(output.splitlines(), expected)
        self.assertEqual(self.cursor(), "2026-11-08")
        self.database.assert_not_called()

    def test_missing_state_requires_explicit_bootstrap(self):
        for arguments in ((), ("--dry-run",)):
            result, _, errors = self.invoke(*arguments)
            self.assertEqual(result, 1)
            self.assertIn("--start-date", errors)
        self.assertFalse(self.state_file.exists())
        self.database.assert_not_called()

    def test_bootstrap_dry_run_does_not_create_cursor_or_audits(self):
        result, output, errors = self.invoke("--start-date", "2026-11-07", "--dry-run")
        self.assertEqual((result, errors), (0, ""))
        self.assertEqual(output.splitlines(), ["2026-11-07", "2026-11-08"])
        self.assertFalse(self.state_file.exists())
        self.assertFalse((self.state_dir / "runs").exists())
        self.database.assert_not_called()

    def test_bootstrap_success_persists_next_date_and_exact_audits(self):
        result, output, errors = self.invoke("--start-date", "2026-11-07")
        self.assertEqual((result, errors), (0, ""))
        self.assertEqual(self.cursor(), "2026-11-09")
        for day in (date(2026, 11, 7), date(2026, 11, 8)):
            self.assertEqual((self.state_dir / "runs" / f"{day}.json").read_text(), audit_text(day))
        self.assertIn("1234567890.123456789012345678", output)
        self.assertNotIn('"before"', output)
        self.assertNotIn('"after"', output)

    def test_bootstrap_directory_entry_is_durable_before_sql(self):
        synced_directories = []
        parent = self.state_dir.parent.stat()
        real_fsync = os.fsync

        def record_sync(descriptor):
            metadata = os.fstat(descriptor)
            synced_directories.append((metadata.st_dev, metadata.st_ino))
            real_fsync(descriptor)

        def check_directory_then_run(day):
            self.assertIn((parent.st_dev, parent.st_ino), synced_directories)
            return audit_text(day)

        self.database.side_effect = check_directory_then_run
        with mock.patch.object(rollup.os, "fsync", side_effect=record_sync):
            self.assertEqual(self.invoke("--start-date", "2026-11-07")[0], 0)

    def test_bootstrap_date_is_ignored_once_state_exists(self):
        self.write_state("2026-11-07")
        result, _, errors = self.invoke("--start-date", "2026-10-01")
        self.assertEqual((result, errors), (0, ""))
        self.assertEqual([call.args[0] for call in self.database.call_args_list],
                         [date(2026, 11, 7), date(2026, 11, 8)])

    def test_bounded_catchup_never_jumps_over_unprocessed_backlog(self):
        self.write_state("2026-10-20")
        for expected_cursor, expected_count in (("2026-10-27", 9), ("2026-11-03", 9), ("2026-11-09", 6)):
            self.database.reset_mock()
            result, _, errors = self.invoke()
            self.assertEqual((result, errors), (0, ""))
            self.assertEqual(self.cursor(), expected_cursor)
            self.assertEqual(self.database.call_count, expected_count)
        self.assertEqual(len(list((self.state_dir / "runs").glob("*.json"))), 20)

    def test_sql_failure_preserves_first_unfinished_day_and_can_resume(self):
        self.write_state("2026-11-01")

        def fail_third(day):
            if day == date(2026, 11, 3):
                raise RuntimeError("SQL failed")
            return audit_text(day)

        self.database.side_effect = fail_third
        result, _, _ = self.invoke()
        self.assertEqual(result, 1)
        self.assertEqual(self.cursor(), "2026-11-03")
        self.assertEqual(sorted(path.name for path in (self.state_dir / "runs").glob("*.json")),
                         ["2026-11-01.json", "2026-11-02.json"])
        self.database.reset_mock(side_effect=True)
        self.database.side_effect = audit_text
        self.assertEqual(self.invoke()[0], 0)
        self.assertEqual(self.database.call_args_list[0].args[0], date(2026, 11, 3))
        self.assertEqual(self.cursor(), "2026-11-09")

    def test_failed_bootstrap_does_not_persist_a_cursor(self):
        self.database.side_effect = RuntimeError("SQL failed")
        self.assertEqual(self.invoke("--start-date", "2026-11-07")[0], 1)
        self.assertFalse(self.state_file.exists())
        self.assertEqual(list((self.state_dir / "runs").glob("*.json")), [])

    def test_recent_recomputes_replace_audits_and_keep_cursor(self):
        self.write_state("2026-11-09")
        self.assertEqual(self.invoke()[0], 0)
        self.database.side_effect = lambda day: audit_text(day).replace('"source_rows": 3', '"source_rows": 4')
        self.assertEqual(self.invoke()[0], 0)
        self.assertEqual(self.cursor(), "2026-11-09")
        self.assertEqual(self.database.call_count, 4)
        self.assertEqual(sorted(path.name for path in (self.state_dir / "runs").iterdir()),
                         ["2026-11-07.json", "2026-11-08.json"])
        self.assertIn('"source_rows": 4', (self.state_dir / "runs/2026-11-07.json").read_text())

    def test_corrupt_state_is_never_replaced_by_bootstrap(self):
        self.state_dir.mkdir()
        for content in ("broken", "[]", "{}", '{"next_date":null}',
                        '{"next_date":123}', '{"next_date":"2026-02-29"}',
                        '{"next_date":"20261107"}', '{"next_date":"2026-11-10"}'):
            for arguments in ((), ("--start-date", "2026-11-07")):
                with self.subTest(content=content, arguments=arguments):
                    self.state_file.write_text(content)
                    self.assertEqual(self.invoke(*arguments)[0], 1)
                    self.assertEqual(self.state_file.read_text(), content)
        self.database.assert_not_called()

    def test_future_bootstrap_is_rejected(self):
        self.assertEqual(self.invoke("--start-date", "2026-11-10")[0], 1)
        self.database.assert_not_called()

    def test_invalid_cli_date_is_rejected(self):
        with self.assertRaises(SystemExit) as error:
            self.invoke("--start-date", "2026-02-29")
        self.assertEqual(error.exception.code, 2)
        self.database.assert_not_called()

    def test_nonblocking_lock_rejects_second_runner(self):
        self.write_state("2026-11-07")
        with (self.state_dir / "runner.lock").open("a") as lock:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            result, _, errors = self.invoke()
        self.assertEqual(result, 1)
        self.assertIn("lock", errors)
        self.assertEqual(self.cursor(), "2026-11-07")
        self.database.assert_not_called()

    def test_audit_fsync_failure_preserves_cursor_and_previous_audit(self):
        self.write_state("2026-11-07")
        runs = self.state_dir / "runs"
        runs.mkdir()
        audit = runs / "2026-11-07.json"
        audit.write_text("previous audit\n")
        real_fsync = os.fsync

        def fail_file_sync(descriptor):
            if not stat.S_ISDIR(os.fstat(descriptor).st_mode):
                raise OSError("disk failure")
            real_fsync(descriptor)

        with mock.patch.object(rollup.os, "fsync", side_effect=fail_file_sync):
            self.assertEqual(self.invoke()[0], 1)
        self.database.assert_called_once_with(date(2026, 11, 7))
        self.assertEqual(self.cursor(), "2026-11-07")
        self.assertEqual(audit.read_text(), "previous audit\n")
        self.assertEqual(list(runs.iterdir()), [audit])

    def test_state_replace_failure_leaves_durable_audit_for_safe_replay(self):
        self.write_state("2026-11-07")
        real_replace = os.replace

        def fail_state_replace(source, destination):
            if Path(destination) == self.state_file:
                self.assertEqual((self.state_dir / "runs/2026-11-07.json").read_text(),
                                 audit_text(date(2026, 11, 7)))
                raise OSError("state replacement failed")
            return real_replace(source, destination)

        with mock.patch.object(rollup.os, "replace", side_effect=fail_state_replace):
            self.assertEqual(self.invoke()[0], 1)
        self.assertEqual(self.cursor(), "2026-11-07")
        self.database.reset_mock()
        self.assertEqual(self.invoke()[0], 0)
        self.assertEqual(self.database.call_args_list[0].args[0], date(2026, 11, 7))
        self.assertEqual(self.cursor(), "2026-11-09")


class SqlExecutionTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.sql_path = Path(temporary.name) / "rollup.sql"
        self.sql_path.write_text("SELECT 'test SQL';\n", encoding="utf-8")
        self.enterContext(mock.patch.object(rollup, "SQL_PATH", self.sql_path))
        self.day = date(2026, 11, 7)

    def test_docker_invocation_keeps_credentials_in_container_and_sql_on_stdin(self):
        result = subprocess.CompletedProcess([], 0, stdout=audit_text(self.day), stderr="")
        with mock.patch.object(rollup.subprocess, "run", return_value=result) as execute:
            self.assertEqual(rollup.run_day(self.day), audit_text(self.day))
        self.assertEqual(execute.call_args.args[0], [
            "docker", "exec", "-i", "sub2api-postgres", "sh", "-c",
            'exec psql -X -qAt -v ON_ERROR_STOP=1 -v rollup_date="$1" '
            '-U "$POSTGRES_USER" -d "$POSTGRES_DB"', "rollup", "2026-11-07",
        ])
        self.assertEqual(execute.call_args.kwargs["input"], self.sql_path.read_text())
        self.assertEqual(execute.call_args.kwargs["timeout"], 45)
        self.assertTrue(execute.call_args.kwargs["check"])
        self.assertTrue(execute.call_args.kwargs["capture_output"])

    def test_command_prefix_is_substitutable_without_credentials(self):
        program = (
            "import sys\n"
            "assert sys.stdin.read() == \"SELECT 'test SQL';\\n\"\n"
            "assert sys.argv[1] == '2026-11-07'\n"
            f"sys.stdout.write({audit_text(self.day)!r})\n"
        )
        with mock.patch.object(rollup, "PSQL_COMMAND", [sys.executable, "-c", program]):
            self.assertEqual(rollup.run_day(self.day), audit_text(self.day))

    def test_timeout_and_sql_error_are_reported_without_raw_stderr(self):
        for error in (subprocess.TimeoutExpired(["test"], 45),
                      subprocess.CalledProcessError(3, ["test"], output=audit_text(self.day),
                                                    stderr="private diagnostics")):
            with self.subTest(error=error), mock.patch.object(rollup.subprocess, "run", side_effect=error):
                with self.assertRaises(RuntimeError) as raised:
                    rollup.run_day(self.day)
                self.assertIn("2026-11-07", str(raised.exception))
                self.assertNotIn("private diagnostics", str(raised.exception))

    def test_stdout_must_be_one_matching_json_audit(self):
        for output in ("", "[]\n", "{}\n", "invalid\n", audit_text(self.day) * 2,
                       "notice\n" + audit_text(self.day),
                       audit_text(date(2026, 11, 8))):
            with self.subTest(output=output):
                result = subprocess.CompletedProcess([], 0, stdout=output, stderr="")
                with mock.patch.object(rollup.subprocess, "run", return_value=result):
                    with self.assertRaises(ValueError):
                        rollup.run_day(self.day)


if __name__ == "__main__":
    unittest.main()
