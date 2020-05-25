/*
Copyright 2019 The KubeEdge Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"github.com/spf13/pflag"
	"io"

	"github.com/spf13/cobra"

	types "github.com/kubeedge/kubeedge/keadm/cmd/keadm/app/cmd/common"
	"github.com/kubeedge/kubeedge/keadm/cmd/keadm/app/cmd/util"
)

var (
	resetLongDescription = `
keadm reset command can be executed in both cloud and edge node
In cloud node it shuts down the cloud processes of KubeEdge
In edge node it shuts down the edge processes of KubeEdge
`
	resetExample = `
For cloud node:
keadm reset --kube-config=/root/.kube/config

  - kube-config is the absolute path of kubeconfig which used to secure connectivity between cloudcore and kube-apiserver

For edge node:
keadm reset
`
)

// NewKubeEdgeReset represents the reset command
func NewKubeEdgeReset(out io.Writer, init *types.InitOptions) *cobra.Command {
	if init == nil {
		init = newInitOptions()
	}

	flagVals := make(map[string]types.FlagData)
	IsEdgeNode := false

	var cmd = &cobra.Command{
		Use:     "reset",
		Short:   "Teardowns KubeEdge (cloud & edge) component",
		Long:    resetLongDescription,
		Example: resetExample,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			whoRunning, err := util.IsCloudCore()
			if err != nil {
				return err
			}
			switch whoRunning {
			case types.KubeEdgeEdgeRunning:
				IsEdgeNode = true
			case types.NoneRunning:
				return fmt.Errorf("None of KubeEdge components are running in this host")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			checkFlags := func(f *pflag.Flag) {
				util.AddToolVals(f, flagVals)
			}
			cmd.Flags().VisitAll(checkFlags)

			// Tear down cloud node. It includes
			// 1. killing cloudcore process

			// Tear down edge node. It includes
			// 1. killing edgecore process, but don't delete node from K8s
			return TearDownKubeEdge(IsEdgeNode, init)
		},
	}

	addResetFlags(cmd, init)

	return cmd
}

//newInitOptions will initialise new instance of options everytime
func newInitOptions() *types.InitOptions {
	opts := &types.InitOptions{}
	opts.KubeConfig = types.DefaultKubeConfig
	return opts
}

func addResetFlags(cmd *cobra.Command, initOpts *types.InitOptions) {
	cmd.Flags().StringVar(&initOpts.KubeConfig, types.KubeConfig, initOpts.KubeConfig,
		"Use this key to set kube-config path, eg: $HOME/.kube/config")

	cmd.Flags().StringVar(&initOpts.Master, types.Master, initOpts.Master,
		"Use this key to set K8s master address, eg: http://127.0.0.1:8080")
}

// TearDownKubeEdge will bring down either cloud or edge components,
// depending upon in which type of node it is executed
func TearDownKubeEdge(isEdgeNode bool, initOptions *types.InitOptions) error {
	var ke types.ToolsInstaller
	common := util.Common{
		KubeConfig: initOptions.KubeConfig,
		Master:     initOptions.Master,
	}
	ke = &util.KubeCloudInstTool{Common: common}
	if isEdgeNode {
		ke = &util.KubeEdgeInstTool{Common: common}
	}

	err := ke.TearDown()
	return err
}
