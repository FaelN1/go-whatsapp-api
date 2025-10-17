package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var (
	errInstanceNotFound   = errors.New("instance not found")
	errClientNotReady     = errors.New("instance client not ready")
	errClientNotConnected = errors.New("instance not connected")
	errCommunityNameEmpty = errors.New("community name is required")
	errAnnouncementEmpty  = errors.New("announcement text is required")
	// ErrCommunityAccessDenied is returned when the account does not have access to the target community.
	ErrCommunityAccessDenied = errors.New("community access denied")
)

type subGroupCacheEntry struct {
	announcement types.JID
	defaultSub   types.JID
	expiresAt    time.Time
}

// CommunityService exposes operations for managing communities through WhatsApp.
type CommunityService interface {
	Create(ctx context.Context, instanceID string, in community.CreateInput) (*community.Community, error)
	Get(ctx context.Context, instanceID, communityJID string) (*community.Community, error)
	List(ctx context.Context, instanceID string) ([]community.Community, error)
	CountMembers(ctx context.Context, instanceID, communityJID string) (int, error)
	ListMembers(ctx context.Context, instanceID, communityJID string) ([]community.Member, error)
	SendAnnouncement(ctx context.Context, instanceID, communityJID, text string) (message.SendTextOutput, error)
}

type communityService struct {
	waMgr         *whatsapp.Manager
	msgSvc        MessageService
	cacheMu       sync.RWMutex
	subGroupCache map[string]subGroupCacheEntry
	cacheTTL      time.Duration
}

// NewCommunityService assembles the community service with the required dependencies.
func NewCommunityService(waMgr *whatsapp.Manager, msgSvc MessageService) CommunityService {
	return &communityService{
		waMgr:         waMgr,
		msgSvc:        msgSvc,
		subGroupCache: make(map[string]subGroupCacheEntry),
		cacheTTL:      time.Minute,
	}
}

func (s *communityService) Create(ctx context.Context, instanceID string, in community.CreateInput) (*community.Community, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errCommunityNameEmpty
	}
	sess, err := s.readySession(instanceID)
	if err != nil {
		return nil, err
	}

	participants, err := s.parseParticipants(in.Participants)
	if err != nil {
		return nil, err
	}

	req := whatsmeow.ReqCreateGroup{
		Name:         strings.TrimSpace(in.Name),
		Participants: participants,
		GroupParent: types.GroupParent{
			IsParent:                      true,
			DefaultMembershipApprovalMode: "request_required",
		},
	}

	info, err := sess.Client.CreateGroup(ctx, req)
	if err != nil {
		return nil, err
	}

	if desc := strings.TrimSpace(in.Description); desc != "" {
		if err := sess.Client.SetGroupDescription(info.JID, desc); err != nil {
			sess.Client.Log.Warnf("failed to set community description: %v", err)
		} else {
			info.Topic = desc
		}
	}

	if img := strings.TrimSpace(in.Image); img != "" {
		if data, err := decodeImage(img); err != nil {
			return nil, fmt.Errorf("invalid image payload: %w", err)
		} else if len(data) > 0 {
			if _, err := sess.Client.SetGroupPhoto(info.JID, data); err != nil {
				sess.Client.Log.Warnf("failed to set community image: %v", err)
			}
		}
	}

	announcementJID, defaultSubJID := s.resolveDefaultSubGroup(sess.Client, info.JID)
	memberCount := len(info.Participants)
	comm := toCommunity(info, memberCount, announcementJID, defaultSubJID)
	return &comm, nil
}

func (s *communityService) Get(ctx context.Context, instanceID, communityJID string) (*community.Community, error) {
	sess, err := s.readySession(instanceID)
	if err != nil {
		return nil, err
	}
	jid, err := parseJID(communityJID)
	if err != nil {
		return nil, err
	}
	info, err := sess.Client.GetGroupInfo(jid)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrNotInGroup) {
			return nil, ErrCommunityAccessDenied
		}
		return nil, err
	}
	announcementJID, defaultSubJID := s.resolveDefaultSubGroup(sess.Client, info.JID)
	memberCount, _ := s.CountMembers(ctx, instanceID, communityJID)
	comm := toCommunity(info, memberCount, announcementJID, defaultSubJID)
	return &comm, nil
}

