"""Model-store publish regression tests. Run in the service image:

    docker compose -f docker-compose.dev.yml run --rm --build clustering python -m unittest -v
"""

import os
import unittest
from unittest import mock

os.environ.setdefault("DATABASE_URL", "postgres://unused")  # read at import time

import train_umap


class PublishModelTests(unittest.TestCase):
    def test_noop_without_bucket(self):
        # Dev configuration: no bucket set → no upload, no boto3 needed.
        with mock.patch.object(train_umap, "S3_BUCKET", ""):
            import sys
            with mock.patch.dict(sys.modules, {"boto3": None}):
                train_umap._publish_model()  # would raise if it touched boto3

    def test_uploads_model_object(self):
        client = mock.Mock()
        boto3 = mock.Mock()
        boto3.client.return_value = client
        import sys
        with (
            mock.patch.object(train_umap, "S3_BUCKET", "currents-models"),
            mock.patch.object(train_umap, "S3_ENDPOINT", "https://s3.fr-par.scw.cloud"),
            mock.patch.dict(sys.modules, {"boto3": boto3}),
        ):
            train_umap._publish_model()
        boto3.client.assert_called_once_with("s3", endpoint_url="https://s3.fr-par.scw.cloud")
        client.upload_file.assert_called_once_with(
            train_umap.UMAP_PATH, "currents-models", "umap_model.joblib"
        )

    def test_upload_failure_propagates(self):
        # A failed publish must fail the cron run loudly — the mac mini would
        # otherwise silently keep syncing a stale model.
        client = mock.Mock()
        client.upload_file.side_effect = RuntimeError("bucket unreachable")
        boto3 = mock.Mock()
        boto3.client.return_value = client
        import sys
        with (
            mock.patch.object(train_umap, "S3_BUCKET", "currents-models"),
            mock.patch.dict(sys.modules, {"boto3": boto3}),
        ):
            with self.assertRaises(RuntimeError):
                train_umap._publish_model()


if __name__ == "__main__":
    unittest.main()
