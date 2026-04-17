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
	Type          string `json:"$type"`
	BlobCID       string `json:"blobCid"`
	ImageURL      string `json:"imageUrl"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	DominantColor string `json:"dominantColor,omitempty"`
}

type saveView struct {
	URI         string           `json:"uri"`
	Author      profileView      `json:"author"`
	Content     any              `json:"content"`
	Text        string           `json:"text,omitempty"`
	OriginURL   string           `json:"originUrl,omitempty"`
	Attribution *saveAttribution `json:"attribution,omitempty"`
	ResaveOf    *strongRef       `json:"resaveOf,omitempty"`
	CreatedAt   string           `json:"createdAt"`
	Viewer      *saveViewerState `json:"viewer,omitempty"`
}

type saveBlobRef struct {
	Ref      map[string]string `json:"ref"`
	MimeType string            `json:"mimeType"`
	Size     int               `json:"size,omitempty"`
}

type saveImageContent struct {
	Type  string      `json:"$type"`
	Image saveBlobRef `json:"image"`
}

type saveRecord struct {
	Collection struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	} `json:"collection"`
	Content     json.RawMessage `json:"content"`
	Image       json.RawMessage `json:"image"`
	OriginURL   string          `json:"originUrl"`
	Attribution struct {
		URL     string `json:"url"`
		License string `json:"license"`
		Credit  string `json:"credit"`
	} `json:"attribution"`
	Text     string `json:"text"`
	ResaveOf struct {
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

func buildImageContentRecord(blob any) map[string]any {
	return map[string]any{
		"$type": saveContentImageNSID,
		"image": blob,
	}
}

func buildSaveContent(contentRaw, legacyImageRaw json.RawMessage) (any, error) {
	if len(contentRaw) > 0 && string(contentRaw) != "null" {
		return rawJSONToAny(contentRaw)
	}
	if len(legacyImageRaw) > 0 && string(legacyImageRaw) != "null" {
		blob, err := rawJSONToAny(legacyImageRaw)
		if err != nil {
			return nil, err
		}
		return buildImageContentRecord(blob), nil
	}
	return nil, fmt.Errorf("save record missing content")
}

func saveContentNSID(contentRaw, legacyImageRaw json.RawMessage) (string, error) {
	if len(contentRaw) > 0 && string(contentRaw) != "null" {
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
	if len(legacyImageRaw) > 0 && string(legacyImageRaw) != "null" {
		return saveContentImageNSID, nil
	}
	return "", fmt.Errorf("save record missing content")
}

func decodeSaveImageContent(contentRaw, legacyImageRaw json.RawMessage) (*saveImageContent, error) {
	contentType, err := saveContentNSID(contentRaw, legacyImageRaw)
	if err != nil {
		return nil, err
	}
	if contentType != saveContentImageNSID {
		return nil, nil
	}
	if len(contentRaw) > 0 && string(contentRaw) != "null" {
		var content saveImageContent
		if err := json.Unmarshal(contentRaw, &content); err != nil {
			return nil, fmt.Errorf("parsing image content: %w", err)
		}
		return &content, nil
	}

	var legacy saveBlobRef
	if err := json.Unmarshal(legacyImageRaw, &legacy); err != nil {
		return nil, fmt.Errorf("parsing legacy image content: %w", err)
	}
	return &saveImageContent{Type: saveContentImageNSID, Image: legacy}, nil
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
	if row.AttributionURL != "" || row.AttributionLicense != "" || row.AttributionCredit != "" {
		sv.Attribution = &saveAttribution{URL: row.AttributionURL, License: row.AttributionLicense, Credit: row.AttributionCredit}
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
		Type:     saveContentImageViewNSID,
		BlobCID:  row.BlobCID,
		ImageURL: cdnBaseURL + "/img/" + row.AuthorDID + "/" + row.BlobCID,
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
