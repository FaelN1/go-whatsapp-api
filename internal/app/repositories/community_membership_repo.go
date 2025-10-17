package repositories

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
)

// CommunityMembershipRecord represents a membership entry stored in the repository.
type CommunityMembershipRecord struct {
	JID         string
	Phone       string
	DisplayName string
	IsAdmin     bool
	FirstSeen   time.Time
	LastSeen    time.Time
	LeftAt      *time.Time
}

// CommunityMembershipSnapshot groups active and former members after reconciliation.
type CommunityMembershipSnapshot struct {
	Active []CommunityMembershipRecord
	Former []CommunityMembershipRecord
}

// CommunityMembershipRepository persists membership history for communities.
type CommunityMembershipRepository interface {
	ReconcileMembers(ctx context.Context, instanceID, communityJID string, members []community.Member) (*CommunityMembershipSnapshot, error)
}

type memoryCommunityMembershipRepo struct {
	mu    sync.RWMutex
	items map[string]map[string]*memoryMembershipRecord
}

type memoryMembershipRecord struct {
	JID         string
	Phone       string
	DisplayName string
	IsAdmin     bool
	FirstSeen   time.Time
	LastSeen    time.Time
	LeftAt      *time.Time
}

// NewInMemoryCommunityMembershipRepo returns an in-memory membership repository implementation.
func NewInMemoryCommunityMembershipRepo() CommunityMembershipRepository {
	return &memoryCommunityMembershipRepo{items: make(map[string]map[string]*memoryMembershipRecord)}
}

func (r *memoryCommunityMembershipRepo) ReconcileMembers(ctx context.Context, instanceID, communityJID string, members []community.Member) (*CommunityMembershipSnapshot, error) {
	_ = ctx
	now := time.Now().UTC()
	key := strings.TrimSpace(instanceID) + "|" + strings.TrimSpace(communityJID)

	trimmed := make(map[string]community.Member)
	for _, member := range members {
		id := strings.TrimSpace(member.JID)
		if id == "" {
			continue
		}
		existing, ok := trimmed[id]
		if !ok {
			trimmed[id] = member
			continue
		}
		// Prefer non-empty fields from the latest occurrence
		if existing.Phone == "" && member.Phone != "" {
			existing.Phone = member.Phone
		}
		if existing.DisplayName == "" && member.DisplayName != "" {
			existing.DisplayName = member.DisplayName
		}
		if member.IsAdmin {
			existing.IsAdmin = true
		}
		trimmed[id] = existing
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	bucket, ok := r.items[key]
	if !ok {
		bucket = make(map[string]*memoryMembershipRecord)
		r.items[key] = bucket
	}

	// Mark or insert active members
	for id, member := range trimmed {
		rec, exists := bucket[id]
		if exists {
			rec.Phone = member.Phone
			rec.DisplayName = member.DisplayName
			rec.IsAdmin = member.IsAdmin
			rec.LastSeen = now
			rec.LeftAt = nil
			continue
		}
		bucket[id] = &memoryMembershipRecord{
			JID:         id,
			Phone:       member.Phone,
			DisplayName: member.DisplayName,
			IsAdmin:     member.IsAdmin,
			FirstSeen:   now,
			LastSeen:    now,
		}
	}

	// Close memberships for members that are no longer active
	for id, rec := range bucket {
		if _, stillPresent := trimmed[id]; stillPresent {
			continue
		}
		if rec.LeftAt != nil {
			continue
		}
		left := now
		rec.LastSeen = now
		rec.LeftAt = &left
	}

	snapshot := &CommunityMembershipSnapshot{}
	for _, rec := range bucket {
		if rec.LeftAt == nil {
			snapshot.Active = append(snapshot.Active, CommunityMembershipRecord{
				JID:         rec.JID,
				Phone:       rec.Phone,
				DisplayName: rec.DisplayName,
				IsAdmin:     rec.IsAdmin,
				FirstSeen:   rec.FirstSeen,
				LastSeen:    rec.LastSeen,
			})
			continue
		}
		leftAt := *rec.LeftAt
		snapshot.Former = append(snapshot.Former, CommunityMembershipRecord{
			JID:         rec.JID,
			Phone:       rec.Phone,
			DisplayName: rec.DisplayName,
			IsAdmin:     rec.IsAdmin,
			FirstSeen:   rec.FirstSeen,
			LastSeen:    rec.LastSeen,
			LeftAt:      &leftAt,
		})
	}

	sort.Slice(snapshot.Active, func(i, j int) bool {
		ai := strings.ToLower(snapshot.Active[i].DisplayName)
		aj := strings.ToLower(snapshot.Active[j].DisplayName)
		if ai != aj {
			return ai < aj
		}
		return snapshot.Active[i].JID < snapshot.Active[j].JID
	})

	sort.Slice(snapshot.Former, func(i, j int) bool {
		li := snapshot.Former[i].LeftAt
		lj := snapshot.Former[j].LeftAt
		switch {
		case li == nil && lj == nil:
			return snapshot.Former[i].JID < snapshot.Former[j].JID
		case li == nil:
			return false
		case lj == nil:
			return true
		default:
			if !li.Equal(*lj) {
				return lj.Before(*li)
			}
			return snapshot.Former[i].JID < snapshot.Former[j].JID
		}
	})

	return snapshot, nil
}
