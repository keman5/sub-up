package service

import (
	"context"
	"testing"
	"time"

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
	_, err = client.UserSubscription.Create().SetUserID(u.ID).SetGroupID(g.ID).SetStartsAt(time.Now().Add(-time.Hour)).SetExpiresAt(time.Now().Add(time.Hour)).Save(ctx)
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
