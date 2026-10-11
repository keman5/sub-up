package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func openStatusDatabases(cfg *Config) ([]*sql.DB, error) {
	configs := []DatabaseConfig{cfg.TargetDatabase}
	for _, name := range cfg.StatusDatabaseNames {
		if cfg.TargetDatabase.DSN != "" {
			return nil, fmt.Errorf("status_database_names requires structured target_database")
		}
		if name == cfg.TargetDatabase.DBName || name == cfg.SourceDatabase.DBName {
			continue
		}
		c := cfg.TargetDatabase
		c.DBName = name
		configs = append(configs, c)
	}
	var databases []*sql.DB
	for _, c := range configs {
		db, err := openDB(c)
		if err != nil {
			for _, opened := range databases {
				opened.Close()
			}
			return nil, err
		}
		databases = append(databases, db)
	}
	return databases, nil
}

// IDs are environment-local; only canonical email identifies the same user.
func mappedUserID(ctx context.Context, from, to *sql.DB, id int64) (int64, error) {
	var email string
	if err := from.QueryRowContext(ctx, `SELECT lower(trim(email)) FROM users WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&email); err != nil {
		return 0, err
	}
	mapped, err := findTargetUserID(ctx, to, email)
	if err == nil && mapped == 0 {
		err = sql.ErrNoRows
	}
	return mapped, err
}

func relayRequests(ctx context.Context, replica, source *sql.DB) error {
	requests, err := readRequests(ctx, replica)
	if err != nil {
		return err
	}
	for id, r := range requests {
		mapped, err := mappedUserID(ctx, replica, source, id)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		r.UserID, r.OrderID = mapped, 0
		value, err := json.Marshal(r)
		if err != nil {
			return err
		}
		_, err = source.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,now()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now() WHERE (settings.value::jsonb->>'updated_at')::timestamptz < $3`, requestPrefix+strconv.FormatInt(mapped, 10), string(value), r.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func mirrorResults(ctx context.Context, source, replica *sql.DB) error {
	results, err := readStates(ctx, source, resultPrefix)
	if err != nil {
		return err
	}
	for _, r := range results {
		mapped, err := mappedUserID(ctx, source, replica, r.UserID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		r.UserID, r.OrderID = mapped, 0
		encoded, err := json.Marshal(r)
		if err != nil {
			return err
		}
		_, err = replica.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) VALUES($1,$2,now()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now() WHERE (settings.value::jsonb->>'updated_at')::timestamptz < $3`, resultPrefix+strconv.FormatInt(mapped, 10), string(encoded), r.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

// Legacy configuration gets a receipt only after an actual successful copy.
func recordLegacyResult(ctx context.Context, source *sql.DB, id int64) error {
	r := syncRequest{UserID: id, Enabled: true, UpdatedAt: time.Now().UTC()}
	value, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = source.ExecContext(ctx, `INSERT INTO settings(key,value,updated_at) SELECT $1,$2,now() WHERE NOT EXISTS(SELECT 1 FROM settings WHERE key=$3) ON CONFLICT(key) DO NOTHING`, resultPrefix+strconv.FormatInt(id, 10), string(value), requestPrefix+strconv.FormatInt(id, 10))
	return err
}
