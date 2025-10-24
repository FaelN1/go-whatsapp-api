package whatsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"
)

// StoreFactory cria containers sqlstore por instância.
type StoreFactory struct {
	baseDir string
	log     waLog.Logger
	mu      sync.Mutex // Protege criação de stores para evitar race conditions
}

func NewStoreFactory(baseDir string, log waLog.Logger) *StoreFactory {
	return &StoreFactory{baseDir: baseDir, log: log}
}

func (f *StoreFactory) EnsureDir() error {
	return os.MkdirAll(f.baseDir, 0o755)
}

func (f *StoreFactory) NewDeviceStore(ctx context.Context, instanceName string) (*sqlstore.Container, error) {
	// Lock para evitar múltiplas inicializações simultâneas do mesmo banco
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.EnsureDir(); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(f.baseDir, fmt.Sprintf("%s.db", instanceName))
	// modernc.org/sqlite driver name is "sqlite" with optimized settings for concurrency
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate", dbPath)
	container, err := sqlstore.New(ctx, "sqlite", dsn, f.log.Sub("DB"))
	if err != nil {
		return nil, err
	}
	return container, nil
}
