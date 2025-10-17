package whatsapp

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    waLog "go.mau.fi/whatsmeow/util/log"
    "go.mau.fi/whatsmeow/store/sqlstore"
    _ "modernc.org/sqlite"
)

// StoreFactory cria containers sqlstore por instância.
type StoreFactory struct {
    baseDir string
    log     waLog.Logger
}

func NewStoreFactory(baseDir string, log waLog.Logger) *StoreFactory {
    return &StoreFactory{baseDir: baseDir, log: log}
}

func (f *StoreFactory) EnsureDir() error {
    return os.MkdirAll(f.baseDir, 0o755)
}

func (f *StoreFactory) NewDeviceStore(ctx context.Context, instanceName string) (*sqlstore.Container, error) {
    if err := f.EnsureDir(); err != nil { return nil, err }
    dbPath := filepath.Join(f.baseDir, fmt.Sprintf("%s.db", instanceName))
    // modernc.org/sqlite driver name is "sqlite" and foreign keys via pragma
    dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)", dbPath)
    container, err := sqlstore.New(ctx, "sqlite", dsn, f.log.Sub("DB"))
    if err != nil { return nil, err }
    return container, nil
}
