package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/faeln1/go-whatsapp-api/internal/app/repositories"
	"github.com/faeln1/go-whatsapp-api/internal/domain/community"
	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

var (
	errInstanceNotFound      = errors.New("instance not found")
	errClientNotReady        = errors.New("instance client not ready")
	errClientNotConnected    = errors.New("instance not connected")
	errCommunityNameEmpty    = errors.New("community name is required")
	errAnnouncementEmpty     = errors.New("announcement text is required")
	errAnnouncementNoTargets = errors.New("no target communities provided")
	// ErrCommunityAccessDenied is returned when the account does not have access to the target community.
	ErrCommunityAccessDenied = errors.New("community access denied")
	// ErrCommunityInviteLink is returned when WhatsApp refuses to provide or reset the invite link.
	ErrCommunityInviteLink = errors.New("failed to fetch community invite link")
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
	ListMembers(ctx context.Context, instanceID, communityJID string) (community.Members, error)
	SendAnnouncement(ctx context.Context, instanceID string, communityIDs []string, in community.SendAnnouncementInput) ([]AnnouncementResult, error)
	FetchInvite(ctx context.Context, instanceID, communityJID string, reset bool) (community.InviteResponse, error)
}

// AnnouncementResult captures the dispatch outcome for a single community announcement.
type AnnouncementResult struct {
	CommunityJID string                 `json:"communityJid"`
	TargetJID    string                 `json:"targetJid"`
	Message      message.SendTextOutput `json:"message"`
}

type communityService struct {
	waMgr          *whatsapp.Manager
	msgSvc         MessageService
	analyticsSvc   AnalyticsService
	membershipRepo repositories.CommunityMembershipRepository
	cacheMu        sync.RWMutex
	subGroupCache  map[string]subGroupCacheEntry
	cacheTTL       time.Duration
}

