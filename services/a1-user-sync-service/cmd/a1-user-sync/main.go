package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

const defaultInterval = 30 * time.Minute

const sourceUserQuery = `
SELECT
	id,
	email,
	password_hash,
	username,
	notes,
	role,
	balance,
	concurrency,
	status,
	signup_source,
	last_login_at,
	last_active_at,
	rpm_limit,
	balance_notify_enabled,
	balance_notify_threshold_type,
	balance_notify_threshold,
	balance_notify_extra_emails,
	total_recharged
FROM users
WHERE deleted_at IS NULL AND id = ANY($1)
`

const sourceActiveSubscriptionQuery = `
SELECT
	us.group_id,
	g.name,
	us.starts_at,
	us.expires_at,
	us.status,
	us.daily_window_start,
	us.weekly_window_start,
	us.monthly_window_start,
	us.daily_usage_usd,
	us.weekly_usage_usd,
	us.monthly_usage_usd,
	us.assigned_at,
	us.notes,
	us.total_usage_usd
FROM user_subscriptions us
JOIN groups g ON g.id = us.group_id
WHERE us.user_id = $1
	AND us.deleted_at IS NULL
	AND us.status = 'active'
	AND us.expires_at > now()
	AND g.deleted_at IS NULL
ORDER BY us.group_id, us.expires_at DESC
`

type Config struct {
	Enabled         bool    `yaml:"enabled"`
	RunOnStart      bool    `yaml:"run_on_start"`
	IntervalMinutes int     `yaml:"interval_minutes"`
	UserIDs         []int64 `yaml:"user_ids"`
	RemoveUserIDs   []int64 `yaml:"remove_user_ids"`
	// SyncSubscriptionUserIDs controls which A1 users should also sync active subscriptions.
	SyncSubscriptionUserIDs []int64        `yaml:"sync_subscription_user_ids"`
	SourceDatabase          DatabaseConfig `yaml:"source_database"`
	TargetDatabase          DatabaseConfig `yaml:"target_database"`
}

