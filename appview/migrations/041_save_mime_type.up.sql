-- Blob mime type from the save record (e.g. "image/gif"), surfaced in XRPC image
-- views so the web client can freeze animated GIFs at their first frame. NULL for
-- existing rows and any save whose record omits it; the client treats "" as
-- "not a GIF" and renders normally.
ALTER TABLE save ADD COLUMN mime_type TEXT;
