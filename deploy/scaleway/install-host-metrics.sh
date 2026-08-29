#!/bin/sh
set -eu

[ "$(id -u)" -eq 0 ] || { echo 'must run as root' >&2; exit 1; }
[ "$#" -eq 1 ] || { echo "usage: $0 /path/to/deploy/scaleway" >&2; exit 2; }

source_dir=$1
install -d -m 700 /etc/currents
install -m 750 "$source_dir/report-host-metrics.sh" /usr/local/sbin/currents-report-host-metrics
install -m 644 "$source_dir/currents-host-metrics.service" /etc/systemd/system/currents-host-metrics.service
install -m 644 "$source_dir/currents-host-metrics.timer" /etc/systemd/system/currents-host-metrics.timer
systemctl daemon-reload
systemctl enable --now currents-host-metrics.timer
