package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/faeln1/go-whatsapp-api/internal/domain/group"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var (
	ErrGroupInvalidInstanceID    = errors.New("invalid instance id")
	ErrGroupInstanceNotFound     = errors.New("instance not found")
	ErrGroupInstanceNotReady     = errors.New("instance client not ready")
	ErrGroupInstanceNotConnected = errors.New("instance not connected")
	ErrGroupInvalidParticipant   = errors.New("invalid participant")
	ErrGroupInvalidSubject       = errors.New("invalid subject")
	ErrGroupInvalidGroupJID      = errors.New("invalid group jid")
	ErrGroupInvalidImage         = errors.New("invalid image")
	ErrGroupInvalidDescription   = errors.New("invalid description")
	ErrGroupCreate               = errors.New("failed to create group")
	ErrGroupPicture              = errors.New("failed to update group picture")
	ErrGroupDescription          = errors.New("failed to update group description")
	ErrGroupInviteLink           = errors.New("failed to fetch invite link")
	ErrGroupInvalidInviteTargets = errors.New("invalid invite targets")
	ErrGroupSendInvite           = errors.New("failed to send invite")
)

type GroupService interface {
	Create(ctx context.Context, in group.CreateInput) (group.Group, error)
	UpdatePicture(ctx context.Context, in group.UpdatePictureInput) (group.Group, error)
	UpdateDescription(ctx context.Context, in group.UpdateDescriptionInput) (group.Group, error)
	FetchInvite(ctx context.Context, in group.InviteInput) (group.InviteResponse, error)
	RevokeInvite(ctx context.Context, in group.InviteInput) (group.InviteResponse, error)
	SendInvite(ctx context.Context, in group.SendInviteInput) (group.SendInviteOutput, error)
	FetchAllGroups(ctx context.Context, in group.FetchAllGroupsInput) ([]group.Group, error)
	FindGroupByJID(ctx context.Context, in group.FindGroupByJIDInput) (group.Group, error)
	FindGroupByInviteCode(ctx context.Context, in group.FindGroupByInviteCodeInput) (group.Group, error)
	FindParticipants(ctx context.Context, in group.FindParticipantsInput) (group.FindParticipantsOutput, error)
	UpdateParticipant(ctx context.Context, in group.UpdateParticipantInput) (group.Group, error)
	UpdateSetting(ctx context.Context, in group.UpdateSettingInput) (group.Group, error)
	ToggleEphemeral(ctx context.Context, in group.ToggleEphemeralInput) (group.Group, error)
	LeaveGroup(ctx context.Context, in group.LeaveGroupInput) error
}

type groupService struct {
	waMgr      *whatsapp.Manager
	httpClient *http.Client
}

func NewGroupService(waMgr *whatsapp.Manager) GroupService {
	return &groupService{
		waMgr:      waMgr,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *groupService) Create(ctx context.Context, in group.CreateInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		return empty, ErrGroupInvalidSubject
	}

	participants, err := s.parseParticipants(in.Participants)
	if err != nil {
		return empty, err
	}
	if len(participants) == 0 {
		return empty, ErrGroupInvalidParticipant
	}

	info, err := sess.Client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         subject,
		Participants: participants,
	})
	if err != nil {
		return empty, fmt.Errorf("%w: %v", ErrGroupCreate, err)
	}

	if desc := strings.TrimSpace(in.Description); desc != "" {
		if err := sess.Client.SetGroupDescription(info.JID, desc); err != nil {
			return empty, fmt.Errorf("%w: %v", ErrGroupDescription, err)
		}
	}

	if img := strings.TrimSpace(in.ProfilePicture); img != "" {
		if err := s.applyGroupPhoto(ctx, sess.Client, info.JID, img); err != nil {
			return empty, fmt.Errorf("%w: %v", ErrGroupPicture, err)
		}
	}

	if in.PromoteParticipants {
		promote := collectPromotable(info)
		if len(promote) > 0 {
			if _, err := sess.Client.UpdateGroupParticipants(info.JID, promote, whatsmeow.ParticipantChangePromote); err != nil {
				return empty, fmt.Errorf("%w: %v", ErrGroupCreate, err)
			}
		}
	}

	return s.buildGroup(sess.Client, info.JID)
}

