-- Feed-suitability gate: probability (0..1) that the image is junk for the
-- global discovery feed (QR codes, UI screenshots, documents), produced by the
-- feed_junk_head ONNX classifier on the SigLIP2 embedding. NULL = not scored
-- yet (head not deployed / VI predates it) and passes the feed filter — the
-- gate only acts on what the head has actually seen. The threshold lives in Go
-- (feedJunkScoreMax in pgstore.go), so tuning it needs no re-scoring.
ALTER TABLE visual_identity ADD COLUMN junk_score REAL;
