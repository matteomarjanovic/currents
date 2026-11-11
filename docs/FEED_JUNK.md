# Feed junk gate — how to turn it on

The global discovery feed excludes content-type junk (QR codes, UI screenshots,
scanned documents) via a binary classification head on the existing SigLIP2
embeddings. Everything ships dormant: until the model file exists, nothing is
scored and the feed is unchanged.

## Steps

1. **Train the head**: run `moderation/04_feed_junk_head.ipynb` on Google Colab
   (T4 GPU). It streams public datasets (Rico screenshots, RVL-CDIP documents,
   synthetic QR codes vs COCO/WikiArt), trains the MLP, and saves
   `feed_junk_head.onnx` to your Drive (`currents_moderation/`). Check the
   per-source accuracy table in the eval cell — screenshots/documents/QR should
   each be near-perfect before deploying.
2. **Deploy the model**: copy `feed_junk_head.onnx` into the inference server's
   models dir (`inference/models/` by default, or point `FEED_JUNK_HEAD_ONNX`
   at it) and restart the server. `GET /health` must show `"junk_head": true`.
   From now on every newly indexed image is scored automatically.
3. **Backfill existing images**:
   `appview backfill-feed-junk --dry-run` first — it logs the first batch's
   scores with a `would_filter` flag so you can sanity-check against real
   images. Then run it without `--dry-run`. It scores stored embeddings via
   `/classify/junk/embeddings` (no blob fetches, no GPU); resumable, Ctrl+C-safe.
4. **Tune the threshold** if needed: `feedJunkScoreMax` in `appview/pgstore.go`
   (default 0.5, higher = more permissive). Scores stay in the DB, so tuning is
   just a constant change + appview rebuild — no re-scoring. Unscored images
   (`junk_score IS NULL`) always pass; the gate applies to the global feed
   only — library, search, and profiles never hide anything.

## Later

Retrain on Currents' own data by swapping the dataset cells in the same
notebook (the zero-shot scores from the first model make hand-labeling fast:
review the borderline band instead of everything). A graded aesthetic scorer
for *ranking* legitimate images is a separate, future head — see the feed
ranking rework.