func (s *communityService) List(ctx context.Context, instanceID string) ([]community.Community, error) {
	sess, err := s.readySession(instanceID)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	groups, err := sess.Client.GetJoinedGroups()
	if err != nil {
		return nil, err
	}
	out := make([]community.Community, 0)
	for _, info := range groups {
		if info == nil || !info.IsParent {
			continue
		}
		announcementJID, defaultSubJID := s.resolveDefaultSubGroup(sess.Client, info.JID)
		memberCount := 0
		if participants, err := sess.Client.GetLinkedGroupsParticipants(info.JID); err == nil {
			memberCount = len(participants)
		}
		out = append(out, toCommunity(info, memberCount, announcementJID, defaultSubJID))
	}
	return out, nil
}

func (s *communityService) CountMembers(ctx context.Context, instanceID, communityJID string) (int, error) {
	sess, err := s.readySession(instanceID)
	if err != nil {
		return 0, err
	}
	jid, err := parseJID(communityJID)
	if err != nil {
		return 0, err
	}
	info, err := sess.Client.GetGroupInfo(jid)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrNotInGroup) {
			return 0, ErrCommunityAccessDenied
		}
		return 0, err
	}
	if info.IsParent {
		participants, err := sess.Client.GetLinkedGroupsParticipants(info.JID)
		if err != nil {
			if errors.Is(err, whatsmeow.ErrNotInGroup) {
				return 0, ErrCommunityAccessDenied
			}
			return 0, err
		}
		return len(participants), nil
	}
	return len(info.Participants), nil
}

func (s *communityService) ListMembers(ctx context.Context, instanceID, communityJID string) ([]community.Member, error) {
	sess, err := s.readySession(instanceID)
	if err != nil {
		return nil, err
	}
	jid, err := parseJID(communityJID)
	if err != nil {
		return nil, err
	}
	info, err := sess.Client.GetGroupInfo(jid)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrNotInGroup) {
			return nil, ErrCommunityAccessDenied
		}
		return nil, err
	}
	if info.IsParent {
		participants, err := sess.Client.GetLinkedGroupsParticipants(info.JID)
		if err != nil {
			if errors.Is(err, whatsmeow.ErrNotInGroup) {
				return nil, ErrCommunityAccessDenied
			}
			return nil, err
		}
		members := make([]community.Member, 0, len(participants))
		for _, p := range participants {
			jidStr, phone := s.resolveMemberContact(ctx, sess.Client, p)
			members = append(members, community.Member{JID: jidStr, Phone: phone})
		}
		return members, nil
	}
	members := make([]community.Member, 0, len(info.Participants))
	for _, part := range info.Participants {
		jidStr := part.JID.String()
		phone := ""
		if !part.PhoneNumber.IsEmpty() {
			jidStr = part.PhoneNumber.String()
			phone = part.PhoneNumber.User
		} else {
			jidStr, phone = s.resolveMemberContact(ctx, sess.Client, part.JID)
		}
		if phone == "" && part.JID.Server == types.DefaultUserServer {
			phone = part.JID.User
		}
		members = append(members, community.Member{
			JID:         jidStr,
			Phone:       phone,
			IsAdmin:     part.IsAdmin,
			DisplayName: part.DisplayName,
		})
	}
	return members, nil
}

func (s *communityService) resolveMemberContact(ctx context.Context, client *whatsmeow.Client, member types.JID) (string, string) {
	if client == nil {
		return member.String(), ""
	}
	if member.Server == types.HiddenUserServer {
		if pn, err := client.Store.LIDs.GetPNForLID(ctx, member); err == nil && !pn.IsEmpty() {
			return pn.String(), pn.User
		}
	}
	if member.Server == types.DefaultUserServer {
		return member.String(), member.User
	}
	return member.String(), ""
}

