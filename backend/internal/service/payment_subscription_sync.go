package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const subscriptionSyncPrefix = "a1_user_sync.request."
const subscriptionSyncResultPrefix = "a1_user_sync.result."

// Enabled is desired state; Synced and Executed are derived from consumer receipts.
type SubscriptionSyncRequest struct {
	UserID      int64     `json:"user_id"`
	OrderID     int64     `json:"order_id"`
	Enabled     bool      `json:"enabled"`
	RequestedBy int64     `json:"requested_by"`
	UpdatedAt   time.Time `json:"updated_at"`
	Synced      bool      `json:"synced"`
	Executed    bool      `json:"executed"`
}

func (s *PaymentService) ListSubscriptionSyncRequests(ctx context.Context) ([]SubscriptionSyncRequest, error) {
	rows, err := s.entClient.Setting.Query().Where(setting.Or(setting.KeyHasPrefix(subscriptionSyncPrefix), setting.KeyHasPrefix(subscriptionSyncResultPrefix))).All(ctx)
	if err != nil {
		return nil, err
	}
	requests := make([]SubscriptionSyncRequest, 0, len(rows))
	results := make(map[int64]SubscriptionSyncRequest)
	for _, row := range rows {
		var request SubscriptionSyncRequest
		if err := json.Unmarshal([]byte(row.Value), &request); err != nil {
			return nil, fmt.Errorf("invalid subscription sync request: %w", err)
		}
		if strings.HasPrefix(row.Key, subscriptionSyncResultPrefix) {
			results[request.UserID] = request
		} else {
			requests = append(requests, request)
		}
	}
	for i := range requests {
		result, ok := results[requests[i].UserID]
		requests[i].Synced = ok && result.Enabled
		requests[i].Executed = ok && result.Enabled == requests[i].Enabled && result.UpdatedAt.Equal(requests[i].UpdatedAt)
		if ok && result.UpdatedAt.After(requests[i].UpdatedAt) {
			result.Synced, result.Executed = result.Enabled, true
			requests[i] = result
		}
		delete(results, requests[i].UserID)
	}
	for _, result := range results {
		result.Synced = result.Enabled
		result.Executed = true
		requests = append(requests, result)
	}
	return requests, nil
}

func (s *PaymentService) RequestSubscriptionSync(ctx context.Context, orderID, operatorID int64, enabled bool) (*SubscriptionSyncRequest, error) {
	order, err := s.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.OrderType != "subscription" {
		return nil, infraerrors.BadRequest("INVALID_SYNC_ORDER", "仅订阅订单支持同步")
	}
	if enabled {
		if order.Status != OrderStatusCompleted {
			return nil, infraerrors.BadRequest("INVALID_SYNC_ORDER", "仅已完成的订阅订单支持同步")
		}
		now := time.Now()
		active, err := s.entClient.UserSubscription.Query().Where(
			usersubscription.UserIDEQ(order.UserID), usersubscription.StatusEQ("active"),
			usersubscription.StartsAtLTE(now), usersubscription.ExpiresAtGT(now), usersubscription.DeletedAtIsNil(),
		).Exist(ctx)
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, infraerrors.BadRequest("NO_ACTIVE_SUBSCRIPTION", "该用户没有有效订阅")
		}
	}
	return s.saveSubscriptionSyncRequest(ctx, order.UserID, orderID, operatorID, enabled)
}

func (s *PaymentService) RequestSubscriptionSyncBySubscription(ctx context.Context, subscriptionID, operatorID int64, enabled bool) (*SubscriptionSyncRequest, error) {
	subscription, err := s.entClient.UserSubscription.Query().Where(usersubscription.IDEQ(subscriptionID), usersubscription.DeletedAtIsNil()).Only(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if enabled && (subscription.Status != "active" || subscription.StartsAt.After(now) || !subscription.ExpiresAt.After(now)) {
		return nil, infraerrors.BadRequest("NO_ACTIVE_SUBSCRIPTION", "该订阅当前无效")
	}
	// The consumer uses user_id; zero order_id also supports manually assigned subscriptions.
	return s.saveSubscriptionSyncRequest(ctx, subscription.UserID, 0, operatorID, enabled)
}

func (s *PaymentService) saveSubscriptionSyncRequest(ctx context.Context, userID, orderID, operatorID int64, enabled bool) (*SubscriptionSyncRequest, error) {
	request := &SubscriptionSyncRequest{UserID: userID, OrderID: orderID, Enabled: enabled, RequestedBy: operatorID, UpdatedAt: time.Now().UTC()}
	value, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	err = s.entClient.Setting.Create().SetKey(subscriptionSyncPrefix + strconv.FormatInt(userID, 10)).SetValue(string(value)).OnConflictColumns(setting.FieldKey).UpdateNewValues().Exec(ctx)
	if err != nil {
		return nil, err
	}
	return request, nil
}
