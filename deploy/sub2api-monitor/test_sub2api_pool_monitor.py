import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

import sub2api_pool_monitor as monitor


NOW = datetime(2026, 7, 12, tzinfo=timezone.utc)


class FakeClient:
    def __init__(self, accounts, availability, monitors, usage=None):
        self._accounts = accounts
        self._availability = availability
        self._monitors = monitors
        self._usage = usage or {}

    def accounts(self):
        return self._accounts

    def availability(self):
        return {"account": self._availability}

    def channel_monitors(self):
        return self._monitors

    def account_usage(self, account_id):
        value = self._usage[account_id]
        if isinstance(value, Exception):
            raise value
        return value


def account(account_id, *, schedulable=True, account_type="oauth"):
    return {
        "id": account_id,
        "name": f"account-{account_id}",
        "status": "active",
        "schedulable": schedulable,
        "type": account_type,
    }


def channel(status="operational", checked_at=NOW, extra=None):
    return {
        "id": 1,
        "name": "channel-1",
        "enabled": True,
        "interval_seconds": 300,
        "last_checked_at": checked_at.isoformat(),
        "primary_model": "gpt-test",
        "primary_status": status,
        "extra_models_status": extra or [],
    }


class MonitorTests(unittest.TestCase):
    def test_collect_utilizations_recurses(self):
        payload = {"five_hour": {"utilization": 79}, "models": [{"utilization": 81}]}
        self.assertEqual([79.0, 81.0], monitor.collect_utilizations(payload))

    def test_unschedulable_accounts_are_excluded(self):
        client = FakeClient(
            [account(1), account(2, schedulable=False)],
            {"1": {"is_available": True}, "2": {"is_available": False}},
            [channel()],
            {1: {"utilization": 10}},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertEqual((1, 1), (report.pool_available, report.pool_total))
        self.assertFalse(report.alerting)

    def test_pool_below_80_alerts(self):
        accounts = [account(i, account_type="apikey") for i in range(1, 6)]
        availability = {str(i): {"is_available": i < 4} for i in range(1, 6)}
        report = monitor.evaluate(FakeClient(accounts, availability, [channel()]), now=NOW)
        self.assertEqual(60.0, report.pool_percentage)
        self.assertTrue(any("号池可用性" in item for item in report.problems))

    def test_model_exactly_80_does_not_alert(self):
        extra = [
            {"model": "m2", "status": "operational"},
            {"model": "m3", "status": "operational"},
            {"model": "m4", "status": "operational"},
            {"model": "m5", "status": "error"},
        ]
        client = FakeClient(
            [account(1)], {"1": {"is_available": True}}, [channel(extra=extra)], {1: {"utilization": 1}}
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertEqual(80.0, report.model_percentage)
        self.assertFalse(any("模型当前可用性" in item for item in report.problems))

    def test_quota_at_80_alerts(self):
        client = FakeClient(
            [account(1)], {"1": {"is_available": True}}, [channel()], {1: {"weekly": {"utilization": 80}}}
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertTrue(any("额度使用率" in item for item in report.problems))

    def test_stale_channel_is_emergency(self):
        client = FakeClient(
            [account(1)],
            {"1": {"is_available": True}},
            [channel(checked_at=NOW - timedelta(minutes=16))],
            {1: {"utilization": 1}},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertTrue(any("数据过期" in item for item in report.emergencies))

    def test_all_quota_checks_failed_is_emergency(self):
        client = FakeClient(
            [account(1)],
            {"1": {"is_available": True}},
            [channel()],
            {1: monitor.MonitorError("failed")},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertTrue(any("额度查询均失败" in item for item in report.emergencies))

    def test_state_round_trip(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            monitor.save_state(path, {"alerting": True})
            self.assertEqual({"alerting": True}, monitor.load_state(path))


if __name__ == "__main__":
    unittest.main()