func (s *communityService) SendAnnouncement(ctx context.Context, instanceID, communityJID, text string) (message.SendTextOutput, error) {
	if strings.TrimSpace(text) == "" {
		return message.SendTextOutput{}, errAnnouncementEmpty
	}
	sess, err := s.readySession(instanceID)
	if err != nil {
		return message.SendTextOutput{}, err
	}
	jid, err := parseJID(communityJID)
	if err != nil {
		return message.SendTextOutput{}, err
	}
	target := s.resolveAnnouncementTarget(sess.Client, jid)
	return s.msgSvc.SendText(ctx, message.SendTextInput{InstanceID: instanceID, To: target.String(), Text: text})
}

func (s *communityService) readySession(instanceID string) (*whatsapp.Session, error) {
	sess, ok := s.waMgr.Get(instanceID)
	if !ok {
		return nil, errInstanceNotFound
	}
	if sess.Client == nil {
		return nil, errClientNotReady
	}
	if !sess.Client.IsConnected() {
		return nil, errClientNotConnected
	}
	return sess, nil
}

func (s *communityService) parseParticipants(raw []string) ([]types.JID, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	participants := make([]types.JID, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		jid, err := parseJID(item)
		if err != nil {
			return nil, fmt.Errorf("invalid participant %s: %w", item, err)
		}
		participants = append(participants, jid)
	}
	return participants, nil
}

func (s *communityService) resolveDefaultSubGroup(client *whatsmeow.Client, community types.JID) (types.JID, types.JID) {
	if client == nil {
		return community, types.JID{}
	}
	key := community.String()
	now := time.Now()

	if s.cacheTTL > 0 {
		s.cacheMu.RLock()
		if entry, ok := s.subGroupCache[key]; ok && now.Before(entry.expiresAt) {
			s.cacheMu.RUnlock()
			announcement := entry.announcement
			if announcement.IsEmpty() {
				announcement = community
			}
			return announcement, entry.defaultSub
		}
		s.cacheMu.RUnlock()
	}

	announcement := types.JID{}
	defaultSub := types.JID{}
	if subGroups, err := client.GetSubGroups(community); err == nil {
		for _, sg := range subGroups {
			if sg == nil {
				continue
			}
			if sg.GroupIsDefaultSub.IsDefaultSubGroup {
				defaultSub = sg.JID
				announcement = sg.JID
				break
			}
		}
	}
	if announcement.IsEmpty() {
		announcement = community
	}

	if s.cacheTTL > 0 {
		s.cacheMu.Lock()
		s.subGroupCache[key] = subGroupCacheEntry{
			announcement: announcement,
			defaultSub:   defaultSub,
			expiresAt:    now.Add(s.cacheTTL),
		}
		s.cacheMu.Unlock()
	}

	return announcement, defaultSub
}

func (s *communityService) resolveAnnouncementTarget(client *whatsmeow.Client, community types.JID) types.JID {
	announcement, defaultSub := s.resolveDefaultSubGroup(client, community)
	if !announcement.IsEmpty() {
		return announcement
	}
	if !defaultSub.IsEmpty() {
		return defaultSub
	}
	return community
}

func toCommunity(info *types.GroupInfo, memberCount int, announcement, defaultSub types.JID) community.Community {
	if info == nil {
		return community.Community{}
	}
	comm := community.Community{
		JID:                info.JID.String(),
		Name:               info.Name,
		Description:        info.Topic,
		CreatedAt:          info.GroupCreated,
		IsLocked:           info.IsLocked,
		IsAnnouncementOnly: info.IsAnnounce,
		MemberCount:        memberCount,
	}
	if !announcement.IsEmpty() {
		comm.AnnouncementJID = announcement.String()
	}
	if !defaultSub.IsEmpty() {
		comm.DefaultSubGroupJID = defaultSub.String()
	}
	return comm
}

func parseJID(raw string) (types.JID, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return types.JID{}, errors.New("invalid jid")
	}
	if strings.Contains(clean, "@") {
		return types.ParseJID(clean)
	}
	digits := strings.Builder{}
	for _, r := range clean {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return types.JID{}, errors.New("invalid jid")
	}
	return types.NewJID(digits.String(), types.DefaultUserServer), nil
}

func decodeImage(encoded string) ([]byte, error) {
	idx := strings.Index(encoded, ",")
	if idx >= 0 {
		encoded = encoded[idx+1:]
	}
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}
