package http

import (
	"encoding/json"
	stdhttp "net/http"
	"os"
	"strings"
	"sync"

	"github.com/faeln1/go-whatsapp-api/internal/app/controllers"
	"github.com/faeln1/go-whatsapp-api/internal/platform/middleware"
	"github.com/faeln1/go-whatsapp-api/internal/platform/whatsapp"
	waLog "go.mau.fi/whatsmeow/util/log"
	yaml "gopkg.in/yaml.v3"
)

type RouterConfig struct {
	InstanceCtrl  *controllers.InstanceController
	MessageCtrl   *controllers.MessageController
	CommunityCtrl *controllers.CommunityController
	WebhookCtrl   *controllers.WebhookController
	SettingsCtrl  *controllers.SettingsController
	GroupCtrl     *controllers.GroupController
	ProfileCtrl   *controllers.ProfileController
	AnalyticsCtrl *controllers.AnalyticsController
	Logger        waLog.Logger
	WAManager     *whatsapp.Manager
	SwaggerEnable bool
	MasterToken   string
}

func NewRouter(cfg RouterConfig) stdhttp.Handler {
	mux := stdhttp.NewServeMux()

	// Root endpoint - API information
	mux.HandleFunc("/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Only handle exact root path
		if r.URL.Path != "/" {
			w.WriteHeader(stdhttp.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "endpoint not found",
			})
			return
		}

		// Only allow GET method
		if r.Method != stdhttp.MethodGet {
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "method not allowed",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Get instance count
		instanceCount := 0
		if cfg.WAManager != nil {
			sessions := cfg.WAManager.List(r.Context())
			instanceCount = len(sessions)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "ok",
			"name":        "Go WhatsApp API",
			"version":     "0.1.0",
			"description": "WhatsApp API compatible with Evolution API",
			"features": map[string]bool{
				"whatsapp":    true,
				"communities": true,
				"groups":      true,
				"messages":    true,
				"webhooks":    true,
				"settings":    true,
				"profiles":    true,
			},
			"instances": map[string]interface{}{
				"count": instanceCount,
			},
			"endpoints": map[string]string{
				"health":        "/health",
				"verify_creds":  "/verify-creds",
				"documentation": "/docs",
				"openapi_yaml":  "/openapi.yaml",
				"openapi_json":  "/openapi.json",
			},
		})
	})

	mux.HandleFunc("/health", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Verify credentials endpoint
	mux.HandleFunc("/verify-creds", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodPost {
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "method not allowed",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Extract token from apikey header
		token := strings.TrimSpace(r.Header.Get("apikey"))
		if token == "" {
			// Try Authorization header as fallback
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				token = strings.TrimSpace(authHeader[7:])
			}
		}

		if token == "" {
			w.WriteHeader(stdhttp.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "missing apikey header",
			})
			return
		}

		// Check master token first
		if cfg.MasterToken != "" && token == cfg.MasterToken {
			// Return master credentials info
			w.WriteHeader(stdhttp.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"valid":        true,
				"tokenType":    "master",
				"instanceName": "master",
				"status":       "authenticated",
			})
			return
		}

		// Validate instance token
		sess, ok := cfg.WAManager.ValidateToken(token)
		if !ok || sess == nil {
			w.WriteHeader(stdhttp.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "invalid credentials",
			})
			return
		}

		// Return instance credentials info
		response := map[string]interface{}{
			"valid":        true,
			"tokenType":    "instance",
			"instanceName": sess.Name,
			"status":       "authenticated",
		}

		// Add connection status if client is available
		if sess.Client != nil {
			if sess.Client.IsConnected() {
				response["connectionStatus"] = "connected"
				if sess.Client.Store != nil && sess.Client.Store.ID != nil {
					response["phoneNumber"] = sess.Client.Store.ID.User
				}
			} else {
				response["connectionStatus"] = "disconnected"
			}
		} else {
			response["connectionStatus"] = "not_initialized"
		}

		w.WriteHeader(stdhttp.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	extractBearer := func(header string) string {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 {
			return ""
		}
		if !strings.EqualFold(parts[0], "Bearer") {
			return ""
		}
		return strings.TrimSpace(parts[1])
	}

	authorizeInstance := func(w stdhttp.ResponseWriter, r *stdhttp.Request, instance string) bool {
		token := extractBearer(r.Header.Get("Authorization"))
		if token == "" {
			token = strings.TrimSpace(r.Header.Get("apikey"))
		}
		if token == "" {
			w.WriteHeader(stdhttp.StatusUnauthorized)
			return false
		}
		// Check master token first (bypasses instance validation)
		if cfg.MasterToken != "" && token == cfg.MasterToken {
			return true
		}
		// Validate instance-specific token
		sess, ok := cfg.WAManager.ValidateToken(token)
		if !ok || sess == nil || sess.Name != instance {
			w.WriteHeader(stdhttp.StatusUnauthorized)
			return false
		}
		return true
	}

	splitSegments := func(path string) []string {
		raw := strings.Split(path, "/")
		out := make([]string, 0, len(raw))
		for _, segment := range raw {
			if segment == "" {
				continue
			}
			out = append(out, segment)
		}
		return out
	}

	// --- Documentation endpoints (if enabled) ---
	if cfg.SwaggerEnable {
		var (
			once     sync.Once
			yamlData []byte
			yamlErr  error
		)
		loadYAML := func() ([]byte, error) {
			once.Do(func() { yamlData, yamlErr = os.ReadFile("docs/openapi.yaml") })
			return yamlData, yamlErr
		}
		mux.HandleFunc("/openapi.yaml", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			data, err := loadYAML()
			if err != nil {
				w.WriteHeader(stdhttp.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.Write(data)
		})
		mux.HandleFunc("/openapi.json", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			data, err := loadYAML()
			if err != nil {
				w.WriteHeader(stdhttp.StatusNotFound)
				return
			}
			var v interface{}
			if err := yaml.Unmarshal(data, &v); err != nil {
				w.WriteHeader(stdhttp.StatusInternalServerError)
				return
			}
			// YAML lib decodes map[interface{}]interface{}; re-marshal via generic map by JSON roundtrip
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				w.WriteHeader(stdhttp.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(jsonBytes)
		})
		mux.HandleFunc("/docs", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			// Simple Swagger UI (CDN)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(`<!DOCTYPE html><html><head><title>API Docs</title><link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/></head><body><div id="swagger-ui"></div><script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script><script>window.onload=()=>{SwaggerUIBundle({url:'/openapi.yaml',dom_id:'#swagger-ui'});};</script></body></html>`))
		})
	}

	// Instance management routes (authenticated)
	instanceMux := stdhttp.NewServeMux()
	instanceMux.HandleFunc("/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Handle /instances and /instances/ (list or create)
		if r.URL.Path == "/instances" || r.URL.Path == "/instances/" {
			switch r.Method {
			case stdhttp.MethodGet:
				cfg.InstanceCtrl.List(w, r)
			case stdhttp.MethodPost:
				cfg.InstanceCtrl.Create(w, r)
			default:
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			}
			return
		}

		// Handle /instances/fetchInstances (Evolution API compatibility)
		if r.URL.Path == "/instances/fetchInstances" {
			if r.Method == stdhttp.MethodGet {
				cfg.InstanceCtrl.List(w, r)
				return
			}
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			return
		}

		// Handle /instances/create (Evolution API compatibility)
		if r.URL.Path == "/instances/create" {
			if r.Method == stdhttp.MethodPost {
				cfg.InstanceCtrl.Create(w, r)
				return
			}
			w.WriteHeader(stdhttp.StatusMethodNotAllowed)
			return
		}

		// Handle /instances/{name}/*
		if !strings.HasPrefix(r.URL.Path, "/instances/") {
			w.WriteHeader(stdhttp.StatusNotFound)
			return
		}

		// patterns: /instances/{name} ou /instances/{name}/logout
		path := r.URL.Path[len("/instances/"):]
		segments := splitSegments(path)
		if cfg.CommunityCtrl != nil && len(segments) >= 2 && segments[1] == "communities" {
			instanceName := segments[0]
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if cfg.WAManager == nil {
				w.WriteHeader(stdhttp.StatusUnauthorized)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			remainder := segments[2:]
			if len(remainder) == 0 {
				switch r.Method {
				case stdhttp.MethodGet:
					cfg.CommunityCtrl.List(w, r, instanceName)
				case stdhttp.MethodPost:
					cfg.CommunityCtrl.Create(w, r, instanceName)
				default:
					w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				}
				return
			}
			communityID := remainder[0]
			if communityID == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			subPath := remainder[1:]
			if len(subPath) == 0 {
				switch r.Method {
				case stdhttp.MethodGet:
					cfg.CommunityCtrl.Get(w, r, instanceName, communityID)
				default:
					w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				}
				return
			}
			switch subPath[0] {
			case "members":
				if len(subPath) == 1 && r.Method == stdhttp.MethodGet {
					cfg.CommunityCtrl.ListMembers(w, r, instanceName, communityID)
					return
				}
				if len(subPath) == 2 && subPath[1] == "count" && r.Method == stdhttp.MethodGet {
					cfg.CommunityCtrl.CountMembers(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			case "name":
				if len(subPath) == 1 && r.Method == stdhttp.MethodPatch {
					cfg.CommunityCtrl.UpdateName(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			case "description":
				if len(subPath) == 1 && r.Method == stdhttp.MethodPatch {
					cfg.CommunityCtrl.UpdateDescription(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			case "admins":
				if len(subPath) == 1 && r.Method == stdhttp.MethodPost {
					cfg.CommunityCtrl.PromoteAdmins(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			case "image":
				if len(subPath) == 1 && r.Method == stdhttp.MethodPatch {
					cfg.CommunityCtrl.UpdateImage(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			case "announce":
				if len(subPath) == 1 && r.Method == stdhttp.MethodPost {
					cfg.CommunityCtrl.SendAnnouncement(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			case "inviteCode":
				if len(subPath) == 1 && r.Method == stdhttp.MethodPost {
					cfg.CommunityCtrl.InviteCode(w, r, instanceName, communityID)
					return
				}
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			default:
				w.WriteHeader(stdhttp.StatusNotFound)
				return
			}
		}
		if r.Method == stdhttp.MethodDelete {
			// delete instance
			r = r.Clone(r.Context())
			cfg.InstanceCtrl.Delete(w, r)
			return
		}
		if r.Method == stdhttp.MethodPost && len(path) > 7 && path[len(path)-7:] == "logout" {
			// /instances/{name}/logout
			r = r.Clone(r.Context())
			cfg.InstanceCtrl.Logout(w, r)
			return
		}
		if r.Method == stdhttp.MethodPost && len(path) > 7 && path[len(path)-7:] == "connect" {
			// /instances/{name}/connect
			r = r.Clone(r.Context())
			cfg.InstanceCtrl.Connect(w, r)
			return
		}
		if r.Method == stdhttp.MethodPost && len(path) > 10 && path[len(path)-10:] == "disconnect" {
			// /instances/{name}/disconnect
			r = r.Clone(r.Context())
			cfg.InstanceCtrl.Disconnect(w, r)
			return
		}
		if r.Method == stdhttp.MethodGet && len(path) > 3 && path[len(path)-3:] == "qr" {
			// /instances/{name}/qr
			r = r.Clone(r.Context())
			cfg.InstanceCtrl.QR(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})

	// Apply authentication middleware to instance routes
	authenticatedInstances := middleware.BearerAuth(func(token string, r *stdhttp.Request) bool {
		// Check master token first
		if cfg.MasterToken != "" && token == cfg.MasterToken {
			return true
		}
		// Validate instance-specific token
		_, ok := cfg.WAManager.ValidateToken(token)
		return ok
	})(instanceMux)

	mux.Handle("/instances", authenticatedInstances)
	mux.Handle("/instances/", authenticatedInstances)

	messageMux := stdhttp.NewServeMux()
	messageMux.HandleFunc("/message/sendText/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendText(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendMedia/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendMedia(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendStatus/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendStatus(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendWhatsAppAudio/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendAudio(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendSticker/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendSticker(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendLocation/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendLocation(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendContact/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendContact(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendReaction/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendReaction(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendPoll/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendPoll(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendList/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendList(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})
	messageMux.HandleFunc("/message/sendButtons/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method == stdhttp.MethodPost {
			cfg.MessageCtrl.SendButtons(w, r)
			return
		}
		w.WriteHeader(stdhttp.StatusMethodNotAllowed)
	})

	authenticatedMessages := middleware.BearerAuth(func(token string, r *stdhttp.Request) bool {
		// Check master token first
		if cfg.MasterToken != "" && token == cfg.MasterToken {
			return true
		}
		// Validate instance-specific token
		_, ok := cfg.WAManager.ValidateToken(token)
		return ok
	})(messageMux)

	mux.Handle("/message/", authenticatedMessages)

	if cfg.GroupCtrl != nil {
		groupMux := stdhttp.NewServeMux()
		handleGroup := func(prefix string, handler func(stdhttp.ResponseWriter, *stdhttp.Request, string)) stdhttp.HandlerFunc {
			return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				if r.Method != stdhttp.MethodPost {
					w.WriteHeader(stdhttp.StatusMethodNotAllowed)
					return
				}
				remainder := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
				if remainder == "" || strings.Contains(remainder, "/") {
					w.WriteHeader(stdhttp.StatusBadRequest)
					return
				}
				if !authorizeInstance(w, r, remainder) {
					return
				}
				handler(w, r, remainder)
			}
		}

		groupMux.HandleFunc("/group/create/", handleGroup("/group/create/", cfg.GroupCtrl.Create))
		groupMux.HandleFunc("/group/updateGroupPicture/", handleGroup("/group/updateGroupPicture/", cfg.GroupCtrl.UpdatePicture))
		groupMux.HandleFunc("/group/updateGroupDescription/", handleGroup("/group/updateGroupDescription/", cfg.GroupCtrl.UpdateDescription))
		groupMux.HandleFunc("/group/sendInviteUrl/", handleGroup("/group/sendInviteUrl/", cfg.GroupCtrl.SendInvite))

		mux.Handle("/group/", groupMux)
	}

	if cfg.WebhookCtrl != nil {
		webhookMux := stdhttp.NewServeMux()
		webhookMux.HandleFunc("/webhook/set/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodPost {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/webhook/set/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.WebhookCtrl.Set(w, r, instanceName)
		})
		webhookMux.HandleFunc("/webhook/find/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodGet {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/webhook/find/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.WebhookCtrl.Find(w, r, instanceName)
		})
		mux.Handle("/webhook/", webhookMux)
	}

	if cfg.SettingsCtrl != nil {
		settingsMux := stdhttp.NewServeMux()
		settingsMux.HandleFunc("/settings/set/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodPost {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/settings/set/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.SettingsCtrl.Set(w, r, instanceName)
		})
		settingsMux.HandleFunc("/settings/find/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodGet {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/settings/find/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.SettingsCtrl.Find(w, r, instanceName)
		})
		mux.Handle("/settings/", settingsMux)
	}

	if cfg.ProfileCtrl != nil {
		chatMux := stdhttp.NewServeMux()

		// POST /chat/updateProfileStatus/{instance}
		chatMux.HandleFunc("/chat/updateProfileStatus/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodPost {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chat/updateProfileStatus/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.ProfileCtrl.UpdateProfileStatus(w, r, instanceName)
		})

		// POST /chat/updateProfilePicture/{instance}
		chatMux.HandleFunc("/chat/updateProfilePicture/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodPost {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chat/updateProfilePicture/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.ProfileCtrl.UpdateProfilePicture(w, r, instanceName)
		})

		// DELETE /chat/removeProfilePicture/{instance}
		chatMux.HandleFunc("/chat/removeProfilePicture/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodDelete {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chat/removeProfilePicture/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.ProfileCtrl.RemoveProfilePicture(w, r, instanceName)
		})

		// GET /chat/fetchPrivacySettings/{instance}
		chatMux.HandleFunc("/chat/fetchPrivacySettings/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodGet {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chat/fetchPrivacySettings/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.ProfileCtrl.FetchPrivacySettings(w, r, instanceName)
		})

		// POST /chat/updatePrivacySettings/{instance}
		chatMux.HandleFunc("/chat/updatePrivacySettings/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodPost {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}
			instanceName := strings.Trim(strings.TrimPrefix(r.URL.Path, "/chat/updatePrivacySettings/"), "/")
			if instanceName == "" {
				w.WriteHeader(stdhttp.StatusBadRequest)
				return
			}
			if !authorizeInstance(w, r, instanceName) {
				return
			}
			cfg.ProfileCtrl.UpdatePrivacySettings(w, r, instanceName)
		})

		mux.Handle("/chat/", chatMux)
	}

	// Analytics endpoints
	if cfg.AnalyticsCtrl != nil {
		analyticsMux := stdhttp.NewServeMux()

		// GET /analytics/messages/{trackId}/metrics
		analyticsMux.HandleFunc("/messages/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodGet {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}

			// Extract trackId from path: /analytics/messages/{trackId}/metrics
			path := strings.TrimPrefix(r.URL.Path, "/messages/")
			parts := strings.Split(path, "/")
			if len(parts) < 2 || parts[0] == "" || parts[1] != "metrics" {
				w.WriteHeader(stdhttp.StatusNotFound)
				return
			}

			trackID := parts[0]
			cfg.AnalyticsCtrl.GetMessageMetrics(w, r, trackID)
		})

		// GET /analytics/instances/{instanceId}/metrics
		analyticsMux.HandleFunc("/instances/", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			if r.Method != stdhttp.MethodGet {
				w.WriteHeader(stdhttp.StatusMethodNotAllowed)
				return
			}

			// Extract instanceId from path: /analytics/instances/{instanceId}/metrics
			path := strings.TrimPrefix(r.URL.Path, "/instances/")
			parts := strings.Split(path, "/")
			if len(parts) < 2 || parts[0] == "" || parts[1] != "metrics" {
				w.WriteHeader(stdhttp.StatusNotFound)
				return
			}

			instanceID := parts[0]
			cfg.AnalyticsCtrl.GetInstanceMetrics(w, r, instanceID)
		})

		mux.Handle("/analytics/", stdhttp.StripPrefix("/analytics", analyticsMux))
	}

	// Middlewares wrap
	var handler stdhttp.Handler = mux
	handler = middleware.Logging(cfg.Logger)(handler)
	handler = middleware.CORS(handler) // Apply CORS to all routes
	return handler
}
