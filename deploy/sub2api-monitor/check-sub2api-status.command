#!/bin/zsh

set -u

CONFIG_FILE=${YUNBAY_STATUS_CONFIG:-"$HOME/Desktop/云贝/服务器相关/sub2api-status.env"}

clear
printf '\033[1;36m%s\033[0m\n' '云贝 Sub2API 账号实时状态'
printf '%s\n\n' '============================================================'

if [[ ! -r "$CONFIG_FILE" ]]; then
  printf '\033[1;31m%s\033[0m\n' "找不到私有连接配置：${CONFIG_FILE}"
  echo '请根据 deploy/sub2api-monitor/sub2api-status.env.example 创建配置文件。'
  exit_code=1
else
  set -a
  source "$CONFIG_FILE"
  set +a

  required=(YUNBAY_SSH_HOST YUNBAY_SSH_PORT YUNBAY_SSH_USER YUNBAY_PROXY_HOST YUNBAY_PROXY_PORT YUNBAY_SSH_KEY YUNBAY_KNOWN_HOSTS)
  missing=()
  for name in $required; do
    if [[ -z ${(P)name:-} ]]; then
      missing+=("$name")
    fi
  done

  if (( ${#missing[@]} > 0 )); then
    printf '\033[1;31m%s\033[0m\n' "私有配置缺少：${(j:, :)missing}"
    exit_code=1
  elif ! nc -z -w 3 "$YUNBAY_PROXY_HOST" "$YUNBAY_PROXY_PORT" >/dev/null 2>&1; then
    printf '\033[1;31m%s\033[0m\n' "无法连接本机代理端口 ${YUNBAY_PROXY_HOST}:${YUNBAY_PROXY_PORT}。"
    echo '请先启动 Clash，然后重新双击本文件。'
    exit_code=1
  elif [[ ! -f "$YUNBAY_SSH_KEY" ]]; then
    printf '\033[1;31m%s\033[0m\n' "找不到 SSH 私钥：${YUNBAY_SSH_KEY}"
    exit_code=1
  else
    echo '正在连接生产服务器并执行实时测活，请稍候……'
    echo
    ssh \
      -p "$YUNBAY_SSH_PORT" \
      -o BatchMode=yes \
      -o ConnectTimeout=20 \
      -o ConnectionAttempts=2 \
      -o IdentitiesOnly=yes \
      -o StrictHostKeyChecking=yes \
      -o "UserKnownHostsFile=${YUNBAY_KNOWN_HOSTS}" \
      -o "ProxyCommand=nc -x ${YUNBAY_PROXY_HOST}:${YUNBAY_PROXY_PORT} -X 5 %h %p" \
      -i "$YUNBAY_SSH_KEY" \
      "${YUNBAY_SSH_USER}@${YUNBAY_SSH_HOST}" \
      "timeout --signal=TERM --kill-after=10s 180s /bin/bash -lc 'set -a; . /home/deploy/.config/yunbay/sub2api-monitor.env; set +a; /opt/new-api/monitor/sub2api-pool-monitor/sub2api_pool_monitor.py --dry-run'"
    exit_code=$?
  fi
fi

printf '\n%s\n' '============================================================'
if (( exit_code == 0 )); then
  printf '\033[1;32m%s\033[0m\n' '实时检查完成。上面的结果不会触发告警邮件。'
else
  printf '\033[1;31m%s\033[0m\n' "检查失败，退出码：${exit_code}"
  echo '如果提示连接超时，请检查代理与服务器状态后重试。'
fi

echo
if [[ -t 0 ]]; then
  read -r '?按回车键关闭窗口。'
fi
exit "$exit_code"