// NewCommunityService assembles the community service with the required dependencies.
func NewCommunityService(waMgr *whatsapp.Manager, msgSvc MessageService, analyticsSvc AnalyticsService, membershipRepo repositories.CommunityMembershipRepository) CommunityService {
	return &communityService{
		waMgr:          waMgr,
		msgSvc:         msgSvc,
		analyticsSvc:   analyticsSvc,
		membershipRepo: membershipRepo,
		subGroupCache:  make(map[string]subGroupCacheEntry),
		cacheTTL:       time.Minute,
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
		if data, err := decodeImage(ctx, img); err != nil {
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

func (s *communityService) ListMembers(ctx context.Context, instanceID, communityJID string) (community.Members, error) {
	var empty community.Members

	sess, err := s.readySession(instanceID)
	if err != nil {
		return empty, err
	}
	jid, err := parseJID(communityJID)
	if err != nil {
		return empty, err
	}
	info, err := sess.Client.GetGroupInfo(jid)
	if err != nil {
		if errors.Is(err, whatsmeow.ErrNotInGroup) {
			return empty, ErrCommunityAccessDenied
		}
		return empty, err
	}

	current := make([]community.Member, 0)
	if info.IsParent {
		participants, err := sess.Client.GetLinkedGroupsParticipants(info.JID)
		if err != nil {
			if errors.Is(err, whatsmeow.ErrNotInGroup) {
				return empty, ErrCommunityAccessDenied
			}
			return empty, err
		}
		current = make([]community.Member, 0, len(participants))
		for _, participant := range participants {
			jidStr, phone := s.resolveMemberContact(ctx, sess.Client, participant)
			current = append(current, community.Member{JID: jidStr, Phone: phone})
		}
	} else {
		current = make([]community.Member, 0, len(info.Participants))
		for _, part := range info.Participants {
			jidStr := strings.TrimSpace(part.JID.String())
			phone := ""
			if !part.PhoneNumber.IsEmpty() {
				if resolved := strings.TrimSpace(part.PhoneNumber.String()); resolved != "" {
					jidStr = resolved
				}
				phone = strings.TrimSpace(part.PhoneNumber.User)
			} else {
				resolvedJID, resolvedPhone := s.resolveMemberContact(ctx, sess.Client, part.JID)
				if trimmed := strings.TrimSpace(resolvedJID); trimmed != "" {
					jidStr = trimmed
				}
				if phone == "" {
					phone = strings.TrimSpace(resolvedPhone)
				}
			}
			if phone == "" && part.JID.Server == types.DefaultUserServer {
				phone = part.JID.User
			}
			current = append(current, community.Member{
				JID:         jidStr,
				Phone:       phone,
				IsAdmin:     part.IsAdmin,
				DisplayName: strings.TrimSpace(part.DisplayName),
			})
		}
	}

	if s.membershipRepo == nil {
		return community.Members{Current: current}, nil
	}

	snapshot, err := s.membershipRepo.ReconcileMembers(ctx, instanceID, jid.String(), current)
	if err != nil {
		return empty, err
	}

	result := community.Members{
		Current: make([]community.Member, 0, len(snapshot.Active)),
		Former:  make([]community.FormerMember, 0, len(snapshot.Former)),
	}

	for _, rec := range snapshot.Active {
		result.Current = append(result.Current, community.Member{
			JID:         rec.JID,
			Phone:       rec.Phone,
			IsAdmin:     rec.IsAdmin,
			DisplayName: strings.TrimSpace(rec.DisplayName),
			FirstSeen:   rec.FirstSeen,
			LastSeen:    rec.LastSeen,
		})
	}

	for _, rec := range snapshot.Former {
		if rec.LeftAt == nil {
			continue
		}
		result.Former = append(result.Former, community.FormerMember{
			Member: community.Member{
				JID:         rec.JID,
				Phone:       rec.Phone,
				IsAdmin:     rec.IsAdmin,
				DisplayName: strings.TrimSpace(rec.DisplayName),
				FirstSeen:   rec.FirstSeen,
				LastSeen:    rec.LastSeen,
			},
			LeftAt: rec.LeftAt.UTC(),
		})
	}

	return result, nil
}

func (s *communityService) FetchInvite(ctx context.Context, instanceID, communityJID string, reset bool) (community.InviteResponse, error) {
	var out community.InviteResponse

	sess, err := s.readySession(instanceID)
	if err != nil {
		return out, err
	}

	jid, err := parseJID(communityJID)
	if err != nil {
		return out, err
	}

	link, err := sess.Client.GetGroupInviteLink(jid, reset)
	if err != nil {
		return out, fmt.Errorf("%w: %v", ErrCommunityInviteLink, err)
	}

	code := extractInviteCode(link)
	if code == "" {
		return out, ErrCommunityInviteLink
	}

	out.InviteURL = link
	out.InviteCode = code
	return out, nil
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

func (s *communityService) SendAnnouncement(ctx context.Context, instanceID string, communityIDs []string, in community.SendAnnouncementInput) ([]AnnouncementResult, error) {
	combinedIDs := make([]string, 0, len(communityIDs)+len(in.Communities))
	combinedIDs = append(combinedIDs, communityIDs...)
	if len(in.Communities) > 0 {
		combinedIDs = append(combinedIDs, in.Communities...)
	}

	sess, err := s.readySession(instanceID)
	if err != nil {
		return nil, err
	}

	targetCommunities, err := s.prepareAnnouncementTargets(combinedIDs)
	if err != nil {
		return nil, err
	}

	mediaPayload := strings.TrimSpace(in.Media)
	hasMedia := mediaPayload != ""
	text := strings.TrimSpace(in.Text)
	caption := strings.TrimSpace(in.Caption)
	if hasMedia {
		if caption == "" {
			caption = text
		}
	} else if text == "" {
		return nil, errAnnouncementEmpty
	}

	results := make([]AnnouncementResult, 0, len(targetCommunities))
	for _, communityJID := range targetCommunities {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		target := s.resolveAnnouncementTarget(sess.Client, communityJID)
		if hasMedia {
			msg, err := s.msgSvc.SendMedia(ctx, message.SendMediaInput{
				InstanceID: instanceID,
				To:         target.String(),
				MediaType:  strings.TrimSpace(in.MediaType),
				MimeType:   strings.TrimSpace(in.MimeType),
				Caption:    caption,
				Media:      mediaPayload,
				FileName:   strings.TrimSpace(in.FileName),
			})
			if err != nil {
				return nil, err
			}

			// Rastrear mensagem enviada
			if s.analyticsSvc != nil {
				_, trackErr := s.analyticsSvc.TrackSentMessage(ctx, instanceID, communityJID.String(), msg, caption, mediaPayload, caption)
				if trackErr != nil {
					// Log error but don't fail the send
					// TODO: Add logging
				}
			}

			results = append(results, AnnouncementResult{
				CommunityJID: communityJID.String(),
				TargetJID:    target.String(),
				Message:      msg,
			})
			continue
		}

		msg, err := s.msgSvc.SendText(ctx, message.SendTextInput{
			InstanceID: instanceID,
			To:         target.String(),
			Text:       text,
		})
		if err != nil {
			return nil, err
		}

		// Rastrear mensagem enviada
		if s.analyticsSvc != nil {
			_, trackErr := s.analyticsSvc.TrackSentMessage(ctx, instanceID, communityJID.String(), msg, text, "", "")
			if trackErr != nil {
				// Log error but don't fail the send
				// TODO: Add logging
			}
		}

		results = append(results, AnnouncementResult{
			CommunityJID: communityJID.String(),
			TargetJID:    target.String(),
			Message:      msg,
		})
	}

	return results, nil
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

func (s *communityService) prepareAnnouncementTargets(rawIDs []string) ([]types.JID, error) {
	seen := make(map[string]struct{})
	ordered := make([]types.JID, 0, len(rawIDs))
	for _, item := range rawIDs {
		clean := strings.TrimSpace(item)
		if clean == "" {
			continue
		}
		jid, err := parseJID(clean)
		if err != nil {
			return nil, err
		}
		key := jid.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ordered = append(ordered, jid)
	}
	if len(ordered) == 0 {
		return nil, errAnnouncementNoTargets
	}
	return ordered, nil
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

const maxCommunityImageBytes = 5 * 1024 * 1024

var (
	errCommunityImageTooLarge = errors.New("community image exceeds 5MB limit")
	errCommunityImageInvalid  = errors.New("community image is not a valid image")
)

func decodeImage(ctx context.Context, encoded string) ([]byte, error) {
	source := strings.TrimSpace(encoded)
	if source == "" {
		return nil, nil
	}
	var (
		data   []byte
		err    error
		remote bool
	)
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		remote = true
		data, err = downloadImage(ctx, source)
		if err != nil {
			return nil, err
		}
	} else {
		if idx := strings.Index(source, ","); idx >= 0 {
			source = source[idx+1:]
		}
		data, err = base64.StdEncoding.DecodeString(source)
		if err != nil {
			data, err = base64.RawStdEncoding.DecodeString(source)
			if err != nil {
				return nil, fmt.Errorf("invalid image payload: %w", err)
			}
		}
	}

	normalized, err := normalizeCommunityImage(data)
	if err != nil {
		if remote {
			return nil, err
		}
		return nil, fmt.Errorf("invalid image payload: %w", err)
	}
	return normalized, nil
}

func downloadImage(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("image download returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > 0 && resp.ContentLength > int64(maxCommunityImageBytes) {
		return nil, errCommunityImageTooLarge
	}
	data, err := readAllLimitedCommunity(resp.Body, maxCommunityImageBytes)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readAllLimitedCommunity(r io.Reader, limit int) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errCommunityImageTooLarge
	}
	return data, nil
}

func normalizeCommunityImage(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errCommunityImageInvalid
	}

	// Decode the image (supports JPEG, PNG, GIF)
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errCommunityImageInvalid, err)
	}

	// WhatsApp has strict requirements for group/community photos
	// We need to resize and re-encode to ensure compatibility
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Resize if image is larger than 640x640 (WhatsApp's preferred max dimension)
	maxDimension := 640
	if width > maxDimension || height > maxDimension {
		img = resizeImage(img, maxDimension)
	}

	// Always re-encode to clean JPEG format compatible with WhatsApp
	// WhatsApp requires clean JPEG without EXIF/ICC profiles/specific subsampling
	// Start with high quality and reduce if necessary
	for quality := 90; quality >= 60; quality -= 5 {
		var buf bytes.Buffer
		if encodeErr := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); encodeErr != nil {
			return nil, fmt.Errorf("failed to encode community image: %w", encodeErr)
		}
		result := buf.Bytes()

		// Check if within size limit
		if len(result) <= maxCommunityImageBytes {
			return result, nil
		}
	}

	return nil, errCommunityImageTooLarge
}

// resizeImage resizes an image to fit within maxDimension while maintaining aspect ratio
func resizeImage(src image.Image, maxDimension int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions maintaining aspect ratio
	var newWidth, newHeight int
	if width > height {
		newWidth = maxDimension
		newHeight = (height * maxDimension) / width
	} else {
		newHeight = maxDimension
		newWidth = (width * maxDimension) / height
	}

	// Create new image with calculated dimensions
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Simple nearest-neighbor scaling
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := (x * width) / newWidth
			srcY := (y * height) / newHeight
			dst.Set(x, y, src.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	return dst
}
