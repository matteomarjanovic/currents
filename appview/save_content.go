package main

import (
	"encoding/json"
	"fmt"
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
	Saves []viewerSave `json:"saves"`
}

type imageView struct {
	Type          string           `json:"$type"`
	BlobCID       string           `json:"blobCid"`
	ImageURL      string           `json:"imageUrl"`
	Width         int              `json:"width,omitempty"`
	Height        int              `json:"height,omitempty"`
	DominantColor string           `json:"dominantColor,omitempty"`
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
}

type saveBlobRef struct {
	Ref      map[string]string `json:"ref"`
	MimeType string            `json:"mimeType"`
	Size     int               `json:"size,omitempty"`
}

type saveImageContent struct {
	Type        string           `json:"$type"`
	Image       saveBlobRef      `json:"image"`
	Attribution *saveAttribution `json:"attribution,omitempty"`
}

type saveRecord struct {
	Collection struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"collection"`
	Content     json.RawMessage  `json:"content"`
	OriginURL   string           `json:"originUrl"`
	Attribution *saveAttribution `json:"attribution,omitempty"`
	Text        string           `json:"text"`
	ResaveOf    struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"resaveOf"`
	CreatedAt string `json:"createdAt"`
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

func buildImageContentRecord(blob any) map[string]any {
	return buildImageContentRecordWithAttribution(blob, nil)
}

func buildImageContentRecordWithAttribution(blob any, attribution *saveAttribution) map[string]any {
	record := map[string]any{
		"$type": saveContentImageNSID,
		"image": blob,
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

func effectiveSaveAttribution(contentRaw json.RawMessage, legacy *saveAttribution) (*saveAttribution, error) {
	content, err := decodeSaveImageContent(contentRaw)
	if err != nil {
		return nil, err
	}
	if content != nil {
		if attr := saveAttributionOrNil(content.Attribution); attr != nil {
			return attr, nil
		}
	}
	return saveAttributionOrNil(legacy), nil
}

func buildSaveContentWithAttribution(contentRaw json.RawMessage, attribution *saveAttribution, legacy *saveAttribution) (any, error) {
	content, err := decodeSaveImageContent(contentRaw)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return buildSaveContent(contentRaw)
	}
	attr := saveAttributionOrNil(attribution)
	if attr == nil {
		attr = saveAttributionOrNil(content.Attribution)
	}
	if attr == nil {
		attr = saveAttributionOrNil(legacy)
	}
	content.Attribution = attr
	return content, nil
}

func parseViewerSaveState(raw json.RawMessage) *saveViewerState {
	var saves []viewerSave
	if len(raw) > 0 && string(raw) != "null" {
		_ = json.Unmarshal(raw, &saves)
	}
	if saves == nil {
		saves = []viewerSave{}
	}
	return &saveViewerState{Saves: saves}
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
		sv.Viewer = parseViewerSaveState(row.ViewerSaves)
	}
	return sv
}

func buildSaveContentView(row SaveRow, cdnBaseURL string) any {
	if row.ContentNSID != saveContentImageNSID {
		return map[string]any{"$type": row.ContentNSID}
	}
	view := imageView{
		Type:        saveContentImageViewNSID,
		BlobCID:     row.BlobCID,
		ImageURL:    cdnBaseURL + "/img/" + row.AuthorDID + "/" + row.BlobCID,
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
