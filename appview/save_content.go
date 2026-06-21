package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	saveContentImageNSID     = "is.currents.content.image"
	saveContentImageViewNSID = "is.currents.content.defs#imageView"
)

type strongRef struct {
	URI string `json:"uri"`
	CID string `json:"cid"`
}

type profileView struct {
	DID         string `json:"did"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
}

type viewerSave struct {
	CollectionURI string `json:"collectionUri"`
	SaveURI       string `json:"saveUri"`
}

type saveViewerState struct {
	Saves       []viewerSave     `json:"saves"`
	Attribution *saveAttribution `json:"attribution,omitempty"`
	Suspected   bool             `json:"suspected,omitempty"`
}

type imageView struct {
	Type          string           `json:"$type"`
	BlobCID       string           `json:"blobCid"`
	ImageURL      string           `json:"imageUrl"`
	Width         int              `json:"width,omitempty"`
	Height        int              `json:"height,omitempty"`
	DominantColor string           `json:"dominantColor,omitempty"`
	Alt           string           `json:"alt,omitempty"`
	Attribution   *saveAttribution `json:"attribution,omitempty"`
}

type saveView struct {
	URI       string           `json:"uri"`
	Author    profileView      `json:"author"`
	Content   any              `json:"content"`
	Text      string           `json:"text,omitempty"`
	OriginURL string           `json:"originUrl,omitempty"`
	ResaveOf  *strongRef       `json:"resaveOf,omitempty"`
	CreatedAt string           `json:"createdAt"`
	Viewer    *saveViewerState `json:"viewer,omitempty"`
	Labels    []labelView      `json:"labels,omitempty"`
}

// labelView is the compact projection of a label record surfaced in XRPC save responses.
// Full label transport (with sig/ver/exp/neg) goes through the labeler's subscribeLabels/queryLabels.
type labelView struct {
	Src string `json:"src"`
	Val string `json:"val"`
	CTS string `json:"cts"`
}

type saveBlobRef struct {
	Type     string            `json:"$type,omitempty"`
	Ref      map[string]string `json:"ref"`
	MimeType string            `json:"mimeType"`
	Size     int               `json:"size,omitempty"`
}

type saveImageContent struct {
	Type        string           `json:"$type"`
	Image       saveBlobRef      `json:"image"`
	Alt         string           `json:"alt,omitempty"`
	Attribution *saveAttribution `json:"attribution,omitempty"`
}

type saveRecord struct {
	Collection struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"collection"`
	Content   json.RawMessage `json:"content"`
	OriginURL string          `json:"originUrl"`
	Text      string          `json:"text"`
	ResaveOf  struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"resaveOf"`
	CreatedAt string      `json:"createdAt"`
	Labels    *selfLabels `json:"labels,omitempty"`
}

// selfLabels is the atproto com.atproto.label.defs#selfLabels shape that creators
// embed in their own records to voluntarily warn viewers.
type selfLabels struct {
	Values []selfLabelValue `json:"values"`
}

type selfLabelValue struct {
	Val string `json:"val"`
}

// allowedSelfLabelVals is the set of labels creators may self-apply to their own
// saves: the four Bluesky-canonical content warnings (which blur) plus the
// Currents AI-generated provenance label (which only badges, never blurs).
// Restricting to this set prevents arbitrary label-value injection via the form.
var allowedSelfLabelVals = map[string]struct{}{
	"porn":                  {},
	"sexual":                {},
	"nudity":                {},
	"graphic-media":         {},
	"currents-ai-generated": {},
}

// selfLabelValsList returns the allowed self-label values as a slice (single
// source of truth: derived from allowedSelfLabelVals). Used by the self-label
// backfill to isolate self-labels from moderator/auto labels.
func selfLabelValsList() []string {
	out := make([]string, 0, len(allowedSelfLabelVals))
	for v := range allowedSelfLabelVals {
		out = append(out, v)
	}
	return out
}

// parseSelfLabels reads a comma-separated form value into a deduplicated list
// of allowed label vals. Unknown vals are silently dropped.
func parseSelfLabels(raw string) []string {
	if raw == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		if _, ok := allowedSelfLabelVals[v]; !ok {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// buildSelfLabelsRecord wraps a list of label vals into the atproto
// com.atproto.label.defs#selfLabels record shape. Returns nil for an empty
// list so the caller can skip adding the field entirely.
func buildSelfLabelsRecord(vals []string) map[string]any {
	if len(vals) == 0 {
		return nil
	}
	values := make([]map[string]any, len(vals))
	for i, v := range vals {
		values[i] = map[string]any{"val": v}
	}
	return map[string]any{
		"$type":  "com.atproto.label.defs#selfLabels",
		"values": values,
	}
}

func rawJSONToAny(raw json.RawMessage) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func saveAttributionOrNil(attribution *saveAttribution) *saveAttribution {
	if attribution == nil {
		return nil
	}
	if attribution.URL == "" && attribution.License == "" && attribution.Credit == "" {
		return nil
	}
	cloned := *attribution
	return &cloned
}

func saveAttributionFromFields(url, license, credit string) *saveAttribution {
	return saveAttributionOrNil(&saveAttribution{
		URL:     url,
		License: license,
		Credit:  credit,
	})
}

func buildImageContentRecordWithAttribution(blob any, attribution *saveAttribution, alt string) map[string]any {
	record := map[string]any{
		"$type": saveContentImageNSID,
		"image": blob,
	}
	if alt != "" {
		record["alt"] = alt
	}
	if attr := saveAttributionOrNil(attribution); attr != nil {
		record["attribution"] = attr
	}
	return record
}

func buildSaveContent(contentRaw json.RawMessage) (any, error) {
	if len(contentRaw) == 0 || string(contentRaw) == "null" {
		return nil, fmt.Errorf("save record missing content")
	}
	return rawJSONToAny(contentRaw)
}

func saveContentNSID(contentRaw json.RawMessage) (string, error) {
	if len(contentRaw) == 0 || string(contentRaw) == "null" {
		return "", fmt.Errorf("save record missing content")
	}
	var content struct {
		Type string `json:"$type"`
	}
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return "", fmt.Errorf("parsing save content: %w", err)
	}
	if content.Type == "" {
		return "", fmt.Errorf("save content missing $type")
	}
	return content.Type, nil
}

func decodeSaveImageContent(contentRaw json.RawMessage) (*saveImageContent, error) {
	contentType, err := saveContentNSID(contentRaw)
	if err != nil {
		return nil, err
	}
	if contentType != saveContentImageNSID {
		return nil, nil
	}
	var content saveImageContent
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		return nil, fmt.Errorf("parsing image content: %w", err)
	}
	return &content, nil
}

// buildSaveContentWithAttribution rebuilds image content with attribution
// applied. When overwrite is false, a nil/empty attribution preserves the
// record's existing attribution (used when editing unrelated fields). When
// overwrite is true, the passed attribution fully replaces it — a nil/empty
// attribution clears it (used by the dedicated attribution endpoint).
func buildSaveContentWithAttribution(contentRaw json.RawMessage, attribution *saveAttribution, overwrite bool) (any, error) {
	content, err := decodeSaveImageContent(contentRaw)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return buildSaveContent(contentRaw)
	}
	if content.Image.Type == "" {
		content.Image.Type = "blob"
	}
	content.Attribution = saveAttributionOrNil(content.Attribution)
	if attr := saveAttributionOrNil(attribution); attr != nil {
		content.Attribution = attr
	} else if overwrite {
		content.Attribution = nil
	}
	return content, nil
}

func parseViewerSaveState(rawSaves, rawAttribution json.RawMessage) *saveViewerState {
	var saves []viewerSave
	if len(rawSaves) > 0 && string(rawSaves) != "null" {
		_ = json.Unmarshal(rawSaves, &saves)
	}
	if saves == nil {
		saves = []viewerSave{}
	}
	state := &saveViewerState{Saves: saves}
	if len(rawAttribution) > 0 && string(rawAttribution) != "null" {
		var attr saveAttribution
		if err := json.Unmarshal(rawAttribution, &attr); err == nil {
			state.Attribution = saveAttributionOrNil(&attr)
		}
	}
	return state
}

func buildSaveView(row SaveRow, author profileView, includeViewer bool, cdnBaseURL string) saveView {
	sv := saveView{
		URI:       row.URI,
		Author:    author,
		Content:   buildSaveContentView(row, cdnBaseURL),
		Text:      row.Text,
		OriginURL: row.OriginURL,
	}
	if row.CreatedAt != nil {
		sv.CreatedAt = row.CreatedAt.UTC().Format(time.RFC3339)
	}
	if row.ResaveOfURI != "" && row.ResaveOfCID != "" {
		sv.ResaveOf = &strongRef{URI: row.ResaveOfURI, CID: row.ResaveOfCID}
	}
	if includeViewer {
		sv.Viewer = parseViewerSaveState(row.ViewerSaves, row.ViewerAttribution)
	}
	return sv
}

// hydrateLabels batch-fetches active labels for the given save views and assigns
// them to each view's Labels field. A no-op when views is empty.
func hydrateLabels(ctx context.Context, store *PgStore, views []saveView) error {
	if len(views) == 0 {
		return nil
	}
	uris := make([]string, len(views))
	for i, v := range views {
		uris[i] = v.URI
	}
	byURI, err := store.GetLabelsByURIs(ctx, uris)
	if err != nil {
		return err
	}
	for i := range views {
		rows := byURI[views[i].URI]
		if len(rows) == 0 {
			continue
		}
		out := make([]labelView, len(rows))
		for j, r := range rows {
			out[j] = labelView{Src: r.Src, Val: r.Val, CTS: r.CTS.UTC().Format(time.RFC3339)}
		}
		views[i].Labels = out
	}
	return nil
}

// hydrateSuspected batch-fetches blob moderation state and sets viewer.suspected
// on any save view whose blob has harm_state='suspected'. Skips views that have
// no viewer state (unauthenticated requests).
func hydrateSuspected(ctx context.Context, store *PgStore, views []saveView) error {
	var cids []string
	for _, v := range views {
		if v.Viewer == nil {
			continue
		}
		if c := extractBlobCID(v); c != "" {
			cids = append(cids, c)
		}
	}
	if len(cids) == 0 {
		return nil
	}
	suspected, err := store.GetSuspectedBlobCIDs(ctx, cids)
	if err != nil {
		return err
	}
	if len(suspected) == 0 {
		return nil
	}
	for i := range views {
		if views[i].Viewer == nil {
			continue
		}
		if c := extractBlobCID(views[i]); suspected[c] {
			views[i].Viewer.Suspected = true
		}
	}
	return nil
}

// extractBlobCID returns the blob CID from a save view's image content, or "".
func extractBlobCID(v saveView) string {
	if img, ok := v.Content.(imageView); ok {
		return img.BlobCID
	}
	if m, ok := v.Content.(map[string]any); ok {
		if c, ok := m["blobCid"].(string); ok {
			return c
		}
	}
	return ""
}

func buildSaveContentView(row SaveRow, cdnBaseURL string) any {
	if row.ContentNSID != saveContentImageNSID {
		return map[string]any{"$type": row.ContentNSID}
	}
	view := imageView{
		Type:        saveContentImageViewNSID,
		BlobCID:     row.BlobCID,
		ImageURL:    cdnBaseURL + "/img/" + row.AuthorDID + "/" + row.BlobCID,
		Alt:         row.AltText,
		Attribution: saveAttributionFromFields(row.AttributionURL, row.AttributionLicense, row.AttributionCredit),
	}
	if row.Width != nil {
		view.Width = *row.Width
	}
	if row.Height != nil {
		view.Height = *row.Height
	}
	view.DominantColor = firstHex(row.DominantColors)
	return view
}
