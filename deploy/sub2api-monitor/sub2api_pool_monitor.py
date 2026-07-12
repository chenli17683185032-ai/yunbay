#!/usr/bin/env python3
"""Monitor Sub2API pool/model availability and quota usage."""

from __future__ import annotations

import argparse
import fcntl
import html
import json
import os
import smtplib
import ssl
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone
from email.message import EmailMessage
from pathlib import Path
from typing import Any


DEFAULT_THRESHOLD = 80.0
DEFAULT_TIMEOUT_SECONDS = 60
PRIMARY_API_KEY_ID = 1
SELF_HOSTED_GROUP = "自建池"
SAFE_GROUP = "自建安全使用"
CAPACITY_ENV = "ACCOUNT_CAPACITY_WEIGHTS_JSON"
RELAY_LATENCY_ALERT_SECONDS = 15.0
RELAY_SLOW_STREAK_REQUIRED = 3


class MonitorError(RuntimeError):
    pass


def utc_now() -> datetime:
    return datetime.now(timezone.utc)



def collect_utilizations(value: Any) -> list[float]:
    found: list[float] = []
    if isinstance(value, dict):
        for key, child in value.items():
            if key == "utilization" and isinstance(child, (int, float)):
                found.append(float(child))
            else:
                found.extend(collect_utilizations(child))
    elif isinstance(value, list):
        for child in value:
            found.extend(collect_utilizations(child))
    return found


def unwrap(payload: Any) -> Any:
    if isinstance(payload, dict) and "data" in payload:
        return payload["data"]
    return payload