type DatabaseConfig struct {
	DSN                    string `yaml:"dsn"`
	Host                   string `yaml:"host"`
	Port                   int    `yaml:"port"`
	User                   string `yaml:"user"`
	Password               string `yaml:"password"`
	DBName                 string `yaml:"dbname"`
	SSLMode                string `yaml:"sslmode"`
	Timezone               string `yaml:"timezone"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeMinutes int    `yaml:"conn_max_lifetime_minutes"`
	ConnMaxIdleTimeMinutes int    `yaml:"conn_max_idle_time_minutes"`
}

type SourceUser struct {
	ID                         int64
	Email                      string
	PasswordHash               string
	Username                   string
	Notes                      string
	Role                       string
	Balance                    float64
	Concurrency                int
	Status                     string
	SignupSource               string
	LastLoginAt                sql.NullTime
	LastActiveAt               sql.NullTime
	RPMLimit                   int
	BalanceNotifyEnabled       bool
	BalanceNotifyThresholdType string
	BalanceNotifyThreshold     sql.NullFloat64
	BalanceNotifyExtraEmails   string
	TotalRecharged             float64
}

type SourceSubscription struct {
	SourceGroupID      int64
	GroupName          string
	StartsAt           time.Time
	ExpiresAt          time.Time
	Status             string
	DailyWindowStart   sql.NullTime
	WeeklyWindowStart  sql.NullTime
	MonthlyWindowStart sql.NullTime
	DailyUsageUSD      float64
	WeeklyUsageUSD     float64
	MonthlyUsageUSD    float64
	AssignedAt         time.Time
	Notes              sql.NullString
	TotalUsageUSD      float64
}

type SyncStats struct {
	Requested           int
	Fetched             int
	Created             int
	Updated             int
	Skipped             int
	Failed              int
	SubscriptionsSynced int
}

func main() {
	configPath := flag.String("config", "/opt/a1-user-sync/config.yaml", "config file path")
	once := flag.Bool("once", false, "run one sync round and exit")
	dryRun := flag.Bool("dry-run", false, "load and compare users without writing target database")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.LUTC)

	if *once {
		if err := runOnce(context.Background(), *configPath, *dryRun); err != nil {
			log.Fatalf("sync failed: %v", err)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	runLoop(ctx, *configPath, *dryRun)
}

func runLoop(ctx context.Context, configPath string, dryRun bool) {
	go runNotificationLoop(ctx, configPath, dryRun)
	first := true
	for {
		cfg, err := loadConfig(configPath)
		if err != nil {
			log.Printf("load config failed: %v", err)
		} else if first && cfg.RunOnStart {
			if err := syncWithConfig(ctx, cfg, dryRun); err != nil {
				log.Printf("sync failed: %v", err)
			}
		}
		first = false

		interval := defaultInterval
		if cfg != nil && cfg.IntervalMinutes > 0 {
			interval = time.Duration(cfg.IntervalMinutes) * time.Minute
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Printf("stopped")
			return
		case <-timer.C:
			if err := runOnce(ctx, configPath, dryRun); err != nil {
				log.Printf("sync failed: %v", err)
			}
		}
	}
}

func runOnce(ctx context.Context, configPath string, dryRun bool) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	return syncWithConfig(ctx, cfg, dryRun)
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	expanded := os.ExpandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}
	if cfg.IntervalMinutes <= 0 {
		cfg.IntervalMinutes = int(defaultInterval / time.Minute)
	}
	if cfg.SourceDatabase.SSLMode == "" {
		cfg.SourceDatabase.SSLMode = "prefer"
	}
	if cfg.TargetDatabase.SSLMode == "" {
		cfg.TargetDatabase.SSLMode = "prefer"
	}
	if cfg.SourceDatabase.Timezone == "" {
		cfg.SourceDatabase.Timezone = "Asia/Shanghai"
	}
	if cfg.TargetDatabase.Timezone == "" {
		cfg.TargetDatabase.Timezone = "Asia/Shanghai"
	}
	return &cfg, nil
}

func syncWithConfig(ctx context.Context, cfg *Config, dryRun bool) error {
	if cfg == nil || !cfg.Enabled {
		log.Printf("sync disabled")
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	sourceDB, err := openDB(cfg.SourceDatabase)
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer sourceDB.Close()
	conn, err := sourceDB.Conn(ctx)
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

	targetDB, err := openDB(cfg.TargetDatabase)
	if err != nil {
		return fmt.Errorf("open target database: %w", err)
	}
	defer targetDB.Close()
	requests, err := readRequests(ctx, sourceDB)
	if err != nil {
		return err
	}
	userIDs, subscriptionUserSet := applyRequests(cfg, requests)

	users, err := fetchSourceUsers(ctx, sourceDB, userIDs)
	if err != nil {
		return err
	}

	stats := SyncStats{Requested: len(userIDs), Fetched: len(users)}
	for i := range users {
		op, syncedSubscriptions, err := syncGuarded(ctx, sourceDB, targetDB, users[i], subscriptionUserSet[users[i].ID], dryRun)
		if err != nil {
			stats.Failed++
			log.Printf("sync user failed: source_id=%d email=%s op=%s err=%v", users[i].ID, users[i].Email, op, err)
			continue
		}
		stats.SubscriptionsSynced += syncedSubscriptions
		switch op {
		case "created":
			stats.Created++
		case "updated":
			stats.Updated++
		case "skipped":
			stats.Skipped++
		}
	}
	log.Printf("sync finished: requested=%d fetched=%d created=%d updated=%d skipped=%d failed=%d subscriptions_synced=%d dry_run=%v",
		stats.Requested, stats.Fetched, stats.Created, stats.Updated, stats.Skipped, stats.Failed, stats.SubscriptionsSynced, dryRun)
	if stats.Failed > 0 {
		return fmt.Errorf("%d users failed", stats.Failed)
	}
	return nil
}

func openDB(cfg DatabaseConfig) (*sql.DB, error) {
	dsn := cfg.DSN
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.keywordDSN()
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetimeMinutes > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMinutes) * time.Minute)
	}
	if cfg.ConnMaxIdleTimeMinutes > 0 {
		db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTimeMinutes) * time.Minute)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (cfg DatabaseConfig) keywordDSN() string {
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}
	timezone := cfg.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	parts := []string{
		"host=" + pqQuote(cfg.Host),
		fmt.Sprintf("port=%d", port),
		"user=" + pqQuote(cfg.User),
		"dbname=" + pqQuote(cfg.DBName),
		"sslmode=" + pqQuote(sslMode),
		"TimeZone=" + pqQuote(timezone),
	}
	if cfg.Password != "" {
		parts = append(parts, "password="+pqQuote(cfg.Password))
	}
	return strings.Join(parts, " ")
}

func pqQuote(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

func effectiveUserIDs(userIDs []int64, removeUserIDs []int64) []int64 {
	removed := make(map[int64]struct{}, len(removeUserIDs))
	for _, id := range removeUserIDs {
		if id > 0 {
			removed[id] = struct{}{}
		}
	}
	seen := make(map[int64]struct{}, len(userIDs))
	out := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			continue
		}
		if _, ok := removed[id]; ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func effectiveUserIDSet(userIDs []int64, removeUserIDs []int64) map[int64]bool {
	out := make(map[int64]bool)
	for _, id := range effectiveUserIDs(userIDs, removeUserIDs) {
		out[id] = true
	}
	return out
}

func fetchSourceUsers(ctx context.Context, db *sql.DB, userIDs []int64) ([]SourceUser, error) {
	rows, err := db.QueryContext(ctx, sourceUserQuery, pq.Array(userIDs))
	if err != nil {
		return nil, fmt.Errorf("query source users: %w", err)
	}
	defer rows.Close()

	users := make([]SourceUser, 0, len(userIDs))
	for rows.Next() {
		var u SourceUser
		var concurrency int64
		var rpmLimit int64
		var thresholdType sql.NullString
		var extraEmails sql.NullString
		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.PasswordHash,
			&u.Username,
			&u.Notes,
			&u.Role,
			&u.Balance,
			&concurrency,
			&u.Status,
			&u.SignupSource,
			&u.LastLoginAt,
			&u.LastActiveAt,
			&rpmLimit,
			&u.BalanceNotifyEnabled,
			&thresholdType,
			&u.BalanceNotifyThreshold,
			&extraEmails,
			&u.TotalRecharged,
		); err != nil {
			return nil, fmt.Errorf("scan source user: %w", err)
		}
		u.Concurrency = int(concurrency)
		u.RPMLimit = int(rpmLimit)
		if thresholdType.Valid && thresholdType.String != "" {
			u.BalanceNotifyThresholdType = thresholdType.String
		} else {
			u.BalanceNotifyThresholdType = "fixed"
		}
		if extraEmails.Valid && json.Valid([]byte(extraEmails.String)) {
			u.BalanceNotifyExtraEmails = extraEmails.String
		} else {
			u.BalanceNotifyExtraEmails = "[]"
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func syncUser(ctx context.Context, sourceDB *sql.DB, targetDB *sql.DB, u SourceUser, syncSubscriptions bool, dryRun bool) (string, int, error) {
	targetID, err := findTargetUserID(ctx, targetDB, u.Email)
	if err != nil {
		return "lookup", 0, err
	}
	if dryRun {
		if targetID > 0 {
			return "updated", 0, nil
		}
		return "created", 0, nil
	}

	tx, err := targetDB.BeginTx(ctx, nil)
	if err != nil {
		return "begin", 0, err
	}
	defer tx.Rollback()

	op := "updated"
	if targetID > 0 {
		if err := updateTargetUser(ctx, tx, targetID, u); err != nil {
			return "updated", 0, err
		}
		if err := ensureEmailIdentity(ctx, tx, targetID, u.Email); err != nil {
			return "updated", 0, err
		}
	} else {
		targetID, err = createTargetUser(ctx, tx, u)
		if err != nil {
			return "created", 0, err
		}
		if err := ensureEmailIdentity(ctx, tx, targetID, u.Email); err != nil {
			return "created", 0, err
		}
		op = "created"
	}

	syncedSubscriptions := 0
	if syncSubscriptions {
		n, err := syncActiveSubscriptions(ctx, sourceDB, tx, u.ID, targetID, u.Email)
		if err != nil {
			return op, syncedSubscriptions, err
		}
		syncedSubscriptions = n
		if err := syncAPIKeys(ctx, sourceDB, tx, u.ID, targetID); err != nil {
			return op, syncedSubscriptions, err
		}
	}
	if err := tx.Commit(); err != nil {
		return op, syncedSubscriptions, err
	}
	return op, syncedSubscriptions, nil
}

func findTargetUserID(ctx context.Context, db *sql.DB, email string) (int64, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id
FROM users
WHERE deleted_at IS NULL AND lower(btrim(email)) = lower(btrim($1))
ORDER BY id
LIMIT 2
`, email)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if len(ids) > 1 {
		return 0, fmt.Errorf("multiple target users match email %q", strings.TrimSpace(email))
	}
	return ids[0], nil
}