func (s *groupService) UpdatePicture(ctx context.Context, in group.UpdatePictureInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, err
	}

	if err := s.applyGroupPhoto(ctx, sess.Client, jid, in.Image); err != nil {
		return empty, fmt.Errorf("%w: %v", ErrGroupPicture, err)
	}

	return s.buildGroup(sess.Client, jid)
}

func (s *groupService) UpdateDescription(ctx context.Context, in group.UpdateDescriptionInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, err
	}

	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		return empty, ErrGroupInvalidDescription
	}

	if err := sess.Client.SetGroupDescription(jid, desc); err != nil {
		return empty, fmt.Errorf("%w: %v", ErrGroupDescription, err)
	}

	return s.buildGroup(sess.Client, jid)
}

func (s *groupService) FetchInvite(ctx context.Context, in group.InviteInput) (group.InviteResponse, error) {
	var out group.InviteResponse
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return out, err
	}

	link, code, err := s.getInviteLink(sess.Client, jid, false)
	if err != nil {
		return out, err
	}

	out.InviteURL = link
	out.InviteCode = code
	return out, nil
}

func (s *groupService) RevokeInvite(ctx context.Context, in group.InviteInput) (group.InviteResponse, error) {
	var out group.InviteResponse
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return out, err
	}

	link, code, err := s.getInviteLink(sess.Client, jid, true)
	if err != nil {
		return out, err
	}

	out.InviteURL = link
	out.InviteCode = code
	return out, nil
}

func (s *groupService) SendInvite(ctx context.Context, in group.SendInviteInput) (group.SendInviteOutput, error) {
	var out group.SendInviteOutput
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return out, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return out, err
	}

	targets, err := s.parseInviteTargets(in.Numbers)
	if err != nil {
		return out, err
	}

	link, code, err := s.getInviteLink(sess.Client, jid, false)
	if err != nil {
		return out, err
	}

	info, err := sess.Client.GetGroupInfo(jid)
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrGroupInviteLink, err)
	}

	caption := strings.TrimSpace(in.Description)
	for _, target := range targets {
		msg := &waE2E.Message{
			GroupInviteMessage: &waE2E.GroupInviteMessage{
				GroupJID:   proto.String(jid.String()),
				InviteCode: proto.String(code),
				GroupName:  proto.String(info.GroupName.Name),
			},
		}
		if caption != "" {
			msg.GroupInviteMessage.Caption = proto.String(caption)
		}
		if _, err := sess.Client.SendMessage(ctx, target, msg); err != nil {
			return out, fmt.Errorf("%w: %v", ErrGroupSendInvite, err)
		}
	}

	out.Send = true
	out.InviteURL = link
	return out, nil
}

func (s *groupService) readySession(instanceID string) (*whatsapp.Session, error) {
	cleaned := strings.TrimSpace(instanceID)
	if cleaned == "" {
		return nil, ErrGroupInvalidInstanceID
	}
	sess, ok := s.waMgr.Get(cleaned)
	if !ok {
		return nil, ErrGroupInstanceNotFound
	}
	if sess.Client == nil {
		return nil, ErrGroupInstanceNotReady
	}
	if !sess.Client.IsConnected() {
		return nil, ErrGroupInstanceNotConnected
	}
	return sess, nil
}

func (s *groupService) parseParticipants(raw []string) ([]types.JID, error) {
	out := make([]types.JID, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, entry := range raw {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		jid, err := parseUserJID(trimmed)
		if err != nil {
			return nil, ErrGroupInvalidParticipant
		}
		key := jid.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, jid)
	}
	return out, nil
}

func (s *groupService) parseInviteTargets(raw []string) ([]types.JID, error) {
	out := make([]types.JID, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, entry := range raw {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		jid, err := parseUserJID(trimmed)
		if err != nil {
			return nil, ErrGroupInvalidInviteTargets
		}
		if jid.Server != types.DefaultUserServer {
			return nil, ErrGroupInvalidInviteTargets
		}
		key := jid.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, jid)
	}
	if len(out) == 0 {
		return nil, ErrGroupInvalidInviteTargets
	}
	return out, nil
}

