package actor

import "github.com/AsynkronIT/protoactor-go/persistence"

// Provider implements persistence.Provider
type Provider struct {
	providerState persistence.ProviderState
}

// GetState returns the persistence.ProviderState associated with this provider
func (p *Provider) GetState() persistence.ProviderState {
	return p.providerState
}

func newInMemoryProvider(snapshotInterval int) persistence.Provider {
	return &Provider{
		providerState: persistence.NewInMemoryProvider(snapshotInterval),
	}
}
