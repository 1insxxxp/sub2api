"""Exercise the maintenance publisher against an isolated local PostgreSQL DB."""

import json
import os
from pathlib import Path
import subprocess
import unittest
from urllib.parse import urlparse


DSN = os.environ.get("SUB2API_ROLLUP_TEST_DSN", "")
SQL = Path(__file__).resolve().parents[1] / "deploy/maintenance/group_usage_rollup.sql"


@unittest.skipUnless(DSN, "set SUB2API_ROLLUP_TEST_DSN to an isolated local test DB")
class RollupSQLTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        address = urlparse(DSN)
        if address.hostname != "127.0.0.1" or address.port == 5432 or not address.path.endswith("_test"):
            raise ValueError("An explicit isolated loopback test database is required")

    def query(self, sql):
        result = subprocess.run(
            ["psql", DSN, "-X", "-qAt", "-v", "ON_ERROR_STOP=1"],
            input=sql, text=True, capture_output=True, check=True,
        )
        return result.stdout.strip()

    def publish(self, day="2026-11-07", succeeds=True):
        result = subprocess.run(
            ["psql", DSN, "-X", "-qAt", "-v", "ON_ERROR_STOP=1", "-v", f"rollup_date={day}"],
            input=SQL.read_text(), text=True, capture_output=True,
        )
        if succeeds:
            self.assertEqual(result.returncode, 0, result.stderr)
            return json.loads(result.stdout)
        self.assertNotEqual(result.returncode, 0, result.stdout)
        return result.stderr

    def setUp(self):
        self.query("""
            DROP SCHEMA public CASCADE;
            CREATE SCHEMA public;
            CREATE TABLE usage_logs (
                id bigint PRIMARY KEY, user_id bigint, group_id bigint,
                created_at timestamptz, actual_cost numeric(20,10)
            );
            CREATE INDEX ON usage_logs(created_at);
            CREATE TABLE sub_admin_commission_exclusions (
                user_id bigint, started_at timestamptz, ended_at timestamptz
            );
            CREATE TABLE usage_group_rollup_state (
                id smallint PRIMARY KEY, closed_before date,
                timezone_name text NOT NULL DEFAULT 'Asia/Shanghai'
            );
            INSERT INTO usage_group_rollup_state(id,closed_before) VALUES (1, '2026-08-07');
            CREATE TABLE usage_group_daily_rollups (
                bucket_date date, group_id bigint, actual_cost numeric(20,10),
                commission_excluded_cost numeric(20,10), computed_at timestamptz,
                PRIMARY KEY(bucket_date,group_id)
            );
            CREATE TABLE wallet_guard (balance numeric, quota_used numeric);
            INSERT INTO wallet_guard VALUES (42,7);
            INSERT INTO usage_group_daily_rollups VALUES
                ('2026-11-06',1,77,0,NOW()),
                ('2026-11-07',1,99,1,NOW()),
                ('2026-11-07',9,9,0,NOW());
            INSERT INTO sub_admin_commission_exclusions VALUES
                (2,'2026-11-07 12:00+08','2026-11-07 14:00+08');
            INSERT INTO usage_logs VALUES
                (1,1,1,'2026-11-07 00:00+08',10),
                (2,2,1,'2026-11-07 11:59+08',20),
                (3,2,1,'2026-11-07 12:00+08',30),
                (4,2,1,'2026-11-07 14:00+08',40),
                (5,3,2,'2026-11-07 23:59+08',5),
                (6,3,2,'2026-11-08 00:00+08',0);
        """)

    def totals(self):
        return self.query("""
            SELECT group_id,actual_cost,commission_excluded_cost
            FROM usage_group_daily_rollups WHERE bucket_date='2026-11-07'
            ORDER BY group_id;
        """)

    def test_boundaries_exclusions_idempotence_and_late_usage(self):
        result = self.publish()
        self.assertEqual((result["source_rows"], result["group_count"]), (5, 2))
        self.assertEqual((result["actual_cost"], result["commission_excluded_cost"]), (105, 30))
        expected = self.totals()
        self.publish()
        self.assertEqual(self.totals(), expected)
        self.assertEqual(self.query("SELECT actual_cost FROM usage_group_daily_rollups WHERE bucket_date='2026-11-06'"), "77.0000000000")
        self.query("INSERT INTO usage_logs VALUES (7,1,1,'2026-11-07 15:00+08',2)")
        self.assertEqual(self.publish()["actual_cost"], 107)
        self.assertEqual(self.query("SELECT balance,quota_used FROM wallet_guard"), "42|7")
        self.assertEqual(self.query("SELECT COUNT(*) FROM usage_logs"), "7")
        self.assertEqual(self.query("SELECT closed_before FROM usage_group_rollup_state"), "2026-08-07")

    def test_partial_publication_failure_rolls_back(self):
        before = self.totals()
        self.query("""
            CREATE FUNCTION reject_rollup() RETURNS trigger LANGUAGE plpgsql AS $$
            BEGIN IF NEW.group_id=2 THEN RAISE EXCEPTION 'injected publication failure'; END IF;
            RETURN NEW; END $$;
            CREATE TRIGGER fail_rollup BEFORE INSERT OR UPDATE ON usage_group_daily_rollups
                FOR EACH ROW EXECUTE FUNCTION reject_rollup();
        """)
        self.assertIn("injected publication failure", self.publish(succeeds=False))
        self.assertEqual(self.totals(), before)

    def test_empty_source_preserves_existing_history(self):
        self.assertIn("Source day is empty", self.publish("2026-11-06", succeeds=False))
        self.assertEqual(self.query("SELECT actual_cost FROM usage_group_daily_rollups WHERE bucket_date='2026-11-06'"), "77.0000000000")

    def test_free_usage_is_a_published_group(self):
        result = self.publish("2026-11-08")
        self.assertEqual((result["source_rows"], result["group_count"], result["actual_cost"]), (1, 1, 0))

    def test_future_and_invalid_amounts_are_rejected(self):
        before = self.totals()
        self.assertIn("complete historical days", self.publish("2100-01-01", succeeds=False))
        self.query("UPDATE usage_logs SET actual_cost=-1 WHERE id=1")
        self.assertIn("Invalid source monetary totals", self.publish(succeeds=False))
        self.assertEqual(self.totals(), before)

    def test_changed_reporting_timezone_is_rejected(self):
        before = self.totals()
        self.query("UPDATE usage_group_rollup_state SET timezone_name='UTC'")
        self.assertIn("unexpected timezone", self.publish(succeeds=False))
        self.assertEqual(self.totals(), before)


if __name__ == "__main__":
    unittest.main()
