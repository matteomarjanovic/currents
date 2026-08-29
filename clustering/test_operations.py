import os
import unittest
from datetime import datetime, timezone
from unittest import mock

os.environ.setdefault("DATABASE_URL", "postgres://unused")

import operations


class OperationsJobTests(unittest.TestCase):
    def test_records_terminal_job_run(self):
        conn = mock.MagicMock()
        with mock.patch.object(operations.psycopg2, "connect", return_value=conn) as connect:
            operations.record_job_run(
                "umap_train",
                "success",
                datetime(2026, 1, 1, tzinfo=timezone.utc),
                {"sampled": 12},
            )

        connect.assert_called_once_with(os.environ["DATABASE_URL"])
        conn.commit.assert_called_once()
        query, params = conn.cursor.return_value.__enter__.return_value.execute.call_args.args
        self.assertIn("INSERT INTO operations_job_run", query)
        self.assertEqual(params[:3], ("umap_train", "success", datetime(2026, 1, 1, tzinfo=timezone.utc)))
        self.assertEqual(params[3], '{"sampled": 12}')


if __name__ == "__main__":
    unittest.main()
