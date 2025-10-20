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

	events := buildMembershipEvents(evt, communityJID.String(), snapshot)
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

func buildMembershipEvents(evt *events.GroupInfo, communityID string, snapshot *repositories.CommunityMembershipSnapshot) []community.MembershipEvent {
	if evt == nil || snapshot == nil {
		return nil
	}

	joined := make([]community.MembershipEvent, 0, len(evt.Join))
	left := make([]community.MembershipEvent, 0, len(evt.Leave))

	active := make(map[string]repositories.CommunityMembershipRecord)
	for _, rec := range snapshot.Active {
		active[strings.ToLower(rec.JID)] = rec
	}
	former := make(map[string]repositories.CommunityMembershipRecord)
	for _, rec := range snapshot.Former {
		former[strings.ToLower(rec.JID)] = rec
	}

	ts := evt.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	total := len(snapshot.Active)

	for _, jid := range evt.Join {
		key := strings.ToLower(jid.String())
		rec, ok := active[key]
		if !ok {
			continue
		}
		joinedAt := rec.FirstSeen
		if joinedAt.IsZero() {
			joinedAt = ts
		}
		payload := community.MembershipPayload{
			UserID:    rec.JID,
			UserName:  safeDisplayName(rec.DisplayName, rec.Phone, rec.JID),
			UserPhone: strings.TrimSpace(rec.Phone),
			JoinedAt:  &joinedAt,
		}
		joined = append(joined, community.MembershipEvent{
			Timestamp:    ts,
			CommunityID:  communityID,
			Action:       actionUserJoined,
			Payload:      payload,
			TotalMembers: total,
		})
	}

	for _, jid := range evt.Leave {
		key := strings.ToLower(jid.String())
		rec, ok := former[key]
		if !ok {
			continue
		}
		leftAt := ts
		if rec.LeftAt != nil && !rec.LeftAt.IsZero() {
			leftAt = rec.LeftAt.UTC()
		}
		payload := community.MembershipPayload{
			UserID:    rec.JID,
			UserName:  safeDisplayName(rec.DisplayName, rec.Phone, rec.JID),
			UserPhone: strings.TrimSpace(rec.Phone),
			LeftAt:    &leftAt,
		}
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
