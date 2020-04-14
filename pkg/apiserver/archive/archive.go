// Copyright 2020 PingCAP, Inc.
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

package archive

import (
	"github.com/gin-gonic/gin"
	"github.com/pingcap/chaos-mesh/pkg/config"
	"github.com/pingcap/chaos-mesh/pkg/store/archive"
	"github.com/pingcap/chaos-mesh/pkg/store/event"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	conf    *config.ChaosServerConfig
	kubeCli client.Client
	archive archive.ArchiveStore
	event   event.EventStore
}

func NewService(
	conf *config.ChaosServerConfig,
	cli client.Client,
	archive archive.ArchiveStore,
	event archive.ArchiveStore,
) *Service {
	return &Service{
		conf:    conf,
		kubeCli: cli,
		archive: archive,
		event:   event,
	}
}

func Register(r *gin.RouterGroup, s *Service) {
	endpoint := r.Group("/archive")

	endpoint.GET("/all", s.listArchiveExperiments)
}

func (s *Service) listArchiveExperiments(c *gin.Context) {}