func updateTargetUser(ctx context.Context, tx *sql.Tx, targetID int64, u SourceUser) error {
	_, err := tx.ExecContext(ctx, `
UPDATE users SET
	email = $2,
	password_hash = $3,
	username = $4,
	notes = $5,
	role = $6,
	balance = $7,
	concurrency = $8,
	status = $9,
	signup_source = $10,
	last_login_at = $11,
	last_active_at = $12,
	rpm_limit = $13,
	balance_notify_enabled = $14,
	balance_notify_threshold_type = $15,
	balance_notify_threshold = $16,
	balance_notify_extra_emails = $17,
	total_recharged = $18,
	updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
`, targetID,
		u.Email,
		u.PasswordHash,
		u.Username,
		u.Notes,
		safeRole(u.Role),
		u.Balance,
		u.Concurrency,
		safeStatus(u.Status),
		safeSignupSource(u.SignupSource),
		nullTimeArg(u.LastLoginAt),
		nullTimeArg(u.LastActiveAt),
		u.RPMLimit,
		u.BalanceNotifyEnabled,
		u.BalanceNotifyThresholdType,
		nullFloatArg(u.BalanceNotifyThreshold),
		u.BalanceNotifyExtraEmails,
		u.TotalRecharged,
	)
	return err
}

