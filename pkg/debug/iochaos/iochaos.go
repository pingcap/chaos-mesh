// Copyright 2019 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package iochaos

import (
	"fmt"

	"github.com/chaos-mesh/chaos-mesh/pkg/debug/common"
)

func Debug(chaos string, ns string) error {
	chaosList, err := common.Debug("iochaos", chaos, ns)
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	for _, chaosName := range chaosList {
		fmt.Println(string(common.ColorCyan), "[CHAOSNAME]:", chaosName, string(common.ColorReset))
		if err := debugEachChaos(chaosName, ns); err != nil {
			return fmt.Errorf("debug chaos failed with: %s", err.Error())
		}
	}
	return nil
}

func debugEachChaos(chaosName string, ns string) error {
	p, err := common.GetPod("iochaos", chaosName, ns)
	if err != nil {
		return err
	}

	// print out debug info
	cmd := fmt.Sprintf("ls /proc/1/fd -al")
	out, err := common.ExecCommand(p.ChaosDaemonName, p.ChaosDaemonNamespace, cmd)
	if err != nil {
		return fmt.Errorf("run command '%s' failed with: %s", cmd, err.Error())
	}
	fmt.Println(string(common.ColorCyan), "1. [file discriptors]", string(common.ColorReset))
	common.PrintWithTab(string(out))

	cmd = fmt.Sprintf("mount")
	out, err = common.ExecCommand(p.ChaosDaemonName, p.ChaosDaemonNamespace, cmd)
	if err != nil {
		return fmt.Errorf("run command '%s' failed with: %s", cmd, err.Error())
	}
	fmt.Println(string(common.ColorCyan), "2. [mount information]", string(common.ColorReset))
	common.PrintWithTab(string(out))

	return nil
}
