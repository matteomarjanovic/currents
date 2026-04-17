package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

func firstHex(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var colors []struct {
		Hex string `json:"hex"`
	}
	if json.Unmarshal(raw, &colors) != nil || len(colors) == 0 {
		return ""
	}
	return colors[0].Hex
}

type saveAttribution struct {
	URL     string `json:"url,omitempty"`
	License string `json:"license,omitempty"`
	Credit  string `json:"credit,omitempty"`
}

func (s *Server) WellKnownDID(w http.ResponseWriter, r *http.Request) {
	doc := map[string]any{
		"@context": []string{"https://www.w3.org/ns/did/v1"},
		"id":       s.ServiceDID,
		"service": []map[string]string{
			{
				"id":              "#atproto_appview",
				"type":            "AtprotoAppView",
				"serviceEndpoint": s.CDNBaseURL,
			},
		},
	}
	w.Header().Set("Content-Type", "application/did+ld+json")
	json.NewEncoder(w).Encode(doc)
}

// optionalAuth returns the viewer DID if the request is authenticated, nil if
// unauthenticated, or an error if auth was attempted but failed (caller should return 401).
func (s *Server) optionalAuth(r *http.Request) (*syntax.DID, error) {
	// Session cookie — first-party web client
	did, _, _ := s.currentSessionDID(r)
	if did != nil {
		return did, nil
	}
	// Bearer inter-service JWT — PDS proxy (atproto-proxy)
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return nil, nil
	}
	scheme, token, ok := strings.Cut(hdr, " ")
	if !ok || scheme != "Bearer" {
		return nil, fmt.Errorf("unsupported auth scheme")
	}
	d, err := s.AuthValidator.Validate(r.Context(), token, nil)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Server) XRPCGetActorCollections(w http.ResponseWriter, r *http.Request) {
	viewerDID, err := s.optionalAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "AuthRequired", "message": err.Error()})
		return
	}

	actorParam := r.URL.Query().Get("actor")
	if actorParam == "" {
		http.Error(w, `{"error":"InvalidRequest","message":"actor is required"}`, http.StatusBadRequest)
		return
	}

	// Resolve actor: DID or handle → DID
	var actorDID syntax.DID
	if parsed, err := syntax.ParseDID(actorParam); err == nil {
		actorDID = parsed
	} else if handle, err := syntax.ParseHandle(actorParam); err == nil {
		ident, err := s.Dir.LookupHandle(r.Context(), handle)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "NotFound", "message": "actor not found"})
			return
		}
		actorDID = ident.DID
	} else {
		http.Error(w, `{"error":"InvalidRequest","message":"invalid actor"}`, http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}
	cursor := r.URL.Query().Get("cursor")

	viewerStr := ""
	if viewerDID != nil {
		viewerStr = viewerDID.String()
	}

	rows, nextCursor, err := s.Store.GetActorCollectionsPage(r.Context(), actorDID.String(), viewerStr, limit, cursor)
	if err != nil {
		slog.Error("GetActorCollectionsPage", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Hydrate actor profile
	type profileView struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName,omitempty"`
		Avatar      string `json:"avatar,omitempty"`
	}
	actor := profileView{DID: actorDID.String()}
	if row, err := s.Store.GetActorByDID(r.Context(), actorDID.String()); err == nil && row != nil {
		actor.Handle = row.Handle
		actor.DisplayName = row.DisplayName
		actor.Avatar = row.Avatar
	} else {
		// Fall back to DID resolution for handle
		if ident, err := s.Dir.LookupDID(r.Context(), actorDID); err == nil {
			actor.Handle = ident.Handle.String()
		}
	}

	type collectionViewerState struct {
		Starred bool `json:"starred"`
	}
	type collectionView struct {
		URI           string                 `json:"uri"`
		CID           string                 `json:"cid"`
		Author        profileView            `json:"author"`
		Name          string                 `json:"name"`
		Description   string                 `json:"description,omitempty"`
		SaveCount     int                    `json:"saveCount,omitempty"`
		PreviewImages []string               `json:"previewImages,omitempty"`
		CreatedAt     string                 `json:"createdAt"`
		Viewer        *collectionViewerState `json:"viewer,omitempty"`
	}

	views := make([]collectionView, 0, len(rows))
	for _, row := range rows {
		cv := collectionView{
			URI:         row.URI,
			CID:         row.CID,
			Author:      actor,
			Name:        row.Name,
			Description: row.Description,
			SaveCount:   row.SaveCount,
		}
		if row.CreatedAt != nil {
			cv.CreatedAt = row.CreatedAt.UTC().Format(time.RFC3339)
		}
		for _, blob := range row.PreviewBlobs {
			parts := strings.SplitN(blob, ",", 2)
			if len(parts) == 2 {
				cv.PreviewImages = append(cv.PreviewImages, s.CDNBaseURL+"/img/"+parts[0]+"/"+parts[1])
			}
		}
		if viewerDID != nil && row.Starred != nil {
			cv.Viewer = &collectionViewerState{Starred: *row.Starred}
		}
		views = append(views, cv)
	}

	type response struct {
		Cursor      string           `json:"cursor,omitempty"`
		Collections []collectionView `json:"collections"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Cursor: nextCursor, Collections: views})
}

func (s *Server) XRPCGetActorProfile(w http.ResponseWriter, r *http.Request) {
	actorParam := r.URL.Query().Get("actor")
	if actorParam == "" {
		http.Error(w, `{"error":"InvalidRequest","message":"actor is required"}`, http.StatusBadRequest)
		return
	}

	var actorDID syntax.DID
	var resolvedHandle string
	if parsed, err := syntax.ParseDID(actorParam); err == nil {
		actorDID = parsed
	} else if handle, err := syntax.ParseHandle(actorParam); err == nil {
		ident, err := s.Dir.LookupHandle(r.Context(), handle)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "NotFound", "message": "actor not found"})
			return
		}
		actorDID = ident.DID
		resolvedHandle = ident.Handle.String()
	} else {
		http.Error(w, `{"error":"InvalidRequest","message":"invalid actor"}`, http.StatusBadRequest)
		return
	}

	type profileView struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName,omitempty"`
		Description string `json:"description,omitempty"`
		Pronouns    string `json:"pronouns,omitempty"`
		Website     string `json:"website,omitempty"`
		Avatar      string `json:"avatar,omitempty"`
		Banner      string `json:"banner,omitempty"`
		CreatedAt   string `json:"createdAt,omitempty"`
	}

	view := profileView{DID: actorDID.String(), Handle: resolvedHandle}
	if row, err := s.Store.GetActorByDID(r.Context(), actorDID.String()); err == nil && row != nil {
		view.Handle = row.Handle
		view.DisplayName = row.DisplayName
		view.Description = row.Description
		view.Pronouns = row.Pronouns
		view.Website = row.Website
		view.Avatar = row.Avatar
		view.Banner = row.Banner
		if row.CreatedAt != nil {
			view.CreatedAt = row.CreatedAt.UTC().Format(time.RFC3339)
		}
	} else if view.Handle == "" {
		if ident, err := s.Dir.LookupDID(r.Context(), actorDID); err == nil {
			view.Handle = ident.Handle.String()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(view)
}

func (s *Server) XRPCGetCollectionSaves(w http.ResponseWriter, r *http.Request) {
	viewerDID, err := s.optionalAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "AuthRequired", "message": err.Error()})
		return
	}

	collectionParam := r.URL.Query().Get("collection")
	if collectionParam == "" {
		http.Error(w, `{"error":"InvalidRequest","message":"collection is required"}`, http.StatusBadRequest)
		return
	}
	if _, err := syntax.ParseATURI(collectionParam); err != nil {
		http.Error(w, `{"error":"InvalidRequest","message":"invalid collection AT-URI"}`, http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}
	cursor := r.URL.Query().Get("cursor")

	viewerStr := ""
	if viewerDID != nil {
		viewerStr = viewerDID.String()
	}

	collRow, err := s.Store.GetCollectionByURI(r.Context(), collectionParam, viewerStr)
	if err != nil {
		slog.Error("GetCollectionByURI", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if collRow == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "NotFound", "message": "collection not found"})
		return
	}

	// Hydrate collection author.
	collATURI, _ := syntax.ParseATURI(collectionParam)
	author := profileView{DID: collATURI.Authority().String()}
	if row, err := s.Store.GetActorByDID(r.Context(), author.DID); err == nil && row != nil {
		author.Handle = row.Handle
		author.DisplayName = row.DisplayName
		author.Avatar = row.Avatar
	} else {
		if did, err := syntax.ParseDID(author.DID); err == nil {
			if ident, err := s.Dir.LookupDID(r.Context(), did); err == nil {
				author.Handle = ident.Handle.String()
			}
		}
	}

	type collectionViewerState struct {
		Starred bool `json:"starred"`
	}
	type collectionView struct {
		URI           string                 `json:"uri"`
		CID           string                 `json:"cid"`
		Author        profileView            `json:"author"`
		Name          string                 `json:"name"`
		Description   string                 `json:"description,omitempty"`
		SaveCount     int                    `json:"saveCount,omitempty"`
		PreviewImages []string               `json:"previewImages,omitempty"`
		CreatedAt     string                 `json:"createdAt"`
		Viewer        *collectionViewerState `json:"viewer,omitempty"`
	}
	cv := collectionView{
		URI:         collRow.URI,
		CID:         collRow.CID,
		Author:      author,
		Name:        collRow.Name,
		Description: collRow.Description,
		SaveCount:   collRow.SaveCount,
	}
	if collRow.CreatedAt != nil {
		cv.CreatedAt = collRow.CreatedAt.UTC().Format(time.RFC3339)
	}
	for _, blob := range collRow.PreviewBlobs {
		parts := strings.SplitN(blob, ",", 2)
		if len(parts) == 2 {
			cv.PreviewImages = append(cv.PreviewImages, s.CDNBaseURL+"/img/"+parts[0]+"/"+parts[1])
		}
	}
	if viewerDID != nil && collRow.Starred != nil {
		cv.Viewer = &collectionViewerState{Starred: *collRow.Starred}
	}

	saveRows, nextCursor, err := s.Store.GetSavesPage(r.Context(), collectionParam, viewerStr, limit, cursor)
	if err != nil {
		slog.Error("GetSavesPage", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	views := make([]saveView, 0, len(saveRows))
	for _, row := range saveRows {
		views = append(views, buildSaveView(row, author, viewerDID != nil, s.CDNBaseURL))
	}

	type response struct {
		Collection collectionView `json:"collection"`
		Cursor     string         `json:"cursor,omitempty"`
		Saves      []saveView     `json:"saves"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Collection: cv, Cursor: nextCursor, Saves: views})
}

func (s *Server) XRPCSearchSaves(w http.ResponseWriter, r *http.Request) {
	viewerDID, err := s.optionalAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "AuthRequired", "message": err.Error()})
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, `{"error":"InvalidRequest","message":"q is required"}`, http.StatusBadRequest)
		return
	}

	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}

	offset := 0
	if c := r.URL.Query().Get("cursor"); c != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(c); err == nil {
			if n, err := strconv.Atoi(string(raw)); err == nil && n > 0 {
				offset = n
			}
		}
	}

	embedding, err := s.Inference.EmbedText(r.Context(), q)
	if err != nil {
		slog.Error("EmbedText", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	viewerStr := ""
	if viewerDID != nil {
		viewerStr = viewerDID.String()
	}

	excludeSaved := r.URL.Query().Get("excludeSaved") == "true" && viewerDID != nil
	saveRows, err := s.Store.SearchSavesByEmbedding(r.Context(), embedding, viewerStr, excludeSaved, limit, offset)
	if err != nil {
		slog.Error("SearchSavesByEmbedding", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Hydrate author profiles (deduplicated).
	authorCache := map[string]profileView{}
	for _, row := range saveRows {
		if _, ok := authorCache[row.AuthorDID]; ok {
			continue
		}
		pv := profileView{DID: row.AuthorDID}
		if actorRow, err := s.Store.GetActorByDID(r.Context(), row.AuthorDID); err == nil && actorRow != nil {
			pv.Handle = actorRow.Handle
			pv.DisplayName = actorRow.DisplayName
			pv.Avatar = actorRow.Avatar
		}
		authorCache[row.AuthorDID] = pv
	}

	views := make([]saveView, 0, len(saveRows))
	for _, row := range saveRows {
		views = append(views, buildSaveView(row, authorCache[row.AuthorDID], viewerDID != nil, s.CDNBaseURL))
	}

	var nextCursor string
	if len(saveRows) == limit {
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset + limit)))
	}

	type response struct {
		Cursor string     `json:"cursor,omitempty"`
		Saves  []saveView `json:"saves"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Cursor: nextCursor, Saves: views})
}

func (s *Server) XRPCGetRelatedSaves(w http.ResponseWriter, r *http.Request) {
	viewerDID, err := s.optionalAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "AuthRequired", "message": err.Error()})
		return
	}

	uri := r.URL.Query().Get("uri")
	if uri == "" {
		http.Error(w, `{"error":"InvalidRequest","message":"uri is required"}`, http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}

	offset := 0
	if c := r.URL.Query().Get("cursor"); c != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(c); err == nil {
			if n, err := strconv.Atoi(string(raw)); err == nil && n > 0 {
				offset = n
			}
		}
	}

	viewerStr := ""
	if viewerDID != nil {
		viewerStr = viewerDID.String()
	}

	saveRows, err := s.Store.GetRelatedSavesByURI(r.Context(), uri, viewerStr, limit, offset)
	if err != nil {
		slog.Error("GetRelatedSavesByURI", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authorCache := map[string]profileView{}
	for _, row := range saveRows {
		if _, ok := authorCache[row.AuthorDID]; ok {
			continue
		}
		pv := profileView{DID: row.AuthorDID}
		if actorRow, err := s.Store.GetActorByDID(r.Context(), row.AuthorDID); err == nil && actorRow != nil {
			pv.Handle = actorRow.Handle
			pv.DisplayName = actorRow.DisplayName
			pv.Avatar = actorRow.Avatar
		}
		authorCache[row.AuthorDID] = pv
	}

	views := make([]saveView, 0, len(saveRows))
	for _, row := range saveRows {
		views = append(views, buildSaveView(row, authorCache[row.AuthorDID], viewerDID != nil, s.CDNBaseURL))
	}

	var nextCursor string
	if len(saveRows) == limit {
		nextCursor = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset + limit)))
	}

	type response struct {
		Cursor string     `json:"cursor,omitempty"`
		Saves  []saveView `json:"saves"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Cursor: nextCursor, Saves: views})
}

func (s *Server) XRPCGetFeed(w http.ResponseWriter, r *http.Request) {
	viewerDID, err := s.optionalAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "AuthRequired", "message": err.Error()})
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}

	cursorState := feedCursor{Version: 1}
	if c := r.URL.Query().Get("cursor"); c != "" {
		if decoded, err := decodeFeedCursor(c); err == nil {
			cursorState = decoded
		}
	}

	alpha := 0.0
	if p := r.URL.Query().Get("personalized"); p != "" {
		if f, err := strconv.ParseFloat(p, 64); err == nil {
			alpha = max(0.0, min(f, 1.0))
		}
	}

	viewerStr := ""
	if viewerDID != nil {
		viewerStr = viewerDID.String()
	} else {
		alpha = 0
	}

	excludeSaved := r.URL.Query().Get("excludeSaved") == "true" && viewerDID != nil

	var saveRows []SaveRow
	nextCursor := ""

	if alpha > 0 {
		strictPersonalized := alpha >= 1.0
		fetchLimit := feedPoolFetchLimit(limit)

		offsets := make(map[string]int, len(cursorState.Collections))
		for _, col := range cursorState.Collections {
			offsets[col.URI] = col.Offset
		}

		personalizedPools := make([]*feedCandidatePool, 0, feedPersonalizedPoolCount)
		appendPersonalizedPool := func(col CollectionImportance, offset int) error {
			candidates, err := s.Store.SearchSavesByEmbedding(r.Context(), col.Embedding, viewerStr, excludeSaved, fetchLimit+1, offset)
			if err != nil {
				return err
			}
			more := len(candidates) > fetchLimit
			if more {
				candidates = candidates[:fetchLimit]
			}
			if len(candidates) == 0 {
				return nil
			}
			personalizedPools = append(personalizedPools, &feedCandidatePool{
				URI:    col.URI,
				Offset: offset,
				Items:  candidates,
				More:   more,
			})
			return nil
		}

		if len(cursorState.Collections) > 0 {
			uris := make([]string, 0, len(cursorState.Collections))
			for _, col := range cursorState.Collections {
				uris = append(uris, col.URI)
			}
			cols, err := s.Store.GetCollectionsByURIs(r.Context(), uris)
			if err != nil {
				slog.Error("GetCollectionsByURIs", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			for _, col := range cols {
				if err := appendPersonalizedPool(col, offsets[col.URI]); err != nil {
					slog.Error("SearchSavesByEmbedding (feed)", "collection", col.URI, "err", err)
					continue
				}
			}
		}

		if len(cursorState.Collections) == 0 {
			cols, err := s.Store.GetCollectionsByImportance(r.Context(), viewerStr, feedPersonalizedPoolCount)
			if err != nil {
				slog.Error("GetCollectionsByImportance", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			for _, col := range cols {
				if err := appendPersonalizedPool(col, 0); err != nil {
					slog.Error("SearchSavesByEmbedding (feed)", "collection", col.URI, "err", err)
					continue
				}
			}
		}

		if len(personalizedPools) > 0 {
			colWeight := alpha / float64(len(personalizedPools))
			for _, pool := range personalizedPools {
				pool.Weight = colWeight
			}
		}

		pools := make([]*feedCandidatePool, 0, len(personalizedPools)+1)
		pools = append(pools, personalizedPools...)

		globalWeight := 1.0 - alpha
		useGlobal := globalWeight > 0
		if len(personalizedPools) == 0 && !strictPersonalized {
			globalWeight = 1.0
			useGlobal = true
		}

		if useGlobal {
			global, err := s.Store.GetGlobalFeedSaves(r.Context(), viewerStr, excludeSaved, fetchLimit+1, cursorState.GlobalOffset)
			if err != nil {
				slog.Error("GetGlobalFeedSaves", "err", err)
			} else {
				more := len(global) > fetchLimit
				if more {
					global = global[:fetchLimit]
				}
				pools = append(pools, &feedCandidatePool{
					Weight: globalWeight,
					Offset: cursorState.GlobalOffset,
					Items:  global,
					More:   more,
				})
			}
		}

		saveRows = buildFeedPage(nil, pools, limit)

		nextState := feedCursor{Version: 1}
		for _, pool := range pools {
			if !pool.hasMoreAfterPage() {
				continue
			}
			if pool.URI == "" {
				nextState.GlobalOffset = pool.nextOffset()
				continue
			}
			nextState.Collections = append(nextState.Collections, feedCursorCollection{URI: pool.URI, Offset: pool.nextOffset()})
		}
		if len(nextState.Collections) > 0 || nextState.GlobalOffset > 0 {
			nextCursor, err = encodeFeedCursor(nextState)
			if err != nil {
				slog.Error("encodeFeedCursor", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	} else {
		fetchLimit := limit + 1
		pool := &feedCandidatePool{Weight: 1, Offset: cursorState.GlobalOffset}
		pool.Items, err = s.Store.GetGlobalFeedSaves(r.Context(), viewerStr, excludeSaved, fetchLimit, cursorState.GlobalOffset)
		if err != nil {
			slog.Error("GetGlobalFeedSaves", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(pool.Items) > limit {
			pool.More = true
			pool.Items = pool.Items[:limit]
		}
		saveRows = buildFeedPage(nil, []*feedCandidatePool{pool}, limit)
		if pool.hasMoreAfterPage() {
			nextCursor, err = encodeFeedCursor(feedCursor{Version: 1, GlobalOffset: pool.nextOffset()})
			if err != nil {
				slog.Error("encodeFeedCursor", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
	}

	authorCache := map[string]profileView{}
	for _, row := range saveRows {
		if _, ok := authorCache[row.AuthorDID]; ok {
			continue
		}
		pv := profileView{DID: row.AuthorDID}
		if actorRow, err := s.Store.GetActorByDID(r.Context(), row.AuthorDID); err == nil && actorRow != nil {
			pv.Handle = actorRow.Handle
			pv.DisplayName = actorRow.DisplayName
			pv.Avatar = actorRow.Avatar
		}
		authorCache[row.AuthorDID] = pv
	}

	views := make([]saveView, 0, len(saveRows))
	for _, row := range saveRows {
		views = append(views, buildSaveView(row, authorCache[row.AuthorDID], viewerDID != nil, s.CDNBaseURL))
	}

	type response struct {
		Cursor string     `json:"cursor,omitempty"`
		Feed   []saveView `json:"feed"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Cursor: nextCursor, Feed: views})
}

