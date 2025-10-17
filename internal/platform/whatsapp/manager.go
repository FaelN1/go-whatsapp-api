package whatsapp

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type Session struct {
	ID        string
	Name      string
	CreatedAt time.Time
	Client    *whatsmeow.Client
	Device    *store.Device
	QRChan    <-chan whatsmeow.QRChannelItem
	Token     string
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session // key: name
	log      waLog.Logger
	lastQR   map[string]string // name -> data:image/png;base64,... or code string
}

func NewManager(log waLog.Logger) *Manager {
	return &Manager{sessions: make(map[string]*Session), log: log, lastQR: make(map[string]string)}
}

func (m *Manager) Create(ctx context.Context, name, token string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[name]; exists {
		return nil, ErrAlreadyExists
	}
	sess := &Session{ID: uuid.NewString(), Name: name, CreatedAt: time.Now(), Token: token}
	m.sessions[name] = sess
	return sess, nil
}

// AttachClient associa um client whatsmeow já criado à sessão.
func (m *Manager) AttachClient(name string, dev *store.Device, client *whatsmeow.Client, qr <-chan whatsmeow.QRChannelItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[name]
	if !ok {
		return ErrNotFound
	}
	sess.Device = dev
	sess.Client = client
	sess.QRChan = qr
	return nil
}

// StartEventLoop registra handler básico.
func (m *Manager) StartEventLoop(sess *Session) {
	if sess.Client == nil {
		return
	}
	sess.Client.AddEventHandler(func(evt any) {
		switch e := evt.(type) {
		case *events.Connected:
			m.log.Infof("Instância %s conectada", sess.Name)
		case *events.Disconnected:
			m.log.Warnf("Instância %s desconectada", sess.Name)
		case *events.PairSuccess:
			m.log.Infof("Instância %s pareada com sucesso! JID: %s, Platform: %s", sess.Name, e.ID.String(), e.Platform)
		case *events.LoggedOut:
			m.log.Warnf("Instância %s deslogada. Motivo: %s", sess.Name, e.Reason.String())
		}
	})
}

func (m *Manager) List(ctx context.Context) []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

func (m *Manager) Get(name string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[name]
	return s, ok
}

func (m *Manager) Delete(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, name)
	delete(m.lastQR, name)
}

// GeneratePairingCode solicita ao cliente whatsmeow que gere um código de pareamento baseado em número de telefone.
// É esperado que a sessão já esteja conectada (via InitNewSession) antes da chamada.
func (m *Manager) GeneratePairingCode(ctx context.Context, name string, phone string) (string, error) {
	m.mu.RLock()
	sess, ok := m.sessions[name]
	m.mu.RUnlock()
	if !ok {
		return "", ErrNotFound
	}
	if sess.Client == nil {
		return "", ErrClientUnavailable
	}
	// Defaults match common desktop login scenario
	return sess.Client.PairPhone(ctx, phone, false, whatsmeow.PairClientChrome, "Chrome (Windows)")
}

// ValidateToken retorna a sessão associada ao token (se existir)
func (m *Manager) ValidateToken(token string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.Token == token {
			return s, true
		}
	}
	return nil, false
}

// QR cache helpers (store raw code; UI can render image elsewhere or upstream can convert)
func (m *Manager) SetLastQR(name, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[name]; !ok {
		return ErrNotFound
	}
	m.lastQR[name] = code
	return nil
}

func (m *Manager) GetLastQR(name string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.lastQR[name]
	return v, ok
}
