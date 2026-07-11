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
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone
from email.message import EmailMessage
from pathlib import Path
from typing import Any


DEFAULT_THRESHOLD = 80.0
DEFAULT_TIMEOUT_SECONDS = 60


class MonitorError(RuntimeError):
    pass


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def parse_time(value: str | None) -> datetime | None:
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


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
        data = None if body is None else json.dumps(body).encode("utf-8")
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        request = urllib.request.Request(
            f"{self.base_url}{path}", data=data, headers=headers, method=method
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                return json.loads(response.read())
        except urllib.error.HTTPError as exc:
            exc.read()
            raise MonitorError(f"Sub2API HTTP {exc.code}: {path}") from exc
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
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

    def availability(self) -> dict[str, Any]:
        payload = unwrap(self.request("/admin/ops/account-availability"))
        if not isinstance(payload, dict) or not isinstance(payload.get("account"), dict):
            raise MonitorError("Sub2API availability response has unexpected shape")
        return payload

    def channel_monitors(self) -> list[dict[str, Any]]:
        payload = unwrap(self.request("/admin/channel-monitors?page=1&page_size=100"))
        items = payload.get("items") if isinstance(payload, dict) else None
        if not isinstance(items, list):
            raise MonitorError("Sub2API channel monitor list has unexpected shape")
        return items

    def account_usage(self, account_id: int) -> Any:
        return unwrap(
            self.request(f"/admin/accounts/{account_id}/usage?source=active&force=false")
        )


@dataclass
class Report:
    checked_at: datetime
    pool_available: int = 0
    pool_total: int = 0
    model_operational: int = 0
    model_total: int = 0
    max_quota_utilization: float | None = None
    quota_account_name: str = ""
    quota_checks_ok: int = 0
    quota_checks_failed: int = 0
    channel_details: list[str] = field(default_factory=list)
    problems: list[str] = field(default_factory=list)
    emergencies: list[str] = field(default_factory=list)

    @property
    def pool_percentage(self) -> float:
        return 100.0 if self.pool_total == 0 else self.pool_available / self.pool_total * 100

    @property
    def model_percentage(self) -> float:
        return 100.0 if self.model_total == 0 else self.model_operational / self.model_total * 100

    @property
    def alerting(self) -> bool:
        return bool(self.problems or self.emergencies)


def evaluate(
    client: Sub2APIClient,
    threshold: float = DEFAULT_THRESHOLD,
    now: datetime | None = None,
) -> Report:
    current = now or utc_now()
    report = Report(checked_at=current)

    accounts = client.accounts()
    availability = client.availability().get("account", {})
    eligible = [
        account
        for account in accounts
        if account.get("status") == "active" and account.get("schedulable") is True
    ]
    report.pool_total = len(eligible)
    report.pool_available = sum(
        1
        for account in eligible
        if availability.get(str(account.get("id")), {}).get("is_available") is True
    )
    if report.pool_total == 0:
        report.emergencies.append("没有找到 active 且 schedulable 的账号")
    elif report.pool_percentage < threshold:
        report.problems.append(
            f"号池可用性 {report.pool_percentage:.1f}% 低于 {threshold:.0f}%"
        )

    monitors = [item for item in client.channel_monitors() if item.get("enabled") is True]
    if not monitors:
        report.emergencies.append("没有启用的 Sub2API 渠道监测")
    for monitor in monitors:
        name = str(monitor.get("name") or f"monitor-{monitor.get('id')}")
        last_checked = parse_time(monitor.get("last_checked_at"))
        interval = max(int(monitor.get("interval_seconds") or 300), 15)
        stale_after = max(interval * 3, 900)
        if last_checked is None or (current - last_checked).total_seconds() > stale_after:
            report.emergencies.append(f"渠道监测数据过期：{name}")

        primary_status = str(monitor.get("primary_status") or "unknown")
        report.model_total += 1
        if primary_status == "operational":
            report.model_operational += 1
        report.channel_details.append(
            f"{name} / {monitor.get('primary_model')}: {primary_status}"
        )
        for extra in monitor.get("extra_models_status") or []:
            status = str(extra.get("status") or "unknown")
            report.model_total += 1
            if status == "operational":
                report.model_operational += 1
            report.channel_details.append(f"{name} / {extra.get('model')}: {status}")

    if report.model_total and report.model_percentage < threshold:
        report.problems.append(
            f"模型当前可用性 {report.model_percentage:.1f}% 低于 {threshold:.0f}%"
        )

    quota_candidates = [
        account
        for account in eligible
        if account.get("type") in {"oauth", "setup-token"}
    ]
    for account in quota_candidates:
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
        if report.max_quota_utilization is None or account_max > report.max_quota_utilization:
            report.max_quota_utilization = account_max
            report.quota_account_name = str(account.get("name") or account.get("id"))

    if quota_candidates and report.quota_checks_ok == 0:
        report.emergencies.append("所有可监控账号的额度查询均失败")
    if report.max_quota_utilization is not None and report.max_quota_utilization >= threshold:
        report.problems.append(
            f"账号 {report.quota_account_name} 的额度使用率达到 "
            f"{report.max_quota_utilization:.1f}%"
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
    else:
        subject = "[告警] 云贝 Sub2API 号池需要处理"
        heading = "Sub2API 号池或模型可用性告警"

    quota = (
        "暂不可用"
        if report.max_quota_utilization is None
        else f"{report.max_quota_utilization:.1f}%（{report.quota_account_name}）"
    )
    lines = [
        heading,
        "",
        f"检查时间：{report.checked_at.astimezone().strftime('%Y-%m-%d %H:%M:%S %Z')}",
        f"号池可用性：{report.pool_available}/{report.pool_total}（{report.pool_percentage:.1f}%）",
        f"模型可用性：{report.model_operational}/{report.model_total}（{report.model_percentage:.1f}%）",
        f"最大额度使用率：{quota}",
        f"额度查询：成功 {report.quota_checks_ok}，失败 {report.quota_checks_failed}",
    ]
    if report.problems:
        lines.extend(["", "告警原因：", *[f"- {item}" for item in report.problems]])
    if report.emergencies:
        lines.extend(["", "突发问题：", *[f"- {item}" for item in report.emergencies]])
    if report.channel_details:
        lines.extend(["", "渠道模型状态：", *[f"- {item}" for item in report.channel_details]])
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
            report = evaluate(build_client(args.timeout), args.threshold)
        except Exception as exc:  # keep the scheduled job alive and alert on unexpected failures
            report = Report(checked_at=utc_now(), emergencies=[str(exc)])

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
                },
            )
        return 1 if report.alerting else 0


if __name__ == "__main__":
    raise SystemExit(main())