def run_command(args: list[str], timeout: int = DEFAULT_TIMEOUT_SECONDS) -> str:
    try:
        result = subprocess.run(
            args,
            check=True,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as exc:
        raise MonitorError(f"command failed: {args[0]}") from exc
    return result.stdout


def container_environment(container: str) -> dict[str, str]:
    output = run_command(
        [
            "docker",
            "inspect",
            container,
            "--format",
            "{{range .Config.Env}}{{println .}}{{end}}",
        ]
    )
    return dict(line.split("=", 1) for line in output.splitlines() if "=" in line)


def container_ip(container: str) -> str:
    output = run_command(
        [
            "docker",
            "inspect",
            container,
            "--format",
            "{{range .NetworkSettings.Networks}}{{println .IPAddress}}{{end}}",
        ]
    )
    address = next((line.strip() for line in output.splitlines() if line.strip()), "")
    if not address:
        raise MonitorError(f"container has no network address: {container}")
    return address


class Sub2APIClient:
    def __init__(self, base_url: str, email: str, password: str, timeout: int) -> None:
        self.base_url = base_url.rstrip("/")
        self.email = email
        self.password = password
        self.timeout = timeout
        self.token = ""

    def request(self, path: str, method: str = "GET", body: Any = None) -> Any:
        raw = self.request_raw(path, method, body)
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            raise MonitorError(f"Sub2API returned invalid JSON: {path}") from exc

    def request_raw(self, path: str, method: str = "GET", body: Any = None) -> bytes:
        data = None if body is None else json.dumps(body).encode("utf-8")
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        request = urllib.request.Request(
            f"{self.base_url}{path}", data=data, headers=headers, method=method
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                return response.read()
        except urllib.error.HTTPError as exc:
            exc.read()
            raise MonitorError(f"Sub2API HTTP {exc.code}: {path}") from exc
        except (urllib.error.URLError, TimeoutError) as exc:
            raise MonitorError(f"Sub2API request failed: {path}") from exc

    def login(self) -> None:
        payload = unwrap(
            self.request(
                "/auth/login",
                "POST",
                {"email": self.email, "password": self.password},
            )
        )
        token = payload.get("access_token") if isinstance(payload, dict) else None
        if not token:
            raise MonitorError("Sub2API login returned no access token")
        self.token = token

    def accounts(self) -> list[dict[str, Any]]:
        payload = unwrap(self.request("/admin/accounts?page=1&page_size=100"))
        items = payload.get("items") if isinstance(payload, dict) else None
        if not isinstance(items, list):
            raise MonitorError("Sub2API account list has unexpected shape")
        return items

    def groups(self) -> list[dict[str, Any]]:
        payload = unwrap(self.request("/admin/groups/all"))
        if not isinstance(payload, list):
            raise MonitorError("Sub2API group list has unexpected shape")
        return payload

    def group_api_keys(self, group_id: int) -> list[dict[str, Any]]:
        payload = unwrap(
            self.request(f"/admin/groups/{group_id}/api-keys?page=1&page_size=100")
        )
        items = payload.get("items") if isinstance(payload, dict) else None
        if not isinstance(items, list):
            raise MonitorError("Sub2API API key list has unexpected shape")
        return items

    def account_usage(self, account_id: int) -> Any:
        return unwrap(
            self.request(f"/admin/accounts/{account_id}/usage?source=active&force=false")
        )

    def test_account(self, account_id: int) -> tuple[bool, float]:
        started = time.monotonic()
        raw = self.request_raw(f"/admin/accounts/{account_id}/test", "POST", {})
        latency = time.monotonic() - started
        for line in raw.decode("utf-8", errors="replace").splitlines():
            if not line.startswith("data: "):
                continue
            try:
                event = json.loads(line[6:])
            except json.JSONDecodeError:
                continue
            if event.get("type") == "test_complete":
                return event.get("success") is True, latency
        return False, latency


@dataclass
class Report:
    checked_at: datetime
    mode: str = ""
    primary_key_name: str = ""
    relay_available: int = 0
    relay_tested: int = 0
    relay_low_latency: int = 0
    relay_all_slow: bool = False
    relay_average_latency: float | None = None
    own_available: int = 0
    own_total: int = 0
    total_quota_utilization: float | None = None
    weighted_used_capacity: float | None = None
    weighted_total_capacity: float | None = None
    own_min_remaining: float | None = None
    own_max_remaining: float | None = None
    quota_checks_ok: int = 0
    quota_checks_failed: int = 0
    account_details: list[str] = field(default_factory=list)
    problems: list[str] = field(default_factory=list)
    emergencies: list[str] = field(default_factory=list)

    @property
    def alerting(self) -> bool:
        return bool(self.problems or self.emergencies)


def evaluate(
    client: Sub2APIClient,
    threshold: float = DEFAULT_THRESHOLD,
    now: datetime | None = None,
    capacity_weights: dict[int, float] | None = None,
) -> Report:
    current = now or utc_now()
    report = Report(checked_at=current)

    accounts = client.accounts()
    groups = client.groups()
    group_by_id = {int(group["id"]): group for group in groups}
    primary_key = None
    primary_group = None
    for group in groups:
        for api_key in client.group_api_keys(int(group["id"])):
            if int(api_key.get("id", 0)) == PRIMARY_API_KEY_ID:
                primary_key = api_key
                primary_group = group
                break
        if primary_key:
            break
    if not primary_key or not primary_group:
        report.emergencies.append("找不到供 new-api 使用的第一个 Sub2API API Key")
        return report

    report.primary_key_name = str(primary_key.get("name") or PRIMARY_API_KEY_ID)
    report.mode = str(primary_group.get("name") or "")
    group_id = int(primary_group["id"])
    members = [
        account
        for account in accounts
        if group_id in (account.get("group_ids") or [])
        and account.get("status") == "active"
        and account.get("schedulable") is True
    ]
    relay_accounts = [account for account in members if account.get("type") == "apikey"]
    own_accounts = [
        account for account in members if account.get("type") in {"oauth", "setup-token"}
    ]
    report.own_total = len(own_accounts)

    usage_by_account: list[tuple[dict[str, Any], float]] = []
    own_remaining_values: list[float] = []
    own_liveness: dict[int, tuple[bool, float | None]] = {}
    for account in own_accounts:
        try:
            own_liveness[int(account["id"])] = client.test_account(int(account["id"]))
        except MonitorError:
            own_liveness[int(account["id"])] = (False, None)
        try:
            values = collect_utilizations(client.account_usage(int(account["id"])))
        except MonitorError:
            report.quota_checks_failed += 1
            continue
        if not values:
            report.quota_checks_failed += 1
            continue
        report.quota_checks_ok += 1
        account_max = max(values)
        usage_by_account.append((account, account_max))
        own_remaining_values.append(max(0.0, 100 - account_max))
        alive, latency = own_liveness[int(account["id"])]
        if account_max < 100 and alive:
            report.own_available += 1
        report.account_details.append(
            f"自有账号 {account.get('name')}: 剩余 {max(0.0, 100-account_max):.1f}%，"
            f"测活 {'成功' if alive else '失败'}"
            + (f"，延迟 {latency:.2f} 秒" if latency is not None else "")
        )

    if own_accounts and report.quota_checks_ok == 0:
        report.emergencies.append("该分组内所有自有账号的额度查询均失败")
    if own_remaining_values:
        report.own_min_remaining = min(own_remaining_values)
        report.own_max_remaining = max(own_remaining_values)

    configured_weights = capacity_weights or {}
    missing_capacity_accounts = [
        str(account.get("name") or account.get("id"))
        for account, _ in usage_by_account
        if float(configured_weights.get(int(account["id"]), 0)) <= 0
    ]
    if usage_by_account and not missing_capacity_accounts:
        report.weighted_total_capacity = sum(
            float(configured_weights[int(account["id"])]) for account, _ in usage_by_account
        )
        report.weighted_used_capacity = sum(
            float(configured_weights[int(account["id"])]) * utilization / 100
            for account, utilization in usage_by_account
        )
        report.total_quota_utilization = (
            report.weighted_used_capacity / report.weighted_total_capacity * 100
        )

    if report.mode == SELF_HOSTED_GROUP:
        relay_latencies: list[float] = []
        for account in relay_accounts:
            report.relay_tested += 1
            try:
                usable, latency = client.test_account(int(account["id"]))
            except MonitorError:
                usable, latency = False, None
            report.account_details.append(
                f"中转站账号 {account.get('name')}: {'可用' if usable else '不可用'}"
                + (f"，延迟 {latency:.2f} 秒" if latency is not None else "")
            )
            if usable:
                report.relay_available += 1
                if latency is not None:
                    relay_latencies.append(latency)
                if latency is not None and latency <= RELAY_LATENCY_ALERT_SECONDS:
                    report.relay_low_latency += 1
        if relay_latencies:
            report.relay_average_latency = sum(relay_latencies) / len(relay_latencies)
        if not relay_accounts:
            report.emergencies.append("自建池中没有可测试的中转站账号")
        elif report.relay_available == 0:
            report.problems.append("自建池中转站账号测试失败，正常文字模型可能不可用")
        elif report.relay_low_latency == 0:
            report.relay_all_slow = True
        if own_accounts and report.own_available == 0 and report.quota_checks_ok > 0:
            if report.relay_available > 0:
                report.problems.append("现在正常的文字模型还能用，但是图片模型已经不能用了。")
            else:
                report.problems.append("自有账号额度已经用完，图片模型已经不能用了。")
    elif report.mode == SAFE_GROUP:
        if relay_accounts:
            report.emergencies.append("自建安全使用分组中出现了不应存在的中转站账号")
        if usage_by_account:
            if missing_capacity_accounts:
                report.emergencies.append(
                    "自建安全使用分组缺少额度总量配置："
                    + "、".join(missing_capacity_accounts)
                )
                return report
            if report.total_quota_utilization >= threshold:
                report.problems.append(
                    f"自建安全使用分组总用量达到 {report.total_quota_utilization:.1f}%"
                )
    else:
        known = ", ".join(
            group.get("name", "") for group in group_by_id.values() if group.get("name")
        )
        report.emergencies.append(
            f"new-api 使用的 Key 当前属于未配置监控逻辑的分组：{report.mode}（现有分组：{known}）"
        )
    return report


def smtp_options() -> dict[str, str]:
    postgres = container_environment("yunbay-postgres")
    user = postgres.get("POSTGRES_USER")
    database = postgres.get("POSTGRES_DB")
    if not user or not database:
        raise MonitorError("PostgreSQL container environment is incomplete")
    keys = (
        "SMTPServer",
        "SMTPPort",
        "SMTPSSLEnabled",
        "SMTPAccount",
        "SMTPToken",
        "SMTPFrom",
        "SystemName",
    )
    quoted = ",".join(f"'{key}'" for key in keys)
    query = f"SELECT key, value FROM options WHERE key IN ({quoted}) ORDER BY key"
    output = run_command(
        [
            "docker",
            "exec",
            "yunbay-postgres",
            "psql",
            "-U",
            user,
            "-d",
            database,
            "-At",
            "-F",
            "\t",
            "-c",
            query,
        ]
    )
    options = dict(line.split("\t", 1) for line in output.splitlines() if "\t" in line)
    missing = [key for key in keys[:-1] if not options.get(key)]
    if missing:
        raise MonitorError(f"new-api SMTP settings missing: {', '.join(missing)}")
    return options


def render_report(report: Report, recovered: bool = False) -> tuple[str, str, str]:
    if recovered:
        subject = "[恢复] 云贝 Sub2API 号池监控已恢复正常"
        heading = "Sub2API 号池监控已恢复正常"
    elif report.emergencies:
        subject = "[紧急] 云贝 Sub2API 监控异常"
        heading = "Sub2API 监控发现突发问题"
    elif report.problems:
        subject = "[告警] 云贝 Sub2API 号池需要处理"
        heading = "Sub2API 分组监控告警"
    else:
        subject = "[正常] 云贝 Sub2API 分组监控"
        heading = "Sub2API 分组监控正常"

    if report.total_quota_utilization is not None:
        own_summary = (
            f"估算总体剩余 {max(0.0, 100-report.total_quota_utilization):.1f}%，"
            f"约剩 ${report.weighted_total_capacity-report.weighted_used_capacity:.2f} / "
            f"估算总额 ${report.weighted_total_capacity:.2f}"
        )
    elif report.own_min_remaining is not None and report.own_max_remaining is not None:
        own_summary = (
            f"{report.own_available}/{report.own_total} 可用，单账号剩余范围 "
            f"{report.own_min_remaining:.1f}%～{report.own_max_remaining:.1f}%；"
            "上游未返回绝对总额度，无法准确加权合计"
        )
    else:
        own_summary = "暂无可汇总数据"
    lines = [
        heading,
        "",
        f"检查时间：{report.checked_at.astimezone().strftime('%Y-%m-%d %H:%M:%S %Z')}",
        f"new-api 使用的 Key：{report.primary_key_name or '未知'}",
        f"当前分组模式：{report.mode or '未知'}",
        "",
        "汇总：",
        f"- 自有账号总体余量：{own_summary}",
    ]
    if report.mode == SELF_HOSTED_GROUP:
        relay_summary = (
            f"{report.relay_available} 个可用中转站，平均延迟 "
            f"{report.relay_average_latency:.2f} 秒"
            if report.relay_average_latency is not None
            else f"{report.relay_available} 个可用中转站，暂无平均延迟"
        )
        lines.append(f"- 中转站总体状态：{relay_summary}")
    lines.append("")
    if report.mode == SELF_HOSTED_GROUP:
        lines.append(
            f"中转站账号测试：成功 {report.relay_available}，已测试 {report.relay_tested}"
        )
    lines.extend(
        [
            f"自有账号额度可用：{report.own_available}/{report.own_total}",
            f"自有账号估算总用量：{report.total_quota_utilization:.1f}%"
            if report.total_quota_utilization is not None
            else "自有账号估算总用量：暂不可用",
            (
                f"加权额度：已用约 ${report.weighted_used_capacity:.2f} / "
                f"总额 ${report.weighted_total_capacity:.2f}"
                if report.weighted_used_capacity is not None
                and report.weighted_total_capacity is not None
                else "加权额度：不适用"
            ),
            f"额度查询：成功 {report.quota_checks_ok}，失败 {report.quota_checks_failed}",
        ]
    )
    if report.problems:
        lines.extend(["", "告警原因：", *[f"- {item}" for item in report.problems]])
    if report.emergencies:
        lines.extend(["", "突发问题：", *[f"- {item}" for item in report.emergencies]])
    if report.account_details:
        lines.extend(["", "账号检测明细：", *[f"- {item}" for item in report.account_details]])
    if report.alerting:
        lines.extend(["", "异常未解除时，本邮件约每 5 分钟发送一次。"])
    text = "\n".join(lines)
    body = "<html><body><pre style=\"font:14px/1.6 sans-serif;white-space:pre-wrap\">" + html.escape(text) + "</pre></body></html>"
    return subject, text, body


def send_email(recipient: str, subject: str, text: str, body: str) -> None:
    options = smtp_options()
    message = EmailMessage()
    sender = options["SMTPFrom"]
    display_name = options.get("SystemName") or "yunbay"
    message["From"] = f"{display_name} <{sender}>"
    message["To"] = recipient
    message["Subject"] = subject
    message.set_content(text)
    message.add_alternative(body, subtype="html")

    host = options["SMTPServer"]
    port = int(options["SMTPPort"])
    use_ssl = options["SMTPSSLEnabled"].strip().lower() == "true"
    context = ssl.create_default_context()
    if use_ssl:
        smtp: smtplib.SMTP = smtplib.SMTP_SSL(host, port, timeout=30, context=context)
    else:
        smtp = smtplib.SMTP(host, port, timeout=30)
    with smtp:
        if not use_ssl:
            smtp.starttls(context=context)
        smtp.login(options["SMTPAccount"], options["SMTPToken"])
        smtp.send_message(message)


def load_state(path: Path) -> dict[str, Any]:
    try:
        return json.loads(path.read_text())
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return {}


def save_state(path: Path, state: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=path.name, dir=path.parent)
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump(state, handle, ensure_ascii=False)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def build_client(timeout: int) -> Sub2APIClient:
    environment = container_environment("yunbay-sub2api")
    email = environment.get("ADMIN_EMAIL")
    password = environment.get("ADMIN_PASSWORD")
    port = environment.get("SERVER_PORT", "8080")
    if not email or not password:
        raise MonitorError("Sub2API admin environment is incomplete")
    client = Sub2APIClient(
        f"http://{container_ip('yunbay-sub2api')}:{port}/api/v1",
        email,
        password,
        timeout,
    )
    client.login()
    return client


def load_capacity_weights() -> dict[int, float]:
    raw = os.environ.get(CAPACITY_ENV, "").strip()
    if not raw:
        return {}
    try:
        payload = json.loads(raw)
        weights = {int(key): float(value) for key, value in payload.items()}
    except (json.JSONDecodeError, TypeError, ValueError, AttributeError) as exc:
        raise MonitorError(f"{CAPACITY_ENV} is invalid") from exc
    if any(value <= 0 for value in weights.values()):
        raise MonitorError(f"{CAPACITY_ENV} values must be positive")
    return weights


def apply_relay_latency_stability(report: Report, previous_streak: int) -> int:
    streak = previous_streak + 1 if report.relay_all_slow else 0
    if streak >= RELAY_SLOW_STREAK_REQUIRED:
        report.problems.append(
            "所有可用中转站连续 3 次检测延迟均高于 15 秒，且没有其他可用的低延迟渠道"
        )
    return streak


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--recipient", default=os.environ.get("ALERT_EMAIL_TO", ""))
    parser.add_argument("--threshold", type=float, default=DEFAULT_THRESHOLD)
    parser.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT_SECONDS)
    parser.add_argument("--state-file", default=os.environ.get("MONITOR_STATE_FILE", "./state.json"))
    parser.add_argument("--lock-file", default=os.environ.get("MONITOR_LOCK_FILE", "./monitor.lock"))
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--test-email", action="store_true")
    args = parser.parse_args()
    if not args.recipient:
        print("ALERT_EMAIL_TO/--recipient is required", file=sys.stderr)
        return 2

    Path(args.lock_file).parent.mkdir(parents=True, exist_ok=True)
    with open(args.lock_file, "w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("another monitor run is still active")
            return 0

        if args.test_email:
            report = Report(checked_at=utc_now())
            subject, text, body = render_report(report)
            send_email(args.recipient, "[测试] 云贝 Sub2API 监控邮件", text, body)
            print("test email sent")
            return 0

        state_path = Path(args.state_file)
        previous = load_state(state_path)
        try:
            report = evaluate(
                build_client(args.timeout),
                args.threshold,
                capacity_weights=load_capacity_weights(),
            )
        except Exception as exc:  # keep the scheduled job alive and alert on unexpected failures
            report = Report(checked_at=utc_now(), emergencies=[str(exc)])

        relay_slow_streak = apply_relay_latency_stability(
            report, int(previous.get("relay_slow_streak", 0) or 0)
        )

        recovered = previous.get("alerting") is True and not report.alerting
        subject, text, body = render_report(report, recovered=recovered)
        print(text)
        if not args.dry_run and (report.alerting or recovered):
            send_email(args.recipient, subject, text, body)
            print("email sent")
        if not args.dry_run:
            save_state(
                state_path,
                {
                    "alerting": report.alerting,
                    "checked_at": report.checked_at.isoformat(),
                    "problems": report.problems,
                    "emergencies": report.emergencies,
                    "relay_slow_streak": relay_slow_streak,
                },
            )
        return 1 if report.alerting else 0


if __name__ == "__main__":
    raise SystemExit(main())
