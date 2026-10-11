package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
)

const requestPrefix = "a1_user_sync.request."
const resultPrefix = "a1_user_sync.result."

type syncRequest struct {
	UserID    int64     `json:"user_id"`
	OrderID   int64     `json:"order_id"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
}

func readRequests(ctx context.Context, db *sql.DB) (map[int64]syncRequest, error) {
	return readStates(ctx, db, requestPrefix)
}

func readStates(ctx context.Context, db *sql.DB, prefix string) (map[int64]syncRequest, error) {
	rows, err := db.QueryContext(ctx, `SELECT key,value FROM settings WHERE left(key,length($1))=$1`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]syncRequest{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		var r syncRequest
		if err := json.Unmarshal([]byte(value), &r); err != nil {
			return nil, fmt.Errorf("invalid request %s: %w", key, err)
		}
		if r.UserID <= 0 || key != prefix+strconv.FormatInt(r.UserID, 10) || r.UpdatedAt.IsZero() {
			return nil, fmt.Errorf("invalid request identity %s", key)
		}
		out[r.UserID] = r
	}
	return out, rows.Err()
}

func applyRequests(cfg *Config, requests map[int64]syncRequest) ([]int64, map[int64]bool) {
	users := effectiveUserIDSet(append(append([]int64{}, cfg.UserIDs...), cfg.SyncSubscriptionUserIDs...), cfg.RemoveUserIDs)
	subs := effectiveUserIDSet(cfg.SyncSubscriptionUserIDs, cfg.RemoveUserIDs)
	for id, r := range requests {
		if r.Enabled {
			users[id] = true
			subs[id] = true
		} else {
			delete(users, id)
			delete(subs, id)
		}
	}
	ids := make([]int64, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	return effectiveUserIDs(ids, nil), subs
}

// Hold the source request row until the target commit; an accepted cancellation cannot race a later copy.
func syncGuarded(ctx context.Context, source, target *sql.DB, u SourceUser, subs, dry bool) (string, int, error) {
	guard, err := source.BeginTx(ctx, nil)
	if err != nil {
		return "guard", 0, err
	}
	defer guard.Rollback()
	var value string
	err = guard.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1 FOR SHARE`, requestPrefix+strconv.FormatInt(u.ID, 10)).Scan(&value)
	if err != nil && err != sql.ErrNoRows {
		return "guard", 0, err
	}
	if err == nil {
		var r syncRequest
		if err := json.Unmarshal([]byte(value), &r); err != nil {
			return "guard", 0, err
		}
		if !r.Enabled {
			return "skipped", 0, nil
		}
	}
	return syncUser(ctx, source, target, u, subs, dry)
}

func runNotificationLoop(ctx context.Context, path string, dry bool) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := consumeNotifications(ctx, path, dry); err != nil {
				log.Printf("notification consumption failed: %v", err)
			}
		}
	}
}

func consumeNotifications(ctx context.Context, path string, dry bool) error {
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	source, err := openDB(cfg.SourceDatabase)
	if err != nil {
		return err
	}
	defer source.Close()
	// Serialize with the periodic round and other replicas without holding a process-wide lock.
	conn, err := source.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(4141001)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(4141001)`)
	replicas, err := openStatusDatabases(cfg)
	if err != nil {
		return err
	}
	defer func() {
		for _, db := range replicas {
			db.Close()
		}
	}()
	if !dry {
		for _, db := range replicas {
			if err := relayRequests(ctx, db, source); err != nil {
				return err
			}
		}
	}
	requests, err := readRequests(ctx, source)
	if err != nil {
		return err
	}
	target, err := openDB(cfg.TargetDatabase)
	if err != nil {
		return err
	}
	defer target.Close()
	for id, r := range requests {
		var old string
		err := source.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, resultPrefix+strconv.FormatInt(id, 10)).Scan(&old)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		var previous struct {
			UpdatedAt time.Time `json:"updated_at"`
			Enabled   bool      `json:"enabled"`
		}
		if json.Unmarshal([]byte(old), &previous) == nil && previous.UpdatedAt.Equal(r.UpdatedAt) && previous.Enabled == r.Enabled {
			continue
		}
		if r.Enabled {
			users, err := fetchSourceUsers(ctx, source, []int64{id})
			if err != nil {
				return err
			}
			if len(users) != 1 {
				log.Printf("notification user_id=%d not found; request not acknowledged", id)
				continue
			}
			op, n, err := syncGuarded(ctx, source, target, users[0], true, dry)
			if err != nil {
				log.Printf("notification user_id=%d failed: %v", id, err)
				continue
			}
			if op == "skipped" {
				continue
			}
			log.Printf("notification synced user_id=%d subscriptions=%d", id, n)
		} else {
			log.Printf("notification cancelled user_id=%d", id)
		}
		if dry {
			continue
		}
		value, _ := json.Marshal(r)
		// Conditional acknowledgement: never acknowledge a newer request using an older result.
		_, err = source.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) SELECT $1,$2,now() WHERE EXISTS (SELECT 1 FROM settings WHERE key=$3 AND (value::jsonb->>'updated_at')::timestamptz=$4 AND (value::jsonb->>'enabled')::boolean=$5) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`, resultPrefix+strconv.FormatInt(id, 10), string(value), requestPrefix+strconv.FormatInt(id, 10), r.UpdatedAt, r.Enabled)
		if err != nil {
			return err
		}
	}
	if !dry {
		for _, db := range replicas {
			if err := mirrorResults(ctx, source, db); err != nil {
				return err
			}
		}
	}
	return nil
}
