package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
)

func TestInMemoryReconcileMembersUpdatesLastSeen(t *testing.T) {
	repo := NewInMemoryCommunityMembershipRepo()
	ctx := context.Background()
	members := []community.Member{{JID: "user@s.whatsapp.net"}}

	snapshot, err := repo.ReconcileMembers(ctx, "instance", "community", members)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(snapshot.Active) != 1 {
		t.Fatalf("expected 1 active member, got %d", len(snapshot.Active))
	}
	firstSeen := snapshot.Active[0].FirstSeen
	lastSeen := snapshot.Active[0].LastSeen
	if !firstSeen.Equal(lastSeen) {
		t.Fatalf("expected first seen == last seen on first observation")
	}

	time.Sleep(10 * time.Millisecond)

	snapshot, err = repo.ReconcileMembers(ctx, "instance", "community", members)
	if err != nil {
		t.Fatalf("reconcile error: %v", err)
	}
	if len(snapshot.Active) != 1 {
		t.Fatalf("expected 1 active member, got %d", len(snapshot.Active))
	}
	if snapshot.Active[0].FirstSeen.IsZero() {
		t.Fatalf("first seen should persist")
	}
	if !snapshot.Active[0].FirstSeen.Equal(firstSeen) {
		t.Fatalf("first seen changed; expected %v got %v", firstSeen, snapshot.Active[0].FirstSeen)
	}
	if !snapshot.Active[0].LastSeen.After(lastSeen) {
		t.Fatalf("last seen should advance; previous %v current %v", lastSeen, snapshot.Active[0].LastSeen)
	}
}
