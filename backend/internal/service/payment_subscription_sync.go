package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const subscriptionSyncPrefix = "a1_user_sync.request."

// SubscriptionSyncRequest is desired state for a1-user-sync, not proof of execution.
type SubscriptionSyncRequest struct {
	UserID      int64     `json:"user_id"`
	OrderID     int64     `json:"order_id"`
	Enabled     bool      `json:"enabled"`
	RequestedBy int64     `json:"requested_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *PaymentService) ListSubscriptionSyncRequests(ctx context.Context) ([]SubscriptionSyncRequest, error) {
	rows, err := s.entClient.Setting.Query().Where(setting.KeyHasPrefix(subscriptionSyncPrefix)).All(ctx)
	if err != nil {
		return nil, err
	}
	requests := make([]SubscriptionSyncRequest, 0, len(rows))
	for _, row := range rows {
		var request SubscriptionSyncRequest
		if err := json.Unmarshal([]byte(row.Value), &request); err != nil {
			return nil, fmt.Errorf("invalid subscription sync request: %w", err)
		}
		requests = append(requests, request)
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
	request := &SubscriptionSyncRequest{UserID: order.UserID, OrderID: orderID, Enabled: enabled, RequestedBy: operatorID, UpdatedAt: time.Now().UTC()}
	value, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	err = s.entClient.Setting.Create().SetKey(subscriptionSyncPrefix + strconv.FormatInt(order.UserID, 10)).SetValue(string(value)).OnConflictColumns(setting.FieldKey).UpdateNewValues().Exec(ctx)
	if err != nil {
		return nil, err
	}
	return request, nil
}
