import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("backup-postgres.sh").read_text()
ARCHIVE = b"PGDMP-test-archive\n"
DOCKER = r'''#!/usr/bin/env python3
import json, os, pathlib, shutil, sys
a = sys.argv[1:]
with open(os.environ["CALLS"], "a") as f:
    f.write(json.dumps(a) + "\n")
stage = os.environ.get("FAIL_STAGE", "")
if a[0] == "exec":
    command = a[a.index("currents-db-1") + 1:]
    if command[0] == "pg_dump":
        assert "--format=custom" in command
        assert "--compress=zstd:1" in command
        if stage == "dump":
            sys.exit("compression failed")
        sys.stdout.buffer.write(b"PGDMP-test-archive\n")
    elif command[0] == "pg_restore":
        assert sys.stdin.buffer.read() == b"PGDMP-test-archive\n"
        if stage == "validate":
            sys.exit("archive invalid")
    elif command[0] == "psql":
        sys.stdin.read()
    else:
        raise AssertionError(command)
else:
    assert a[0] == "run"
    i = a.index("cp")
    source, destination = a[i + 1:i + 3]
    checksum = source.endswith(".sha256")
    if stage == ("checksum" if checksum else "upload"):
        sys.exit("upload failed")
    name = pathlib.Path(source).name
    shutil.copyfile(pathlib.Path(os.environ["BACKUP_DIR"]) / name,
                    pathlib.Path(os.environ["CLOUD_DIR"]) / name)
'''


class BackupPostgresTests(unittest.TestCase):
    def run_backup(self, stage=""):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            data = root / "data"
            data.mkdir()
            cloud = root / "cloud"
            cloud.mkdir()
            binaries = root / "bin"
            binaries.mkdir()
            for name, source in {
                "docker": DOCKER,
                "mountpoint": "#!/bin/sh\nexit 0\n",
                "sha256sum": (
                    "#!/usr/bin/env python3\nimport hashlib,sys\n"
                    "for p in sys.argv[1:]:\n"
                    " print(hashlib.sha256(open(p,'rb').read()).hexdigest()+'  '+p)\n"
                ),
            }.items():
                path = binaries / name
                path.write_text(source)
                path.chmod(0o755)
            storage = root / "storage.env"
            storage.write_text("S3_ACCESS_KEY=test\nS3_SECRET_KEY=test\nBACKUPS_S3_BUCKET=test\n")
            calls = root / "calls"
            script = SCRIPT.replace("/mnt/pgdata", str(data)).replace(
                "/opt/currents/.env.storage", str(storage)
            )
            result = subprocess.run(
                ["sh"], input=script, text=True, capture_output=True, timeout=10,
                env={**os.environ, "PATH": str(binaries) + os.pathsep + os.environ["PATH"],
                     "CALLS": str(calls), "BACKUP_DIR": str(data / "currents-backups"),
                     "CLOUD_DIR": str(cloud), "FAIL_STAGE": stage},
            )
            return (result, [json.loads(line) for line in calls.read_text().splitlines()],
                    {p.name: p.read_bytes() for p in cloud.iterdir()},
                    list((data / "currents-backups").iterdir()))

    def test_validated_zstd_archive_and_checksum_uploaded_before_success(self):
        result, calls, cloud, remaining = self.run_backup()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Uploaded", result.stdout)
        self.assertEqual(len(cloud), 2)
        name = next(name for name in cloud if name.endswith(".dump"))
        self.assertEqual(cloud[name], ARCHIVE)
        self.assertEqual(cloud[name + ".sha256"].decode().strip(),
                         hashlib.sha256(ARCHIVE).hexdigest() + "  " + name)
        self.assertIn("pg_restore", calls[1])
        self.assertEqual([a[0] for a in calls], ["exec", "exec", "run", "run", "exec"])
        self.assertIn("run_status=success", calls[-1])
        details = next(a.removeprefix("details=") for a in calls[-1] if a.startswith("details="))
        self.assertEqual(json.loads(details), {"name": name, "bytes": len(ARCHIVE)})
        self.assertEqual(remaining, [])

    def test_failure_never_records_success_and_removes_local_archive(self):
        for stage, uploaded in [("dump", 0), ("validate", 0), ("upload", 0), ("checksum", 1)]:
            with self.subTest(stage=stage):
                result, calls, cloud, remaining = self.run_backup(stage)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("Uploaded", result.stdout)
                self.assertIn("run_status=failed", calls[-1])
                self.assertEqual(len(cloud), uploaded)
                self.assertEqual(remaining, [])
