package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

func mapGroup(ctx context.Context, source *sql.DB, target *sql.Tx, id int64) (int64, error) {
	var name, platform string
	if err := source.QueryRowContext(ctx, `SELECT name,platform FROM groups WHERE id=$1 AND deleted_at IS NULL`, id).Scan(&name, &platform); err != nil {
		return 0, err
	}
	rows, err := target.QueryContext(ctx, `SELECT id FROM groups WHERE name=$1 AND platform=$2 AND deleted_at IS NULL`, name, platform)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return 0, err
		}
		ids = append(ids, v)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, fmt.Errorf("group mapping must be unique: source_group_id=%d matches=%d", id, len(ids))
	}
	return ids[0], nil
}

func syncActiveSubscriptions(ctx context.Context, source *sql.DB, target *sql.Tx, sourceID, targetID int64, email string) (int, error) {
	rows, err := source.QueryContext(ctx, sourceActiveSubscriptionQuery, sourceID)
	if err != nil {
		return 0, err
	}
	var subs []SourceSubscription
	for rows.Next() {
		var s SourceSubscription
		if err := rows.Scan(&s.SourceGroupID, &s.GroupName, &s.StartsAt, &s.ExpiresAt, &s.Status, &s.DailyWindowStart, &s.WeeklyWindowStart, &s.MonthlyWindowStart, &s.DailyUsageUSD, &s.WeeklyUsageUSD, &s.MonthlyUsageUSD, &s.AssignedAt, &s.Notes, &s.TotalUsageUSD); err != nil {
			rows.Close()
			return 0, err
		}
		subs = append(subs, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, s := range subs {
		group, err := mapGroup(ctx, source, target, s.SourceGroupID)
		if err != nil {
			return 0, err
		}
		_, err = target.ExecContext(ctx, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,status,daily_window_start,weekly_window_start,monthly_window_start,daily_usage_usd,weekly_usage_usd,monthly_usage_usd,assigned_at,notes,total_usage_usd,created_at,updated_at)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now(),now())
   ON CONFLICT(user_id,group_id) WHERE deleted_at IS NULL DO UPDATE SET starts_at=EXCLUDED.starts_at,expires_at=EXCLUDED.expires_at,status=EXCLUDED.status,daily_window_start=EXCLUDED.daily_window_start,weekly_window_start=EXCLUDED.weekly_window_start,monthly_window_start=EXCLUDED.monthly_window_start,daily_usage_usd=EXCLUDED.daily_usage_usd,weekly_usage_usd=EXCLUDED.weekly_usage_usd,monthly_usage_usd=EXCLUDED.monthly_usage_usd,assigned_at=EXCLUDED.assigned_at,notes=EXCLUDED.notes,total_usage_usd=EXCLUDED.total_usage_usd,updated_at=now()`, targetID, group, s.StartsAt, s.ExpiresAt, s.Status, nullTimeArg(s.DailyWindowStart), nullTimeArg(s.WeeklyWindowStart), nullTimeArg(s.MonthlyWindowStart), s.DailyUsageUSD, s.WeeklyUsageUSD, s.MonthlyUsageUSD, s.AssignedAt, s.Notes, s.TotalUsageUSD)
		if err != nil {
			return 0, err
		}
	}
	return len(subs), nil
}

func syncAPIKeys(ctx context.Context, source *sql.DB, target *sql.Tx, sourceID, targetID int64) error {
	// Copy shared schema columns so limits, expiration and revocation travel with each key.
	columns, err := target.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name='api_keys' AND column_name<>'id' ORDER BY ordinal_position`)
	if err != nil {
		return err
	}
	var names []string
	for columns.Next() {
		var name string
		if err := columns.Scan(&name); err != nil {
			columns.Close()
			return err
		}
		names = append(names, name)
	}
	err = columns.Err()
	columns.Close()
	if err != nil {
		return err
	}
	rows, err := source.QueryContext(ctx, `SELECT row_to_json(k) FROM api_keys k WHERE user_id=$1`, sourceID)
	if err != nil {
		return err
	}
	var keys []map[string]json.RawMessage
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			rows.Close()
			return err
		}
		var key map[string]json.RawMessage
		if err := json.Unmarshal(data, &key); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, key := range keys {
		key["user_id"], _ = json.Marshal(targetID)
		unavailable := string(key["deleted_at"]) != "null" && len(key["deleted_at"]) > 0
		if raw := key["group_id"]; len(raw) > 0 && string(raw) != "null" {
			var id int64
			if err := json.Unmarshal(raw, &id); err != nil {
				return err
			}
			var available bool
			if err := source.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM groups WHERE id=$1 AND deleted_at IS NULL)`, id).Scan(&available); err != nil {
				return err
			}
			unavailable = unavailable || !available
			if unavailable {
				if err := revokeUnavailableKey(ctx, target, key, targetID); err != nil {
					return err
				}
				continue
			}
			mapped, err := mapGroup(ctx, source, target, id)
			if err != nil {
				return err
			}
			key["group_id"], _ = json.Marshal(mapped)
		}
		if unavailable {
			if err := revokeUnavailableKey(ctx, target, key, targetID); err != nil {
				return err
			}
			continue
		}
		var cols, updates []string
		for _, name := range names {
			if _, ok := key[name]; !ok {
				continue
			}
			quoted := pq.QuoteIdentifier(name)
			cols = append(cols, quoted)
			if name != "key" && name != "user_id" {
				updates = append(updates, quoted+"=EXCLUDED."+quoted)
			}
		}
		data, err := json.Marshal(key)
		if err != nil {
			return err
		}
		joined := strings.Join(cols, ",")
		var id int64
		err = target.QueryRowContext(ctx, `INSERT INTO api_keys(`+joined+`) SELECT `+joined+` FROM json_populate_record(NULL::api_keys,$1::json) ON CONFLICT(key) DO UPDATE SET `+strings.Join(updates, ",")+` WHERE api_keys.user_id=EXCLUDED.user_id RETURNING id`, string(data)).Scan(&id)
		if err != nil {
			return fmt.Errorf("API key copy failed (source_user_id=%d): %w", sourceID, err)
		}
	}
	return nil
}

// Deleted groups cannot grant access; propagate revocation without inventing a replacement group.
func revokeUnavailableKey(ctx context.Context, target *sql.Tx, key map[string]json.RawMessage, userID int64) error {
	var value string
	if err := json.Unmarshal(key["key"], &value); err != nil {
		return err
	}
	var deleted *string
	if err := json.Unmarshal(key["deleted_at"], &deleted); err != nil {
		return err
	}
	_, err := target.ExecContext(ctx, `UPDATE api_keys SET status='inactive',deleted_at=coalesce($3::timestamptz,deleted_at),updated_at=now() WHERE key=$1 AND user_id=$2`, value, userID, deleted)
	return err
}
