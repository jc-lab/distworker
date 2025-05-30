package provisioner

import (
	"github.com/jc-lab/distworker/go/pkg/controller/config"
)

type Provisioner struct {
	Name string
	config.ProvisionerSettings
}

type Manager struct {
	Provisioners map[string]*Provisioner
}

func NewManager(cfg config.ProvisionerConfig) (*Manager, error) {
	manager := &Manager{
		Provisioners: make(map[string]*Provisioner),
	}

	for name, settings := range cfg {
		manager.Provisioners[name] = &Provisioner{
			Name:                name,
			ProvisionerSettings: settings,
		}
	}

	return manager, nil
}

func (m *Manager) GetProvisioner(name string) *Provisioner {
	return m.Provisioners[name]
}
