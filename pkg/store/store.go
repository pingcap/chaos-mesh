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

package store

import (
	"context"
	"go.uber.org/fx"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/pingcap/chaos-mesh/pkg/config"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	log = ctrl.Log.WithName("store")
)

type DB struct {
	*gorm.DB
}

func NewDBStore(lc fx.Lifecycle, conf *config.ChaosServerConfig) (*DB, error) {
	gormDB, err := gorm.Open(conf.Database.Driver, conf.Database.Datasource)
	if err != nil {
		log.Error(err, "failed to open DB")
		return nil, err
	}

	db := &DB{
		gormDB,
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return db.Close()
		},
	})

	return db, nil
}
