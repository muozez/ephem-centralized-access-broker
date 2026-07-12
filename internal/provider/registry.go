package provider

import (
	"fmt"
	"os"
	"sync"

	"github.com/muozez/ephem-centralized-access-broker/internal/mtls"
)

type Registry struct {
	providers map[string]Provider
}

var (
	globalRegistry *Registry
	registryOnce   sync.Once
)

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func GetRegistry() *Registry {
	registryOnce.Do(func() {
		globalRegistry = NewRegistry()
		globalRegistry.Register(NewPostgresProvider())
		globalRegistry.Register(NewSSHProvider())
		globalRegistry.Register(NewRedisProvider())

		certDir := os.Getenv("MTLS_CERT_DIR")
		if certDir == "" {
			certDir = "certs"
		}
		_ = mtls.GenerateKeysAndCerts(certDir)
		globalRegistry.Register(NewRemoteProvider(certDir))
	})
	return globalRegistry
}

func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}
