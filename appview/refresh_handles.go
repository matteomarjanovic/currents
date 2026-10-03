package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bluesky-social/indigo/atproto/identity"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// Refresh users whose handle changed before the TAP identity handler was enabled.
func refreshUserHandles(ctx context.Context, store *PgStore, dir identity.Directory, did string, dryRun bool) error {
	rows, err := store.pool.Query(ctx, `SELECT did, COALESCE(handle, '') FROM "user"
		WHERE $1 = '' OR did = $1 ORDER BY did`, did)
	if err != nil {
		return err
	}
	type actor struct{ did, handle string }
	var actors []actor
	for rows.Next() {
		var a actor
		if err := rows.Scan(&a.did, &a.handle); err != nil {
			rows.Close()
			return err
		}
		actors = append(actors, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if did != "" && len(actors) == 0 {
		return fmt.Errorf("no indexed actor for %s", did)
	}

	changed, unresolved := 0, 0
	for _, a := range actors {
		ident, err := dir.LookupDID(ctx, syntax.DID(a.did))
		if err == nil && (ident == nil || ident.Handle.IsInvalidHandle()) {
			err = fmt.Errorf("DID has no valid handle")
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("could not refresh actor handle", "did", a.did, "err", err)
			unresolved++
			continue
		}
		handle := ident.Handle.Normalize().String()
		if handle == a.handle {
			continue
		}
		slog.Info("actor handle changed", "did", a.did, "from", a.handle, "to", handle, "dry_run", dryRun)
		if !dryRun {
			if err := store.UpdateUserHandle(ctx, a.did, handle); err != nil {
				return err
			}
		}
		changed++
	}
	slog.Info("handle refresh finished", "accounts", len(actors), "changed", changed, "unresolved", unresolved, "dry_run", dryRun)
	if unresolved != 0 {
		return fmt.Errorf("%d actor handles could not be resolved", unresolved)
	}
	return nil
}
