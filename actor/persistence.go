package actor

import "github.com/AsynkronIT/protoactor-go/persistence"

// implements persistence.Provider
type Provider struct {
	providerState persistence.ProviderState
}

func (p *Provider) GetState() persistence.ProviderState {
	return p.providerState
}

func newInMemoryProvider(snapshotInterval int) persistence.Provider {
	return &Provider{
		providerState: persistence.NewInMemoryProvider(snapshotInterval),
	}
}