func (s *groupService) applyGroupPhoto(ctx context.Context, client *whatsmeow.Client, jid types.JID, value string) error {
	data, err := s.fetchImage(ctx, value)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return ErrGroupInvalidImage
	}
	if _, err := client.SetGroupPhoto(jid, data); err != nil {
		return err
	}
	return nil
}

func (s *groupService) fetchImage(ctx context.Context, raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrGroupInvalidImage
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmed, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGroupInvalidImage, err)
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGroupInvalidImage, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("%w: unexpected status %d", ErrGroupInvalidImage, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGroupInvalidImage, err)
		}
		return data, nil
	}
	if idx := strings.Index(trimmed, ","); idx != -1 && strings.HasPrefix(trimmed[:idx], "data:") {
		trimmed = trimmed[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGroupInvalidImage, err)
	}
	return data, nil
}

func (s *groupService) buildGroup(client *whatsmeow.Client, jid types.JID) (group.Group, error) {
	info, err := client.GetGroupInfo(jid)
	if err != nil {
		return group.Group{}, err
	}
	var pictureURL string
	if pic, err := client.GetProfilePictureInfo(jid, nil); err == nil && pic != nil {
		pictureURL = pic.URL
	}
	return mapGroupInfo(info, pictureURL), nil
}

func parseUserJID(raw string) (types.JID, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return types.JID{}, ErrGroupInvalidParticipant
	}
	if strings.Contains(cleaned, "@") {
		jid, err := types.ParseJID(cleaned)
		if err != nil {
			return types.JID{}, err
		}
		return jid, nil
	}
	digits := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, cleaned)
	if digits == "" {
		return types.JID{}, ErrGroupInvalidParticipant
	}
	return types.NewJID(digits, types.DefaultUserServer), nil
}

func parseGroupJID(raw string) (types.JID, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return types.JID{}, ErrGroupInvalidGroupJID
	}
	jid, err := types.ParseJID(cleaned)
	if err != nil {
		return types.JID{}, ErrGroupInvalidGroupJID
	}
	return jid, nil
}

func collectPromotable(info *types.GroupInfo) []types.JID {
	promote := make([]types.JID, 0, len(info.Participants))
	for _, p := range info.Participants {
		if p.Error != 0 {
			continue
		}
		if p.JID == info.OwnerJID {
			continue
		}
		if p.IsAdmin || p.IsSuperAdmin {
			continue
		}
		promote = append(promote, p.JID)
	}
	return promote
}

func mapGroupInfo(info *types.GroupInfo, pictureURL string) group.Group {
	participants := make([]group.GroupParticipant, 0, len(info.Participants))
	for _, p := range info.Participants {
		participants = append(participants, mapGroupParticipant(p))
	}
	return group.Group{
		ID:                  jidString(info.JID),
		Subject:             info.GroupName.Name,
		SubjectOwner:        jidString(info.GroupName.NameSetBy),
		SubjectTime:         info.GroupName.NameSetAt,
		PictureURL:          pictureURL,
		Size:                len(info.Participants),
		Creation:            info.GroupCreated,
		Owner:               jidString(info.OwnerJID),
		Description:         info.GroupTopic.Topic,
		DescriptionID:       info.GroupTopic.TopicID,
		Restrict:            info.GroupLocked.IsLocked,
		Announce:            info.GroupAnnounce.IsAnnounce,
		Participants:        participants,
		IsCommunity:         info.GroupParent.IsParent,
		IsCommunityAnnounce: info.GroupParent.IsParent && info.GroupAnnounce.IsAnnounce,
		LinkedParent:        jidString(info.GroupLinkedParent.LinkedParentJID),
	}
}

