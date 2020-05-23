package plugin

import (
	"fmt"

	"github.com/go-chassis/go-archaius"
	"github.com/go-chassis/go-chassis/control"
	"github.com/go-chassis/go-chassis/core/config"
	"github.com/go-chassis/go-chassis/core/config/model"
	"github.com/go-chassis/go-chassis/core/loadbalancer"
	"github.com/go-chassis/go-chassis/core/registry"

	meshConfig "github.com/kubeedge/kubeedge/edgemesh/pkg/config"
	meshRegistry "github.com/kubeedge/kubeedge/edgemesh/pkg/plugin/registry"
)

// Install installs go-chassis plugins
func Install() {
	// service discovery
	opt := registry.Options{}
	registry.DefaultServiceDiscoveryService = meshRegistry.NewEdgeServiceDiscovery(opt)
	// load balance
	loadbalancer.InstallStrategy(meshConfig.Config.LBStrategy, func() loadbalancer.Strategy {
		switch meshConfig.Config.LBStrategy {
		case loadbalancer.StrategyRoundRobin:
			return &loadbalancer.RoundRobinStrategy{}
		case loadbalancer.StrategyRandom:
			return &loadbalancer.RandomStrategy{}
		case loadbalancer.StrategySessionStickiness:
			return &loadbalancer.SessionStickinessStrategy{}
		default:
			return &loadbalancer.RoundRobinStrategy{}
		}
	})
	// control panel
	config.GlobalDefinition = &model.GlobalCfg{
		Panel: model.ControlPanel{
			Infra: "edge",
		},
		Ssl: make(map[string]string),
	}
	opts := control.Options{
		Infra:   config.GlobalDefinition.Panel.Infra,
		Address: config.GlobalDefinition.Panel.Settings["address"],
	}
	if err := control.Init(opts); err != nil {
		_ = fmt.Errorf("failed to init control: %v", err)
	}
	// init archaius
	if err := archaius.Init(); err != nil {
		_ = fmt.Errorf("failed to init arahaius: %v", err)
	}
}
