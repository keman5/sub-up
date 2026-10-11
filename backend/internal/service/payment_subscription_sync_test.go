package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/setting"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionSyncDesiredState(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	u, err := client.User.Create().SetEmail("sync@example.com").SetPasswordHash("hash").Save(ctx)
	require.NoError(t, err)
	g, err := client.Group.Create().SetName("sync").Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().SetUserID(u.ID).SetUserEmail(u.Email).SetUserName("sync-user").SetAmount(10).SetPayAmount(10).SetFeeRate(0).SetRechargeCode("sync-code").SetPaymentTradeNo("").SetClientIP("127.0.0.1").SetSrcHost("a1.upit.top").SetOutTradeNo("sync-order").SetPaymentType("alipay").SetOrderType("subscription").SetStatus(OrderStatusCompleted).SetExpiresAt(time.Now().Add(time.Hour)).Save(ctx)
	require.NoError(t, err)
	_, err = svc.RequestSubscriptionSync(ctx, int64(order.ID), u.ID, true)
	require.Error(t, err, "users without a current subscription cannot be enabled")
	subscription, err := client.UserSubscription.Create().SetUserID(u.ID).SetGroupID(g.ID).SetStartsAt(time.Now().Add(-time.Hour)).SetExpiresAt(time.Now().Add(time.Hour)).Save(ctx)
	require.NoError(t, err)
	request, err := svc.RequestSubscriptionSyncBySubscription(ctx, subscription.ID, u.ID, true)
	require.NoError(t, err)
	require.Equal(t, u.ID, request.UserID)
	require.Zero(t, request.OrderID)
	_, err = client.UserSubscription.UpdateOneID(subscription.ID).SetStatus("revoked").Save(ctx)
	require.NoError(t, err)
	_, err = svc.RequestSubscriptionSyncBySubscription(ctx, subscription.ID, u.ID, true)
	require.Error(t, err)
	_, err = svc.RequestSubscriptionSyncBySubscription(ctx, subscription.ID, u.ID, false)
	require.NoError(t, err)
	_, err = client.UserSubscription.UpdateOneID(subscription.ID).SetStatus("active").Save(ctx)
	require.NoError(t, err)
	for _, enabled := range []bool{true, true, false} {
		request, err := svc.RequestSubscriptionSync(ctx, int64(order.ID), u.ID, enabled)
		require.NoError(t, err)
		require.Equal(t, enabled, request.Enabled)
		requests, err := svc.ListSubscriptionSyncRequests(ctx)
		require.NoError(t, err)
		require.Len(t, requests, 1, "repeated clicks replace per-user desired state")
		require.Equal(t, enabled, requests[0].Enabled)
	}
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetStatus(OrderStatusPending).Save(ctx)
	require.NoError(t, err)
	_, err = svc.RequestSubscriptionSync(ctx, int64(order.ID), u.ID, true)
	require.Error(t, err)
	_, err = svc.RequestSubscriptionSync(ctx, int64(order.ID), u.ID, false)
	require.NoError(t, err, "cancellation must remain available even when an order is no longer completed")
	_, err = client.PaymentOrder.UpdateOneID(order.ID).SetOrderType("balance").Save(ctx)
	require.NoError(t, err)
	_, err = svc.RequestSubscriptionSync(ctx, int64(order.ID), u.ID, false)
	require.Error(t, err)
}

func TestSubscriptionSyncExecutionResult(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	now := time.Now().UTC()
	request := SubscriptionSyncRequest{UserID: 21, Enabled: false, UpdatedAt: now}
	result := SubscriptionSyncRequest{UserID: 21, Enabled: true, UpdatedAt: now.Add(-time.Second)}
	for _, item := range []struct {
		prefix string
		value  SubscriptionSyncRequest
	}{{subscriptionSyncPrefix, request}, {subscriptionSyncResultPrefix, result}} {
		value, err := json.Marshal(item.value)
		require.NoError(t, err)
		_, err = client.Setting.Create().SetKey(item.prefix + strconv.FormatInt(item.value.UserID, 10)).SetValue(string(value)).Save(ctx)
		require.NoError(t, err)
	}
	states, err := svc.ListSubscriptionSyncRequests(ctx)
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.True(t, states[0].Synced, "pending cancellation must not be treated as executed")
	require.False(t, states[0].Executed)
	value, err := json.Marshal(request)
	require.NoError(t, err)
	_, err = client.Setting.Update().Where(setting.KeyEQ(subscriptionSyncResultPrefix + "21")).SetValue(string(value)).Save(ctx)
	require.NoError(t, err)
	states, err = svc.ListSubscriptionSyncRequests(ctx)
	require.NoError(t, err)
	require.False(t, states[0].Synced)
	require.True(t, states[0].Executed)
	request.Enabled = true
	request.UpdatedAt = now.Add(time.Second)
	value, err = json.Marshal(request)
	require.NoError(t, err)
	_, err = client.Setting.Update().Where(setting.KeyEQ(subscriptionSyncResultPrefix + "21")).SetValue(string(value)).Save(ctx)
	require.NoError(t, err)
	states, err = svc.ListSubscriptionSyncRequests(ctx)
	require.NoError(t, err)
	require.True(t, states[0].Synced)
	require.True(t, states[0].Executed, "newer cross-environment result supersedes a stale request")
	require.Equal(t, request.UpdatedAt, states[0].UpdatedAt)
	_, err = client.Setting.Delete().Where(setting.KeyEQ(subscriptionSyncPrefix + "21")).Exec(ctx)
	require.NoError(t, err)
	states, err = svc.ListSubscriptionSyncRequests(ctx)
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.True(t, states[0].Synced)
	require.True(t, states[0].Executed)
}
