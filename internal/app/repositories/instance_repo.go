package repositories

import (
	"context"
	"errors"
	"sync"

	"github.com/faeln1/go-whatsapp-api/internal/domain/instance"
)

var (
	ErrInstanceAlreadyExists = errors.New("instance already exists")
	ErrTokenAlreadyExists    = errors.New("token already exists")
	ErrInstanceNotFound      = errors.New("not found")
)

type InstanceRepository interface {
	Create(ctx context.Context, inst *instance.Instance) error
	List(ctx context.Context) ([]*instance.Instance, error)
	GetByName(ctx context.Context, name string) (*instance.Instance, error)
	Delete(ctx context.Context, name string) error
	Update(ctx context.Context, inst *instance.Instance) error
}

type inMemoryInstanceRepo struct {
	mu        sync.RWMutex
	instances map[string]*instance.Instance
}

func NewInMemoryInstanceRepo() InstanceRepository {
	return &inMemoryInstanceRepo{instances: make(map[string]*instance.Instance)}
}

func (r *inMemoryInstanceRepo) Create(ctx context.Context, inst *instance.Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.instances[inst.Name]; exists {
		return ErrInstanceAlreadyExists
	}
	r.instances[inst.Name] = inst
	return nil
}

func (r *inMemoryInstanceRepo) List(ctx context.Context) ([]*instance.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*instance.Instance, 0, len(r.instances))
	for _, v := range r.instances {
		out = append(out, v)
	}
	return out, nil
}

func (r *inMemoryInstanceRepo) GetByName(ctx context.Context, name string) (*instance.Instance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.instances[name]
	if !ok {
		return nil, ErrInstanceNotFound
	}
	return inst, nil
}

func (r *inMemoryInstanceRepo) Delete(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.instances[name]; !ok {
		return ErrInstanceNotFound
	}
	delete(r.instances, name)
	return nil
}

func (r *inMemoryInstanceRepo) Update(ctx context.Context, inst *instance.Instance) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.instances[inst.Name]; !ok {
		return ErrInstanceNotFound
	}
	r.instances[inst.Name] = inst
	return nil
}