func createTargetUser(ctx context.Context, tx *sql.Tx, u SourceUser) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
INSERT INTO users (
	email,
	password_hash,
	username,
	notes,
	role,
	balance,
	concurrency,
	status,
	signup_source,
	last_login_at,
	last_active_at,
	rpm_limit,
	balance_notify_enabled,
	balance_notify_threshold_type,
	balance_notify_threshold,
	balance_notify_extra_emails,
	total_recharged,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
	$11, $12, $13, $14, $15, $16, $17, now(), now()
)
RETURNING id
`,
		u.Email,
		u.PasswordHash,
		u.Username,
		u.Notes,
		safeRole(u.Role),
		u.Balance,
		u.Concurrency,
		safeStatus(u.Status),
		safeSignupSource(u.SignupSource),
		nullTimeArg(u.LastLoginAt),
		nullTimeArg(u.LastActiveAt),
		u.RPMLimit,
		u.BalanceNotifyEnabled,
		u.BalanceNotifyThresholdType,
		nullFloatArg(u.BalanceNotifyThreshold),
		u.BalanceNotifyExtraEmails,
		u.TotalRecharged,
	).Scan(&id)
	return id, err
}

func ensureEmailIdentity(ctx context.Context, tx *sql.Tx, userID int64, email string) error {
	subject := normalizedEmailSubject(email)
	if subject == "" {
		return nil
	}

	var ownerID int64
	err := tx.QueryRowContext(ctx, `
SELECT user_id
FROM auth_identities
WHERE provider_type = 'email'
	AND provider_key = 'email'
	AND provider_subject = $1
`, subject).Scan(&ownerID)
	if err == nil {
		if ownerID != userID {
			return fmt.Errorf("email identity %q already belongs to user %d", subject, ownerID)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO auth_identities (
	user_id,
	provider_type,
	provider_key,
	provider_subject,
	verified_at,
	metadata,
	created_at,
	updated_at
) VALUES (
	$1, 'email', 'email', $2, now(), '{"source":"a1_user_sync"}'::jsonb, now(), now()
)
ON CONFLICT (provider_type, provider_key, provider_subject) DO NOTHING
`, userID, subject)
	return err
}

func normalizedEmailSubject(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	syntheticDomains := []string{
		"@linuxdo.local",
		"@oidc.local",
		"@wechat.local",
		"@dingtalk.local",
	}
	for _, suffix := range syntheticDomains {
		if strings.HasSuffix(normalized, suffix) {
			return ""
		}
	}
	return normalized
}

func nullTimeArg(v sql.NullTime) any {
	if !v.Valid {
		return nil
	}
	return v.Time
}

func nullFloatArg(v sql.NullFloat64) any {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func safeRole(role string) string {
	switch strings.TrimSpace(role) {
	case "admin":
		return "admin"
	default:
		return "user"
	}
}

func safeStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "inactive":
		return "inactive"
	default:
		return "active"
	}
}

func safeSignupSource(source string) string {
	switch strings.TrimSpace(source) {
	case "linuxdo", "wechat", "oidc", "github", "google", "dingtalk":
		return strings.TrimSpace(source)
	default:
		return "email"
	}
}
