import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("deploy-main-release.sh").read_text()
# Load the real functions without executing privileged setup or deploying anything.
FUNCTIONS = SCRIPT[SCRIPT.index("compose() {") : SCRIPT.index('case ",$services,"')]
FAIL = SCRIPT[SCRIPT.index("fail() {") : SCRIPT.index("release_sha=${1:-}")]


class DeployMainReleaseTests(unittest.TestCase):
    def run_shell(self, body, **env):
        with tempfile.TemporaryDirectory() as directory:
            calls = Path(directory) / "calls"
            calls.touch()
            result = subprocess.run(
                ["sh"],
                input="set -eu\n" + FAIL + FUNCTIONS + body,
                text=True,
                capture_output=True,
                env={**os.environ, "TEST_DIR": directory, **env},
                timeout=10,
            )
            return result, calls.read_text().splitlines()

    def test_appview_recovery_decisions(self):
        mocks = r'''
SCW_REGISTRY=registry.test
release_sha=new
backup_service=backup
prepare_rollback() { printf 'old-image-id\n'; }
systemctl() { [ "$1" != is-active ]; }
logger() { :; }
compose() {
    printf '%s\n' "$APPVIEW_IMAGE" >> "$TEST_DIR/calls"
    touch "$TEST_DIR/started"
}
migration_version() {
    if [ -f "$TEST_DIR/started" ]; then value=$AFTER; else value=$BEFORE; fi
    [ "$value" != unknown ] || return 1
    printf '%s\n' "$value"
}
migration_state() {
    if [ -f "$TEST_DIR/started" ]; then printf '%s\n' "$AFTER_STATE";
    else printf '%s\n' "$BEFORE_STATE"; fi
}
wait_for_appview() {
    if [ "$HEALTH" = success ] || [ -f "$TEST_DIR/checked" ]; then return 0; fi
    touch "$TEST_DIR/checked"
    return 1
}
record_release() { printf 'recorded\n' >> "$TEST_DIR/calls"; }
deploy_appview
'''
        for name, overrides, want_calls, message in [
            ("healthy migration", {"AFTER": "58", "HEALTH": "success"},
             ["registry.test/currents-appview:new", "recorded"], None),
            ("safe rollback", {},
             ["registry.test/currents-appview:new", "currents-appview:rollback"], "previous image restored"),
            ("new schema", {"AFTER": "58"},
             ["registry.test/currents-appview:new"], "schema changed from 57 to 58"),
            ("dirty schema", {"AFTER_STATE": "dirty"},
             ["registry.test/currents-appview:new"], "schema migration is dirty"),
            ("unreadable schema", {"AFTER": "unknown"},
             ["registry.test/currents-appview:new"], "automatic rollback was refused"),
            ("unknown state", {"AFTER_STATE": "unknown"},
             ["registry.test/currents-appview:new"], "automatic rollback was refused"),
            ("unreadable initial schema", {"BEFORE": "unknown"}, [], "could not read"),
            ("dirty initial schema", {"BEFORE_STATE": "dirty"}, [], "not clean"),
        ]:
            with self.subTest(name=name):
                env = dict(BEFORE="57", AFTER="57", BEFORE_STATE="clean",
                           AFTER_STATE="clean", HEALTH="fail")
                env.update(overrides)
                result, calls = self.run_shell(mocks, **env)
                self.assertEqual(calls, want_calls, result.stderr)
                if message:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(message, result.stderr)
                else:
                    self.assertEqual(result.returncode, 0, result.stderr)

    def test_public_feed_has_time_to_respond(self):
        result, calls = self.run_shell(r'''
curl() {
    timeout=0
    url=''
    while [ "$#" -gt 0 ]; do
        case "$1" in
            --max-time) shift; timeout=$1 ;;
            https://*) url=$1 ;;
        esac
        shift
    done
    printf '%s %s\n' "$timeout" "$url" >> "$TEST_DIR/calls"
    case "$url" in *getFeed*) [ "$timeout" -ge 7 ] ;; *) return 0 ;; esac
}
public_appview_responds
''')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(calls), 4)
        self.assertTrue(all(call.startswith("5 ") for call in calls[:3]))
        self.assertTrue(calls[3].startswith("30 https://api.currents.is/xrpc/"))

    def test_public_failure_names_the_url(self):
        result, _ = self.run_shell("curl() { return 28; }\npublic_appview_responds\n")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("https://api.currents.is/api/supporter/stats", result.stderr)
        self.assertIn("timeout 5s", result.stderr)

    def test_internal_probes_are_bounded(self):
        result, calls = self.run_shell(r'''
container_id() { printf 'container\n'; }
docker() { printf '%s\n' "$*" >> "$TEST_DIR/calls"; }
appview_responds
frontend_responds
''')
        self.assertEqual(result.returncode, 0, result.stderr)
        probes = [call for call in calls if "wget" in call]
        self.assertEqual(len(probes), 2)
        self.assertTrue(all("wget -T 5 -t 1" in call for call in probes))

    def test_readiness_retries_use_elapsed_time(self):
        for function in ("wait_for_appview", "wait_for_frontend"):
            with self.subTest(function=function):
                result, calls = self.run_shell(r'''
date() {
    clock=0
    if [ -f "$TEST_DIR/clock" ]; then clock=$(cat "$TEST_DIR/clock"); fi
    printf '%s\n' "$((clock + 60))" > "$TEST_DIR/clock"
    printf '%s\n' "$clock"
}
sleep() { :; }
container_running() { printf 'probe\n' >> "$TEST_DIR/calls"; return 1; }
''' + function + "\n")
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(calls, ["probe"])


if __name__ == "__main__":
    unittest.main()
