import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path

import sub2api_pool_monitor as monitor


NOW = datetime(2026, 7, 12, tzinfo=timezone.utc)


class FakeClient:
    def __init__(self, group_name, accounts, usage=None, relay_results=None):
        self.group = {"id": 6 if group_name == monitor.SELF_HOSTED_GROUP else 9, "name": group_name}
        self._accounts = accounts
        self._usage = usage or {}
        self._relay_results = relay_results or {}

    def accounts(self):
        return self._accounts

    def account_details(self, account_id):
        return next(account for account in self._accounts if account["id"] == account_id)

    def groups(self):
        return [self.group]

    def group_api_keys(self, group_id):
        return [{"id": monitor.PRIMARY_API_KEY_ID, "name": "new-api-upstream"}]

    def account_usage(self, account_id):
        value = self._usage[account_id]
        if isinstance(value, Exception):
            raise value
        return value

    def test_account(self, account_id):
        value = self._relay_results.get(account_id, (True, 1.0))
        if isinstance(value, Exception):
            raise value
        return value


def account(account_id, account_type="oauth", group_id=6):
    return {
        "id": account_id,
        "name": f"account-{account_id}",
        "status": "active",
        "schedulable": True,
        "type": account_type,
        "group_ids": [group_id],
    }


class MonitorTests(unittest.TestCase):
    def resource_stats(self, cpu=20.0, memory=30.0, disk=40.0):
        gib = 1024 ** 3
        return monitor.ResourceStats(
            cpu_percent=cpu,
            cpu_count=2,
            load_1m=0.4,
            memory_used_bytes=2 * gib,
            memory_total_bytes=8 * gib,
            memory_used_percent=memory,
            disk_used_bytes=20 * gib,
            disk_total_bytes=100 * gib,
            disk_used_percent=disk,
        )

    def test_collect_utilizations_recurses(self):
        payload = {"five_hour": {"utilization": 79}, "models": [{"utilization": 81}]}
        self.assertEqual([79.0, 81.0], monitor.collect_utilizations(payload))

    def test_capacity_is_inferred_from_long_window_metadata(self):
        self.assertEqual(
            200.0,
            monitor.estimate_account_capacity_usd(
                {"extra": {"codex_primary_window_minutes": 43800}}
            ),
        )
        self.assertEqual(
            15.0,
            monitor.estimate_account_capacity_usd(
                {"extra": {"codex_primary_window_minutes": 300}}
            ),
        )

    def test_capacity_override_wins(self):
        self.assertEqual(350.0, monitor.estimate_account_capacity_usd({}, 350))

    def test_self_hosted_text_works_and_images_work_is_normal(self):
        client = FakeClient(
            monitor.SELF_HOSTED_GROUP,
            [account(1, "apikey"), account(2)],
            {2: {"utilization": 99}},
            {1: (True, 2.0), 2: (True, 1.0)},
        )
        report = monitor.evaluate(client, now=NOW, capacity_weights={2: 200})
        self.assertFalse(report.alerting)
        self.assertEqual((1, 1), (report.relay_available, report.own_available))
        self.assertEqual(99.0, report.total_quota_utilization)

    def test_self_hosted_exhausted_own_pool_uses_required_message(self):
        client = FakeClient(
            monitor.SELF_HOSTED_GROUP,
            [account(1, "apikey"), account(2), account(3)],
            {2: {"utilization": 100}, 3: {"utilization": 100}},
            {1: (True, 2.0), 2: (True, 1.0), 3: (True, 1.0)},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertIn(
            "现在正常的文字模型还能用，但是图片模型已经不能用了。", report.problems
        )

    def test_self_hosted_tests_all_relays_for_latency(self):
        client = FakeClient(
            monitor.SELF_HOSTED_GROUP,
            [account(1, "apikey"), account(2, "apikey"), account(3)],
            {3: {"utilization": 10}},
            {1: (True, 2.0), 2: (False, 1.0), 3: (True, 1.0)},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertEqual(2, report.relay_tested)
        self.assertEqual(2.0, report.relay_average_latency)

    def test_self_hosted_marks_all_live_relays_slow(self):
        client = FakeClient(
            monitor.SELF_HOSTED_GROUP,
            [account(1, "apikey"), account(2, "apikey"), account(3)],
            {3: {"utilization": 10}},
            {1: (True, 16.0), 2: (True, 20.0), 3: (True, 1.0)},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertTrue(report.relay_all_slow)
        self.assertEqual(2, monitor.apply_relay_latency_stability(report, 1))
        self.assertFalse(report.problems)
        self.assertEqual(3, monitor.apply_relay_latency_stability(report, 2))
        self.assertTrue(any("连续 3 次检测" in item for item in report.problems))

    def test_self_hosted_does_not_alert_when_one_relay_is_fast(self):
        client = FakeClient(
            monitor.SELF_HOSTED_GROUP,
            [account(1, "apikey"), account(2, "apikey"), account(3)],
            {3: {"utilization": 10}},
            {1: (True, 16.0), 2: (True, 14.9), 3: (True, 1.0)},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertFalse(report.relay_all_slow)
        self.assertEqual(0, monitor.apply_relay_latency_stability(report, 2))

    def test_safe_group_uses_weighted_total_usage(self):
        accounts = [account(1, group_id=9), account(2, group_id=9)]
        client = FakeClient(
            monitor.SAFE_GROUP,
            accounts,
            {1: {"utilization": 70}, 2: {"utilization": 90}},
        )
        report = monitor.evaluate(client, now=NOW, capacity_weights={1: 15, 2: 200})
        self.assertAlmostEqual(88.6047, report.total_quota_utilization, places=3)
        self.assertTrue(any("总用量达到 88.6%" in item for item in report.problems))

    def test_safe_group_below_80_does_not_alert(self):
        accounts = [account(1, group_id=9), account(2, group_id=9)]
        client = FakeClient(
            monitor.SAFE_GROUP,
            accounts,
            {1: {"utilization": 60}, 2: {"utilization": 99}},
        )
        report = monitor.evaluate(client, now=NOW, capacity_weights={1: 200, 2: 15})
        self.assertAlmostEqual(62.7209, report.total_quota_utilization, places=3)
        self.assertFalse(report.alerting)

    def test_safe_group_new_account_is_auto_estimated(self):
        accounts = [account(1, group_id=9), account(2, group_id=9)]
        client = FakeClient(
            monitor.SAFE_GROUP,
            accounts,
            {1: {"utilization": 60}, 2: {"utilization": 70}},
        )
        report = monitor.evaluate(client, now=NOW, capacity_weights={1: 15})
        self.assertFalse(any("缺少额度总量配置" in item for item in report.emergencies))

    def test_safe_group_report_omits_relay_wording(self):
        accounts = [account(1, group_id=9), account(2, group_id=9)]
        client = FakeClient(
            monitor.SAFE_GROUP,
            accounts,
            {1: {"utilization": 10}, 2: {"utilization": 20}},
        )
        report = monitor.evaluate(client, now=NOW, capacity_weights={1: 15, 2: 200})
        _, text, _ = monitor.render_report(report)
        self.assertNotIn("中转站", text)
        self.assertIn("估算总体剩余", text)
        self.assertIn("估算总额 $215.00", text)

    def test_safe_group_rejects_relay_account(self):
        client = FakeClient(
            monitor.SAFE_GROUP,
            [account(1, "apikey", group_id=9), account(2, group_id=9)],
            {2: {"utilization": 10}},
        )
        report = monitor.evaluate(client, now=NOW, capacity_weights={2: 15})
        self.assertTrue(any("不应存在的中转站账号" in item for item in report.emergencies))

    def test_all_own_usage_checks_failed_is_emergency(self):
        client = FakeClient(
            monitor.SELF_HOSTED_GROUP,
            [account(1, "apikey"), account(2)],
            {2: monitor.MonitorError("failed")},
            {1: (True, 2.0), 2: (True, 1.0)},
        )
        report = monitor.evaluate(client, now=NOW)
        self.assertTrue(any("额度查询均失败" in item for item in report.emergencies))

    def test_state_round_trip(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "state.json"
            monitor.save_state(path, {"alerting": True})
            self.assertEqual({"alerting": True}, monitor.load_state(path))

    def test_normal_resource_report_has_half_hour_subject_and_stats(self):
        report = monitor.Report(checked_at=NOW, resources=self.resource_stats())
        subject, text, _ = monitor.render_report(report, periodic=True)
        self.assertEqual("[定时] 云贝服务器资源与 Sub2API 状态", subject)
        self.assertIn("CPU：20.0%", text)
        self.assertIn("内存：2.00 / 8.00 GiB", text)
        self.assertIn("根分区：20.00 / 100.00 GiB", text)

    def test_resource_thresholds_feed_existing_alert_channel(self):
        report = monitor.Report(checked_at=NOW)
        monitor.apply_resource_thresholds(
            report, self.resource_stats(cpu=81.0, memory=86.0, disk=90.0)
        )
        self.assertTrue(report.alerting)
        self.assertTrue(any("CPU 使用率达到 81.0%" in item for item in report.problems))
        self.assertTrue(any("内存使用率达到 86.0%" in item for item in report.problems))
        self.assertTrue(any("根分区使用率达到 90.0%" in item for item in report.problems))

    def test_normal_report_is_due_every_thirty_minutes(self):
        self.assertTrue(monitor.normal_report_due({}, NOW, 1800))
        self.assertFalse(
            monitor.normal_report_due(
                {"last_normal_email_at": "2026-07-11T23:45:01+00:00"}, NOW, 1800
            )
        )
        self.assertTrue(
            monitor.normal_report_due(
                {"last_normal_email_at": "2026-07-11T23:30:00+00:00"}, NOW, 1800
            )
        )


if __name__ == "__main__":
    unittest.main()
