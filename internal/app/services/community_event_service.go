package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

const (
	actionUserJoined = "user_joined"
	actionUserLeft   = "user_left"
)

// CommunityEventListener consome eventos do WhatsApp sobre mudanças na comunidade.
type CommunityEventListener interface {
	HandleGroupInfo(ctx context.Context, instanceName string, evt *events.GroupInfo)
}

type communityEventService struct {
	waMgr          *whatsapp.Manager
	membershipRepo repositories.CommunityMembershipRepository
	dispatcher     CommunityEventsDispatcher
	log            waLog.Logger
}

// NewCommunityEventService monta o serviço responsável pelos webhooks globais de comunidade.
func NewCommunityEventService(waMgr *whatsapp.Manager, membershipRepo repositories.CommunityMembershipRepository, dispatcher CommunityEventsDispatcher, log waLog.Logger) CommunityEventListener {
	if membershipRepo == nil {
		membershipRepo = repositories.NewInMemoryCommunityMembershipRepo()
	}
	return &communityEventService{
		waMgr:          waMgr,
		membershipRepo: membershipRepo,
		dispatcher:     dispatcher,
		log:            log,
	}
}

func (s *communityEventService) HandleGroupInfo(ctx context.Context, instanceName string, evt *events.GroupInfo) {
	if s == nil || evt == nil {
		return
	}
	if len(evt.Join) == 0 && len(evt.Leave) == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if s.dispatcher == nil {
		return
	}

	sess, ok := s.waMgr.Get(instanceName)
	if !ok || sess == nil || sess.Client == nil {
		return
	}

	client := sess.Client

	groupInfo, err := client.GetGroupInfo(evt.JID)
	if err != nil {
		if s.log != nil {
			s.log.Warnf("community events: falha ao carregar grupo %s: %v", evt.JID, err)
		}
		return
	}

	communityJID := groupInfo.JID
	if !groupInfo.GroupParent.IsParent {
		parent := groupInfo.GroupLinkedParent.LinkedParentJID
		if parent.IsEmpty() {
			// Não é comunidade, ignorar
			return
		}
		communityJID = parent
		groupInfo, err = client.GetGroupInfo(parent)
		if err != nil {
			if s.log != nil {
				s.log.Warnf("community events: falha ao carregar comunidade %s: %v", parent, err)
			}
			return
		}
		if !groupInfo.GroupParent.IsParent {
			return
		}
	}

	members, err := s.collectMembers(ctx, client, groupInfo)
	if err != nil {
		if s.log != nil {
			s.log.Warnf("community events: falha ao coletar membros %s: %v", communityJID, err)
		}
		return
	}

	snapshot, err := s.membershipRepo.ReconcileMembers(ctx, instanceName, communityJID.String(), members)
	if err != nil {
		if s.log != nil {
			s.log.Warnf("community events: reconcile falhou para %s: %v", communityJID, err)
		}
		return
	}

	events := s.buildMembershipEvents(ctx, client, evt, communityJID.String(), snapshot)
	if len(events) == 0 {
		return
	}

	if err := s.dispatcher.Dispatch(ctx, events); err != nil && s.log != nil {
		s.log.Warnf("community events: webhook falhou para %s: %v", communityJID, err)
	}
}

func (s *communityEventService) collectMembers(ctx context.Context, client *whatsmeow.Client, info *types.GroupInfo) ([]community.Member, error) {
	if client == nil || info == nil {
		return nil, errors.New("client ou info não disponível")
	}

	if info.GroupParent.IsParent {
		linked, err := client.GetLinkedGroupsParticipants(info.JID)
		if err != nil {
			return nil, err
		}
		members := make([]community.Member, 0, len(linked))
		for _, memberJID := range linked {
			jidStr, phone := resolveMemberContact(ctx, client, memberJID)
			if phone == "" && memberJID.Server == types.DefaultUserServer {
				phone = memberJID.User
			}
			members = append(members, community.Member{JID: jidStr, Phone: phone})
		}
		return members, nil
	}

	participants := info.Participants
	members := make([]community.Member, 0, len(participants))
	for _, part := range participants {
		jidStr, phone := resolveMemberContact(ctx, client, part.JID)
		if phone == "" && part.JID.Server == types.DefaultUserServer {
			phone = part.JID.User
		}
		display := strings.TrimSpace(part.DisplayName)
		if display == "" && !part.PhoneNumber.IsEmpty() {
			display = strings.TrimSpace(part.PhoneNumber.String())
		}
		members = append(members, community.Member{
			JID:         jidStr,
			Phone:       phone,
			IsAdmin:     part.IsAdmin || part.IsSuperAdmin,
			DisplayName: display,
		})
	}

	return members, nil
}

