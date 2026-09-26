package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/bluesky-social/indigo/atproto/atclient"
)

const repairTestDID = "did:plc:repairtest"

func collectionTestURI(key string) string {
	return "at://" + repairTestDID + "/" + collectionNSID + "/" + key
}
func saveTestURI(key string) string { return "at://" + repairTestDID + "/" + saveNSID + "/" + key }
func testRepositoryRecord(uri, value string) repositoryRecord {
	var v map[string]json.RawMessage
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		panic(err)
	}
	return repositoryRecord{URI: uri, CID: "cid-" + rkeyFromURI(uri), Value: v}
}

// A stateful PDS fixture enforces commit preconditions and pagination. It lets
// us exercise retries without relying on live accounts or simplified stores.
type maintenancePDS struct {
	records          map[string]repositoryRecord
	revision         int
	failPage         bool
	changeDuringRead bool
	failBatch        int
	batches          int
	calls            []string
}

func newMaintenancePDS(t *testing.T, records ...repositoryRecord) (*maintenancePDS, *atclient.APIClient) {
	t.Helper()
	p := &maintenancePDS{records: map[string]repositoryRecord{}}
	for _, r := range records {
		p.records[r.URI] = r
	}
	srv := httptest.NewServer(http.HandlerFunc(p.serveHTTP))
	t.Cleanup(srv.Close)
	return p, atclient.NewAPIClient(srv.URL)
}
func (p *maintenancePDS) serveHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fail := func(code int, name string) {
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(map[string]string{"error": name})
	}
	switch r.URL.Path {
	case "/xrpc/com.atproto.sync.getLatestCommit":
		json.NewEncoder(w).Encode(map[string]string{"cid": fmt.Sprintf("commit-%d", p.revision)})
	case "/xrpc/com.atproto.repo.listRecords":
		start, _ := strconv.Atoi(r.URL.Query().Get("cursor"))
		if p.failPage && start > 0 {
			fail(503, "Unavailable")
			return
		}
		var keys []string
		prefix := "at://" + repairTestDID + "/" + r.URL.Query().Get("collection") + "/"
		for uri := range p.records {
			if len(uri) >= len(prefix) && uri[:len(prefix)] == prefix {
				keys = append(keys, uri)
			}
		}
		slices.Sort(keys)
		end := min(start+100, len(keys))
		page := []repositoryRecord{}
		for _, uri := range keys[start:end] {
			page = append(page, p.records[uri])
		}
		cursor := ""
		if end < len(keys) {
			cursor = strconv.Itoa(end)
		}
		json.NewEncoder(w).Encode(map[string]any{"records": page, "cursor": cursor})
		if p.changeDuringRead {
			p.revision++
		}
	case "/xrpc/com.atproto.repo.getRecord":
		uri := "at://" + r.URL.Query().Get("repo") + "/" + r.URL.Query().Get("collection") + "/" + r.URL.Query().Get("rkey")
		record, ok := p.records[uri]
		if !ok {
			fail(400, "RecordNotFound")
			return
		}
		json.NewEncoder(w).Encode(record)
	case "/xrpc/com.atproto.repo.applyWrites":
		var body struct {
			Repo       string `json:"repo"`
			SwapCommit string `json:"swapCommit"`
			Writes     []struct {
				Type       string                     `json:"$type"`
				Collection string                     `json:"collection"`
				Rkey       string                     `json:"rkey"`
				Value      map[string]json.RawMessage `json:"value"`
			} `json:"writes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			fail(400, "InvalidRequest")
			return
		}
		p.batches++
		if p.batches == p.failBatch {
			fail(429, "RateLimitExceeded")
			return
		}
		if body.SwapCommit != fmt.Sprintf("commit-%d", p.revision) {
			fail(400, "InvalidSwap")
			return
		}
		for _, wr := range body.Writes {
			uri := "at://" + body.Repo + "/" + wr.Collection + "/" + wr.Rkey
			p.calls = append(p.calls, uri)
			if wr.Type == "com.atproto.repo.applyWrites#delete" {
				delete(p.records, uri)
			} else {
				p.records[uri] = repositoryRecord{URI: uri, CID: "new-cid", Value: wr.Value}
			}
		}
		p.revision++
		json.NewEncoder(w).Encode(map[string]any{"commit": map[string]string{"cid": fmt.Sprintf("commit-%d", p.revision)}})
	default:
		fail(404, "NotFound")
	}
}

func TestOrphanRepairPreservesRecordsAndIsIdempotent(t *testing.T) {
	root := collectionTestURI("root")
	section := collectionTestURI("section")
	missing := collectionTestURI("gone")
	records := []repositoryRecord{
		testRepositoryRecord(root, `{"name":"Root","createdAt":"2026-01-01T00:00:00Z"}`),
		testRepositoryRecord(section, fmt.Sprintf(`{"name":"Section","parent":{"uri":%q,"cid":"old"},"unknown":{"number":9007199254740993},"createdAt":"2026-01-01T00:00:00Z"}`, missing)),
		testRepositoryRecord(saveTestURI("in-section"), fmt.Sprintf(`{"collection":{"uri":%q},"content":{"image":{"ref":{"$link":"blob"}}},"labels":{"values":[{"val":"nudity"}]}}`, section)),
		testRepositoryRecord(saveTestURI("orphan"), fmt.Sprintf(`{"collection":{"uri":%q},"content":{"image":{"ref":{"$link":"blob"}},"alt":"description"},"createdAt":"2026-01-01T00:00:00Z","text":"note","resaveOf":{"uri":"at://source/post/1"},"labels":{"values":[{"val":"nudity"}]},"custom":[1,2]}`, missing)),
		testRepositoryRecord(saveTestURI("unsorted"), `{"content":{"alt":"already unsorted"}}`),
		testRepositoryRecord(saveTestURI("foreign"), `{"collection":{"uri":"at://did:plc:other/is.currents.feed.collection/root"}}`),
	}
	p, c := newMaintenancePDS(t, records...)
	ctx := context.Background()
	snap, err := readRepository(ctx, c, repairTestDID)
	if err != nil {
		t.Fatal(err)
	}
	repairs := findOrphans(repairTestDID, snap)
	if len(repairs) != 2 || repairs[0].Field != "parent" || repairs[1].Field != "collection" {
		t.Fatalf("unexpected repairs: %+v", repairs)
	}
	writes := []map[string]any{repairs[0].write(), repairs[1].write()}
	if _, err := applyRepositoryWrites(ctx, c, repairTestDID, snap.Commit, writes); err != nil {
		t.Fatal(err)
	}
	if len(p.records) != len(records) {
		t.Fatal("repair created or deleted records")
	}
	for _, before := range records {
		after := p.records[before.URI]
		for field, value := range before.Value {
			if before.URI == section && field == "parent" || before.URI == saveTestURI("orphan") && field == "collection" {
				if _, ok := after.Value[field]; ok {
					t.Fatal("broken reference remains")
				}
				continue
			}
			if string(after.Value[field]) != string(value) {
				t.Fatalf("changed %s on %s", field, before.URI)
			}
		}
	}
	snap, err = readRepository(ctx, c, repairTestDID)
	if err != nil {
		t.Fatal(err)
	}
	if got := findOrphans(repairTestDID, snap); len(got) != 0 {
		t.Fatalf("second pass: %+v", got)
	}
	if _, ok := records[1].Value["parent"]; !ok {
		t.Fatal("planning mutated original record")
	}
}

func TestRepositorySnapshotRefusesPartialOrChangingReads(t *testing.T) {
	var records []repositoryRecord
	for i := 0; i < 105; i++ {
		records = append(records, testRepositoryRecord(collectionTestURI(fmt.Sprint(i)), `{"name":"Collection"}`))
	}
	p, c := newMaintenancePDS(t, records...)
	snap, err := readRepository(context.Background(), c, repairTestDID)
	if err != nil || len(snap.Collections) != 105 {
		t.Fatalf("pagination: %d, %v", len(snap.Collections), err)
	}
	p.failPage = true
	if _, err := readRepository(context.Background(), c, repairTestDID); err == nil {
		t.Fatal("partial enumeration accepted")
	}
	p.failPage = false
	p.changeDuringRead = true
	if _, err := readRepository(context.Background(), c, repairTestDID); err == nil {
		t.Fatal("changing repository accepted")
	}
	if len(p.calls) != 0 {
		t.Fatal("inspection wrote to PDS")
	}
}

func TestRepositoryWriteRejectsConcurrentChangeAndRateLimit(t *testing.T) {
	record := testRepositoryRecord(collectionTestURI("section"), fmt.Sprintf(`{"name":"Section","parent":{"uri":%q}}`, collectionTestURI("missing")))
	p, c := newMaintenancePDS(t, record)
	ctx := context.Background()
	snap, err := readRepository(ctx, c, repairTestDID)
	if err != nil {
		t.Fatal(err)
	}
	writes := []map[string]any{findOrphans(repairTestDID, snap)[0].write()}
	p.revision++
	if _, err := applyRepositoryWrites(ctx, c, repairTestDID, snap.Commit, writes); err == nil {
		t.Fatal("concurrent commit overwritten")
	}
	p.failBatch = p.batches + 1
	if _, err := applyRepositoryWrites(ctx, c, repairTestDID, "commit-1", writes); !isRateLimited(err) {
		t.Fatalf("expected 429: %v", err)
	}
	if !reflect.DeepEqual(p.records[record.URI], record) {
		t.Fatal("failed writes changed record")
	}
}