func (s *Server) XRPCGetCollections(w http.ResponseWriter, r *http.Request) {
	did := r.Context().Value("did").(syntax.DID)

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = max(1, min(n, 100))
		}
	}
	cursor := r.URL.Query().Get("cursor")

	rows, nextCursor, err := s.Store.GetCollectionsPage(r.Context(), did.String(), limit, cursor)
	if err != nil {
		slog.Error("GetCollectionsPage", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	type collectionView struct {
		URI           string   `json:"uri"`
		CID           string   `json:"cid"`
		Name          string   `json:"name"`
		Description   string   `json:"description,omitempty"`
		SaveCount     int      `json:"saveCount,omitempty"`
		PreviewImages []string `json:"previewImages,omitempty"`
		CreatedAt     string   `json:"createdAt"`
	}

	views := make([]collectionView, 0, len(rows))
	for _, row := range rows {
		cv := collectionView{
			URI:         row.URI,
			CID:         row.CID,
			Name:        row.Name,
			Description: row.Description,
			SaveCount:   row.SaveCount,
		}
		if row.CreatedAt != nil {
			cv.CreatedAt = row.CreatedAt.UTC().Format(time.RFC3339)
		}
		for _, blob := range row.PreviewBlobs {
			parts := strings.SplitN(blob, ",", 2)
			if len(parts) == 2 {
				cv.PreviewImages = append(cv.PreviewImages, s.CDNBaseURL+"/img/"+parts[0]+"/"+parts[1])
			}
		}
		views = append(views, cv)
	}

	type response struct {
		Cursor      string           `json:"cursor,omitempty"`
		Collections []collectionView `json:"collections"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Cursor: nextCursor, Collections: views})
}

func (s *Server) XRPCGetSaves(w http.ResponseWriter, r *http.Request) {
	viewerDID, err := s.optionalAuth(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "AuthRequired", "message": err.Error()})
		return
	}

	uris := r.URL.Query()["uris"]
	if len(uris) == 0 {
		http.Error(w, `{"error":"InvalidRequest","message":"uris is required"}`, http.StatusBadRequest)
		return
	}
	if len(uris) > 25 {
		http.Error(w, `{"error":"InvalidRequest","message":"at most 25 uris allowed"}`, http.StatusBadRequest)
		return
	}
	for _, u := range uris {
		if _, err := syntax.ParseATURI(u); err != nil {
			http.Error(w, `{"error":"InvalidRequest","message":"invalid at-uri"}`, http.StatusBadRequest)
			return
		}
	}

	viewerStr := ""
	if viewerDID != nil {
		viewerStr = viewerDID.String()
	}

	saveRows, err := s.Store.GetSavesByURIs(r.Context(), uris, viewerStr)
	if err != nil {
		slog.Error("GetSavesByURIs", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	authorCache := map[string]profileView{}
	for _, row := range saveRows {
		if _, ok := authorCache[row.AuthorDID]; ok {
			continue
		}
		pv := profileView{DID: row.AuthorDID}
		if actorRow, err := s.Store.GetActorByDID(r.Context(), row.AuthorDID); err == nil && actorRow != nil {
			pv.Handle = actorRow.Handle
			pv.DisplayName = actorRow.DisplayName
			pv.Avatar = actorRow.Avatar
		}
		authorCache[row.AuthorDID] = pv
	}

	byURI := map[string]saveView{}
	for _, row := range saveRows {
		byURI[row.URI] = buildSaveView(row, authorCache[row.AuthorDID], viewerDID != nil, s.CDNBaseURL)
	}

	views := make([]saveView, 0, len(saveRows))
	for _, u := range uris {
		if sv, ok := byURI[u]; ok {
			views = append(views, sv)
		}
	}

	type response struct {
		Saves []saveView `json:"saves"`
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response{Saves: views})
}
