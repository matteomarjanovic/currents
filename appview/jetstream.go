package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bluesky-social/jetstream"
)

const jetstreamInitialLookback = time.Hour

func runJetstreamListener(ctx context.Context, host, apiKey string, handler *TapHandler) error {
	for ctx.Err() == nil {
		cursor, err := handler.Store.JetstreamCursor(ctx)
		if err != nil {
			return err
		}
		opts := []jetstream.Option{jetstream.WithCollection("is.currents.*")}
		switch {
		case cursor == 0:
			// Overlap the TAP baseline at cutover instead of starting at the tip.
			opts = append(opts, jetstream.WithLiveCursor(uint64(time.Now().Add(-jetstreamInitialLookback).UnixMicro())))
		case apiKey != "":
			opts = append(opts, jetstream.WithAPIKey(apiKey), jetstream.WithAfterSeq(cursor))
		default:
			opts = append(opts, jetstream.WithLiveCursor(cursor))
		}
		client, err := jetstream.Subscribe(host, opts...)
		if err != nil {
			return err
		}
		slog.Info("Jetstream listener started", "host", host, "cursor", cursor)
		err = consumeJetstream(ctx, client, handler, cursor)
		client.Close()
		if ctx.Err() != nil {
			return nil
		}
		if errors.Is(err, jetstream.ErrFatal) || errors.Is(err, jetstream.ErrCursorClamped) {
			return err
		}
		slog.Warn("Jetstream listener reconnecting", "err", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
	return nil
}

func consumeJetstream(ctx context.Context, client *jetstream.Client, handler *TapHandler, cursor uint64) error {
	lastProgress := time.Now()
	processed := 0
	for batch, err := range client.Events(ctx) {
		if err != nil {
			if errors.Is(err, jetstream.ErrFatal) || errors.Is(err, jetstream.ErrCursorClamped) {
				return err
			}
			slog.Warn("Jetstream stream error", "err", err)
			continue // The SDK reconnects without advancing our saved cursor.
		}
		for _, event := range batch.Events() {
			if event.Seq <= cursor {
				continue
			}
			if err := applyJetstreamEvent(ctx, handler, event); err != nil {
				if !errors.Is(err, errSkipRecord) {
					return fmt.Errorf("Jetstream seq %d did %s: %w", event.Seq, event.DID, err)
				}
				slog.Warn("Jetstream record skipped", "seq", event.Seq, "did", event.DID, "err", err)
			}
			if err := handler.Store.AdvanceJetstreamCursor(ctx, event.Seq); err != nil {
				return err // Replayed events are safe to apply again.
			}
			cursor = event.Seq
			processed++
			if time.Since(lastProgress) >= time.Minute {
				lag := time.Duration(0)
				if event.WitnessedAtUS != 0 {
					lag = time.Since(time.UnixMicro(event.WitnessedAtUS)).Round(time.Second)
				}
				slog.Info("Jetstream progress", "cursor", cursor, "events", processed, "witness_lag", lag)
				lastProgress, processed = time.Now(), 0
			}
		}
	}
	return nil
}

func applyJetstreamEvent(ctx context.Context, handler *TapHandler, event jetstream.Event) error {
	state, _, err := handler.Store.JetstreamRepoState(ctx, event.DID)
	if err != nil {
		return err
	}
	if state == "opted_out" || state == "" && (event.Kind != jetstream.KindCommit || event.Commit == nil || event.Commit.Collection != currentsProfileNSID) {
		return nil
	}
	unlock, err := handler.Store.lockRepository(ctx, event.DID)
	if err != nil {
		return err
	}
	defer unlock()
	state, reconciledRev, err := handler.Store.JetstreamRepoState(ctx, event.DID)
	if err != nil {
		return err
	}
	if state == "opted_out" {
		return nil
	}

	switch event.Kind {
	case jetstream.KindCommit:
		if event.Commit == nil || state == "inactive" || state == "" && event.Commit.Collection != currentsProfileNSID {
			return nil
		}
		if reconciledRev != "" && event.Commit.Rev != "" && event.Commit.Rev <= reconciledRev {
			return nil // The PDS snapshot already included this change.
		}
		var record json.RawMessage
		if event.Commit.Operation != jetstream.OpDelete {
			if event.Commit.Record == nil {
				return errSkipRecord
			}
			record, err = json.Marshal(event.Commit.Record)
			if err != nil {
				return errSkipRecord
			}
		}
		if err := handleTapRecord(ctx, handler, &TapRecordEvent{
			DID: event.DID, Collection: event.Commit.Collection, Rkey: event.Commit.Rkey,
			Action: string(event.Commit.Operation), CID: event.Commit.CID, Record: record,
		}); err != nil {
			return err
		}
		if state == "" && event.Commit.Operation != jetstream.OpDelete {
			return handler.Store.EnableJetstreamRepo(ctx, event.DID)
		}
	case jetstream.KindIdentity:
		if state == "active" && event.Identity != nil {
			return handleTapIdentity(ctx, handler, &TapIdentityEvent{DID: event.DID, Handle: event.Identity.Handle})
		}
	case jetstream.KindAccount:
		if state == "" || event.Account == nil {
			return nil
		}
		if event.Account.Active {
			_, err := handler.Store.ReactivateJetstreamRepo(ctx, event.DID)
			return err
		}
		if err := handler.Store.SetJetstreamRepoInactive(ctx, event.DID); err != nil {
			return err
		}
		return handler.Store.ClearIndexedRepo(ctx, event.DID)
	case jetstream.KindSync:
		if state == "active" {
			if event.Sync != nil && reconciledRev != "" && event.Sync.Rev != "" && event.Sync.Rev <= reconciledRev {
				return nil
			}
			if err := handler.Store.ClearIndexedRepo(ctx, event.DID); err != nil {
				return err
			}
			if err := handler.Store.SetJetstreamReconciledRev(ctx, event.DID, ""); err != nil {
				return err
			}
			return handler.Store.QueueJetstreamBackfill(ctx, event.DID)
		}
	}
	return nil
}