func (s *communityEventService) buildMembershipEvents(ctx context.Context, client *whatsmeow.Client, evt *events.GroupInfo, communityID string, snapshot *repositories.CommunityMembershipSnapshot) []community.MembershipEvent {
	if evt == nil || snapshot == nil {
		return nil
	}

	ts := evt.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	total := len(snapshot.Active)

	active := make(map[string]repositories.CommunityMembershipRecord)
	indexMembershipRecords(active, snapshot.Active)
	former := make(map[string]repositories.CommunityMembershipRecord)
	indexMembershipRecords(former, snapshot.Former)

	joined := make([]community.MembershipEvent, 0, len(evt.Join))
	for _, jid := range evt.Join {
		rec, ok := s.lookupMembershipRecord(ctx, client, jid, active)
		payload := buildMembershipPayloadFromRecord(rec, ok, jid, ts, true)
		joined = append(joined, community.MembershipEvent{
			Timestamp:    ts,
			CommunityID:  communityID,
			Action:       actionUserJoined,
			Payload:      payload,
			TotalMembers: total,
		})
	}

	left := make([]community.MembershipEvent, 0, len(evt.Leave))
	for _, jid := range evt.Leave {
		rec, ok := s.lookupMembershipRecord(ctx, client, jid, former)
		payload := buildMembershipPayloadFromRecord(rec, ok, jid, ts, false)
		left = append(left, community.MembershipEvent{
			Timestamp:    ts,
			CommunityID:  communityID,
			Action:       actionUserLeft,
			Payload:      payload,
			TotalMembers: total,
		})
	}

	return append(joined, left...)
}

func indexMembershipRecords(target map[string]repositories.CommunityMembershipRecord, records []repositories.CommunityMembershipRecord) {
	for _, rec := range records {
		addMembershipIndex(target, rec, rec.JID)
		addMembershipIndex(target, rec, strings.Split(strings.TrimSpace(rec.JID), "@")[0])
		addMembershipIndex(target, rec, rec.Phone)
	}
}

func addMembershipIndex(target map[string]repositories.CommunityMembershipRecord, rec repositories.CommunityMembershipRecord, raw string) {
	key := normalizeMembershipKey(raw)
	if key == "" {
		return
	}
	target[key] = rec
}

func normalizeMembershipKey(raw string) string {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if clean == "" {
		return ""
	}
	return clean
}

func (s *communityEventService) lookupMembershipRecord(ctx context.Context, client *whatsmeow.Client, jid types.JID, index map[string]repositories.CommunityMembershipRecord) (repositories.CommunityMembershipRecord, bool) {
	for _, key := range s.membershipKeys(ctx, client, jid) {
		if rec, ok := index[key]; ok {
			return rec, true
		}
	}
	return repositories.CommunityMembershipRecord{}, false
}

func (s *communityEventService) membershipKeys(ctx context.Context, client *whatsmeow.Client, jid types.JID) []string {
	seen := make(map[string]struct{})
	add := func(val string) {
		key := normalizeMembershipKey(val)
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
	}

	add(jid.String())
	add(jid.User)
	if string(jid.Server) != "" {
		add(jid.User + "@" + string(jid.Server))
	}

	if client != nil && jid.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, jid); err == nil && !pn.IsEmpty() {
			add(pn.String())
			add(pn.User)
			if string(pn.Server) != "" {
				add(pn.User + "@" + string(pn.Server))
			}
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	return keys
}

func buildMembershipPayloadFromRecord(rec repositories.CommunityMembershipRecord, found bool, jid types.JID, ts time.Time, isJoin bool) community.MembershipPayload {
	userID := strings.TrimSpace(jid.String())
	if userID == "" {
		userID = strings.TrimSpace(jid.User)
	}
	userName := strings.TrimSpace(jid.User)
	userPhone := ""

	if found {
		if raw := strings.TrimSpace(rec.JID); raw != "" {
			userID = raw
		}
		userName = safeDisplayName(rec.DisplayName, rec.Phone, rec.JID)
		userPhone = strings.TrimSpace(rec.Phone)
	}

	payload := community.MembershipPayload{
		UserID:    userID,
		UserName:  userName,
		UserPhone: userPhone,
	}

	if isJoin {
		joinedAt := ts
		if found {
			if !rec.FirstSeen.IsZero() {
				joinedAt = rec.FirstSeen
			}
		}
		payload.JoinedAt = &joinedAt
	} else {
		leftAt := ts
		if found && rec.LeftAt != nil && !rec.LeftAt.IsZero() {
			leftAt = rec.LeftAt.UTC()
		}
		payload.LeftAt = &leftAt
	}

	if payload.UserName == "" {
		payload.UserName = payload.UserID
	}
	if payload.UserPhone == "" && jid.Server == types.DefaultUserServer {
		payload.UserPhone = strings.TrimSpace(jid.User)
	}

	return payload
}

func safeDisplayName(name, phone, jid string) string {
	sanitized := strings.TrimSpace(name)
	if sanitized != "" {
		return sanitized
	}
	phone = strings.TrimSpace(phone)
	if phone != "" {
		return phone
	}
	return strings.TrimSpace(jid)
}

func resolveMemberContact(ctx context.Context, client *whatsmeow.Client, member types.JID) (string, string) {
	if client == nil {
		return strings.TrimSpace(member.String()), ""
	}
	if member.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, member); err == nil && !pn.IsEmpty() {
			return strings.TrimSpace(pn.String()), strings.TrimSpace(pn.User)
		}
	}
	if member.Server == types.DefaultUserServer {
		return strings.TrimSpace(member.String()), strings.TrimSpace(member.User)
	}
	return strings.TrimSpace(member.String()), ""
}