func mapGroupParticipant(p types.GroupParticipant) group.GroupParticipant {
	return group.GroupParticipant{
		JID:          jidString(p.JID),
		Phone:        participantPhone(p),
		IsAdmin:      p.IsAdmin,
		IsSuperAdmin: p.IsSuperAdmin,
		DisplayName:  p.DisplayName,
		Error:        p.Error,
	}
}

func jidString(j types.JID) string {
	if j.IsEmpty() {
		return ""
	}
	return j.String()
}

func participantPhone(p types.GroupParticipant) string {
	switch {
	case !p.PhoneNumber.IsEmpty():
		return p.PhoneNumber.User
	case !p.JID.IsEmpty():
		return p.JID.User
	default:
		return ""
	}
}

func (s *groupService) getInviteLink(client *whatsmeow.Client, jid types.JID, reset bool) (string, string, error) {
	link, err := client.GetGroupInviteLink(jid, reset)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrGroupInviteLink, err)
	}
	code := extractInviteCode(link)
	if code == "" {
		return "", "", ErrGroupInviteLink
	}
	return link, code, nil
}

func extractInviteCode(link string) string {
	trimmed := strings.TrimSpace(link)
	if trimmed == "" {
		return ""
	}
	if parts := strings.SplitN(trimmed, "?", 2); len(parts) > 0 {
		trimmed = parts[0]
	}
	slash := strings.LastIndex(trimmed, "/")
	if slash == -1 || slash == len(trimmed)-1 {
		return ""
	}
	return trimmed[slash+1:]
}

func (s *groupService) FetchAllGroups(ctx context.Context, in group.FetchAllGroupsInput) ([]group.Group, error) {
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return nil, err
	}

	client := sess.Client
	rawGroups, err := client.GetJoinedGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch groups: %w", err)
	}

	result := make([]group.Group, 0, len(rawGroups))
	for _, info := range rawGroups {
		g, err := s.buildGroup(client, info.JID)
		if err != nil {
			// Skip groups that fail to fetch
			continue
		}
		if !in.GetParticipants {
			g.Participants = nil
		}
		result = append(result, g)
	}

	return result, nil
}

func (s *groupService) FindGroupByJID(ctx context.Context, in group.FindGroupByJIDInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, ErrGroupInvalidGroupJID
	}

	g, err := s.buildGroup(sess.Client, jid)
	if err != nil {
		return empty, fmt.Errorf("failed to fetch group info: %w", err)
	}

	return g, nil
}

func (s *groupService) FindGroupByInviteCode(ctx context.Context, in group.FindGroupByInviteCodeInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	code := strings.TrimSpace(in.InviteCode)
	if code == "" {
		return empty, errors.New("invite code is required")
	}

	info, err := sess.Client.GetGroupInfoFromLink(code)
	if err != nil {
		return empty, fmt.Errorf("failed to fetch group by invite code: %w", err)
	}

	g := group.Group{
		ID:                  info.JID.String(),
		Subject:             info.Name,
		SubjectOwner:        info.NameSetBy.String(),
		SubjectTime:         info.NameSetAt,
		Size:                len(info.Participants),
		Creation:            info.GroupCreated,
		Owner:               info.OwnerJID.String(),
		Description:         info.Topic,
		DescriptionID:       info.TopicID,
		Restrict:            info.IsLocked,
		Announce:            info.IsAnnounce,
		IsCommunity:         info.IsParent,
		IsCommunityAnnounce: info.IsDefaultSubGroup,
		LinkedParent:        info.LinkedParentJID.String(),
	}

	// Try to fetch picture URL
	pic, err := sess.Client.GetProfilePictureInfo(info.JID, nil)
	if err == nil && pic != nil {
		g.PictureURL = pic.URL
	}

	// Convert participants
	if len(info.Participants) > 0 {
		g.Participants = make([]group.GroupParticipant, 0, len(info.Participants))
		for _, p := range info.Participants {
			g.Participants = append(g.Participants, group.GroupParticipant{
				JID:          p.JID.String(),
				Phone:        participantPhone(p),
				IsAdmin:      p.IsAdmin,
				IsSuperAdmin: p.IsSuperAdmin,
				DisplayName:  p.DisplayName,
			})
		}
	}

	return g, nil
}

