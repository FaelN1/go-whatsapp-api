package repositories

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
)

type postgresCommunityMembershipRepo struct {
	db *sql.DB
}

// NewPostgresCommunityMembershipRepo builds a membership repository backed by PostgreSQL.
func NewPostgresCommunityMembershipRepo(db *sql.DB) (CommunityMembershipRepository, error) {
	repo := &postgresCommunityMembershipRepo{db: db}
	if err := repo.ensureSchema(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *postgresCommunityMembershipRepo) ensureSchema() error {
	const createTable = `
        CREATE TABLE IF NOT EXISTS community_members (
            instance_id TEXT NOT NULL,
            community_jid TEXT NOT NULL,
            member_jid TEXT NOT NULL,
            phone TEXT NOT NULL DEFAULT '',
            display_name TEXT NOT NULL DEFAULT '',
            is_admin BOOLEAN NOT NULL DEFAULT FALSE,
            first_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            left_at TIMESTAMPTZ NULL,
            PRIMARY KEY (instance_id, community_jid, member_jid)
        )`
	if _, err := r.db.Exec(createTable); err != nil {
		return err
	}
	if _, err := r.db.Exec(`CREATE INDEX IF NOT EXISTS idx_community_members_left ON community_members (instance_id, community_jid, left_at)`); err != nil {
		return err
	}
	return nil
}

func (r *postgresCommunityMembershipRepo) ReconcileMembers(ctx context.Context, instanceID, communityJID string, members []community.Member) (*CommunityMembershipSnapshot, error) {
	trimmed := make(map[string]community.Member)
	for _, member := range members {
		id := strings.ToLower(strings.TrimSpace(member.JID))
		if id == "" {
			continue
		}
		member.JID = id
		if existing, ok := trimmed[id]; ok {
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
			continue
		}
		trimmed[id] = member
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	existing := make(map[string]struct {
		rawID  string
		leftAt *time.Time
	})

	rows, err := tx.QueryContext(ctx, `
        SELECT member_jid, left_at
        FROM community_members
        WHERE instance_id = $1 AND community_jid = $2
        FOR UPDATE`, instanceID, communityJID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			memberID string
			leftAt   sql.NullTime
		)
		if err := rows.Scan(&memberID, &leftAt); err != nil {
			rows.Close()
			return nil, err
		}
		key := strings.ToLower(strings.TrimSpace(memberID))
		if leftAt.Valid {
			t := leftAt.Time.UTC()
			existing[key] = struct {
				rawID  string
				leftAt *time.Time
			}{rawID: memberID, leftAt: &t}
		} else {
			existing[key] = struct {
				rawID  string
				leftAt *time.Time
			}{rawID: memberID, leftAt: nil}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	for id, member := range trimmed {
		if rec, ok := existing[id]; ok {
			if _, err := tx.ExecContext(ctx, `
                UPDATE community_members
                SET phone = $1,
                    display_name = $2,
                    is_admin = $3,
                    last_seen = $4,
                    left_at = NULL
                WHERE instance_id = $5 AND community_jid = $6 AND member_jid = $7`,
				member.Phone,
				member.DisplayName,
				member.IsAdmin,
				now,
				instanceID,
				communityJID,
				rec.rawID,
			); err != nil {
				return nil, err
			}
			delete(existing, id)
			continue
		}
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO community_members (instance_id, community_jid, member_jid, phone, display_name, is_admin, first_seen, last_seen, left_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $7, NULL)
            ON CONFLICT (instance_id, community_jid, member_jid)
            DO UPDATE SET phone = EXCLUDED.phone,
                          display_name = EXCLUDED.display_name,
                          is_admin = EXCLUDED.is_admin,
                          last_seen = EXCLUDED.last_seen,
                          left_at = NULL`,
			instanceID,
			communityJID,
			id,
			member.Phone,
			member.DisplayName,
			member.IsAdmin,
			now,
		); err != nil {
			return nil, err
		}
	}

	for _, rec := range existing {
		if rec.leftAt != nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
            UPDATE community_members
            SET left_at = $1,
                last_seen = $1
            WHERE instance_id = $2 AND community_jid = $3 AND member_jid = $4 AND left_at IS NULL`,
			now,
			instanceID,
			communityJID,
			rec.rawID,
		); err != nil {
			return nil, err
		}
	}

	snapshot := &CommunityMembershipSnapshot{}

	activeRows, err := tx.QueryContext(ctx, `
        SELECT member_jid, phone, display_name, is_admin, first_seen, last_seen
        FROM community_members
        WHERE instance_id = $1 AND community_jid = $2 AND left_at IS NULL
        ORDER BY LOWER(display_name), member_jid`, instanceID, communityJID)
	if err != nil {
		return nil, err
	}
	for activeRows.Next() {
		var rec CommunityMembershipRecord
		if err := activeRows.Scan(&rec.JID, &rec.Phone, &rec.DisplayName, &rec.IsAdmin, &rec.FirstSeen, &rec.LastSeen); err != nil {
			activeRows.Close()
			return nil, err
		}
		rec.FirstSeen = rec.FirstSeen.UTC()
		rec.LastSeen = rec.LastSeen.UTC()
		snapshot.Active = append(snapshot.Active, rec)
	}
	activeRows.Close()
	if err := activeRows.Err(); err != nil {
		return nil, err
	}

	formerRows, err := tx.QueryContext(ctx, `
        SELECT member_jid, phone, display_name, is_admin, first_seen, last_seen, left_at
        FROM community_members
        WHERE instance_id = $1 AND community_jid = $2 AND left_at IS NOT NULL
        ORDER BY left_at DESC, member_jid`, instanceID, communityJID)
	if err != nil {
		return nil, err
	}
	for formerRows.Next() {
		var (
			rec  CommunityMembershipRecord
			left time.Time
		)
		if err := formerRows.Scan(&rec.JID, &rec.Phone, &rec.DisplayName, &rec.IsAdmin, &rec.FirstSeen, &rec.LastSeen, &left); err != nil {
			formerRows.Close()
			return nil, err
		}
		rec.FirstSeen = rec.FirstSeen.UTC()
		rec.LastSeen = rec.LastSeen.UTC()
		left = left.UTC()
		rec.LeftAt = &left
		snapshot.Former = append(snapshot.Former, rec)
	}
	formerRows.Close()
	if err := formerRows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return snapshot, nil
}
