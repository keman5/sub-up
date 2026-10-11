package main

import "testing"

func TestApplyRequests(t *testing.T) {
	cfg := &Config{UserIDs: []int64{1, 2}, SyncSubscriptionUserIDs: []int64{2, 3}, RemoveUserIDs: []int64{4}}
	ids, subs := applyRequests(cfg, map[int64]syncRequest{2: {Enabled: false}, 4: {Enabled: true}})
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 3 || ids[2] != 4 || subs[2] || !subs[4] {
		t.Fatalf("unexpected selection: %v %v", ids, subs)
	}
	ids, subs = applyRequests(cfg, map[int64]syncRequest{2: {Enabled: false}, 3: {Enabled: false}})
	if len(ids) != 1 || ids[0] != 1 || len(subs) != 0 {
		t.Fatalf("cancellation did not override configuration: %v %v", ids, subs)
	}
}