func (s *groupService) FindParticipants(ctx context.Context, in group.FindParticipantsInput) (group.FindParticipantsOutput, error) {
	var empty group.FindParticipantsOutput
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, ErrGroupInvalidGroupJID
	}

	info, err := sess.Client.GetGroupInfo(jid)
	if err != nil {
		return empty, fmt.Errorf("failed to fetch group info: %w", err)
	}

	out := group.FindParticipantsOutput{
		Participants: make([]group.GroupParticipant, 0, len(info.Participants)),
	}

	for _, p := range info.Participants {
		out.Participants = append(out.Participants, group.GroupParticipant{
			JID:          p.JID.String(),
			Phone:        participantPhone(p),
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
			DisplayName:  p.DisplayName,
		})
	}

	return out, nil
}

func (s *groupService) UpdateParticipant(ctx context.Context, in group.UpdateParticipantInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, ErrGroupInvalidGroupJID
	}

	participants, err := s.parseParticipants(in.Participants)
	if err != nil {
		return empty, err
	}

	if len(participants) == 0 {
		return empty, ErrGroupInvalidParticipant
	}

	action := strings.ToLower(strings.TrimSpace(in.Action))
	client := sess.Client

	switch action {
	case "add":
		_, err = client.UpdateGroupParticipants(jid, participants, whatsmeow.ParticipantChangeAdd)
	case "remove":
		_, err = client.UpdateGroupParticipants(jid, participants, whatsmeow.ParticipantChangeRemove)
	case "promote":
		_, err = client.UpdateGroupParticipants(jid, participants, whatsmeow.ParticipantChangePromote)
	case "demote":
		_, err = client.UpdateGroupParticipants(jid, participants, whatsmeow.ParticipantChangeDemote)
	default:
		return empty, errors.New("invalid action: must be add, remove, promote, or demote")
	}

	if err != nil {
		return empty, fmt.Errorf("failed to update participants: %w", err)
	}

	// Return updated group info
	return s.buildGroup(client, jid)
}

func (s *groupService) UpdateSetting(ctx context.Context, in group.UpdateSettingInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, ErrGroupInvalidGroupJID
	}

	action := strings.ToLower(strings.TrimSpace(in.Action))
	client := sess.Client

	switch action {
	case "announcement":
		err = client.SetGroupAnnounce(jid, true)
	case "not_announcement":
		err = client.SetGroupAnnounce(jid, false)
	case "locked":
		err = client.SetGroupLocked(jid, true)
	case "unlocked":
		err = client.SetGroupLocked(jid, false)
	default:
		return empty, errors.New("invalid action: must be announcement, not_announcement, locked, or unlocked")
	}

	if err != nil {
		return empty, fmt.Errorf("failed to update group setting: %w", err)
	}

	// Return updated group info
	return s.buildGroup(client, jid)
}

func (s *groupService) ToggleEphemeral(ctx context.Context, in group.ToggleEphemeralInput) (group.Group, error) {
	var empty group.Group
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return empty, err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return empty, ErrGroupInvalidGroupJID
	}

	client := sess.Client

	// Convert seconds to time.Duration
	// 0 means disable ephemeral messages
	var timer time.Duration
	if in.Expiration > 0 {
		timer = time.Duration(in.Expiration) * time.Second
	}

	err = client.SetDisappearingTimer(jid, timer, time.Now())
	if err != nil {
		return empty, fmt.Errorf("failed to toggle ephemeral: %w", err)
	}

	// Return updated group info
	return s.buildGroup(client, jid)
}

func (s *groupService) LeaveGroup(ctx context.Context, in group.LeaveGroupInput) error {
	sess, err := s.readySession(in.InstanceID)
	if err != nil {
		return err
	}

	jid, err := parseGroupJID(in.GroupJID)
	if err != nil {
		return ErrGroupInvalidGroupJID
	}

	err = sess.Client.LeaveGroup(jid)
	if err != nil {
		return fmt.Errorf("failed to leave group: %w", err)
	}

	return nil
}
