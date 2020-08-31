// Copyright 2020 Chaos Mesh Authors.
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

package chaosdaemon

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"

	pb "github.com/chaos-mesh/chaos-mesh/pkg/chaosdaemon/pb"
)

const (
	// DNSServerConfFile is the default config file for DNS server
	DNSServerConfFile = "/etc/resolv.conf"
)

func (s *daemonServer) SetDNSServer(ctx context.Context,
	req *pb.SetDNSServerRequest) (*empty.Empty, error) {
	log.Info("SetDNSServer", "request", req)
	pid, err := s.crClient.GetPidFromContainerID(ctx, req.ContainerId)
	if err != nil {
		log.Error(err, "GetPidFromContainerID")
		return nil, err
	}

	if req.Enable {
		// set dns server to the chaos dns server's address

		if len(req.DnsServer) == 0 {
			return &empty.Empty{}, fmt.Errorf("invalid set dns server request %v", req)
		}

		// backup the /etc/resolv.conf
		cmd := defaultProcessBuilder("cp", DNSServerConfFile, DNSServerConfFile+".chaos.bak").
			SetMountNS(GetNsPath(pid, mountNS)).
			Build(context.Background())
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		if len(out) != 0 {
			log.Info("cmd output", "output", string(out))
		}

		// add chaos dns server to the first line of /etc/resolv.conf
		// Note: can not use sed, will execute with error `Device or resource busy`
		cmd = defaultProcessBuilder("sh", "-c", fmt.Sprintf("echo 'nameserver %s' | cat - %s > temp && cat temp > %s", req.DnsServer, DNSServerConfFile, DNSServerConfFile)).
			SetMountNS(GetNsPath(pid, mountNS)).
			Build(context.Background())
		out, err = cmd.Output()
		if err != nil {
			return nil, err
		}
		if len(out) != 0 {
			log.Info("cmd output", "output", string(out))
		}
	} else {
		// recover the dns server's address
		cmd := defaultProcessBuilder("sh", "-c", fmt.Sprintf("ls %s && cat %s.chaos.bak > %s || true", DNSServerConfFile, DNSServerConfFile, DNSServerConfFile)).
			SetMountNS(GetNsPath(pid, mountNS)).
			Build(context.Background())
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		if len(out) != 0 {
			log.Info("cmd output", "output", string(out))
		}
	}

	return &empty.Empty{}, nil
}
