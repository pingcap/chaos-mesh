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

package uiserver

import (
	"io"
	"net/http"

	"github.com/chaos-mesh/chaos-mesh/pkg/httpfs"
)

// Handler returns a FileServer `http.Handler` to handle http request.
func Handler(root http.FileSystem) http.Handler {
	if root != nil {
		return httpfs.FileServer(root, httpfs.FileServerOptions{IndexHTML: true, Fallback: "/index.html"})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "Dashboard UI is not built. Please run `UI=1 make`.\n")
	})
}

// AssetFS returns assets.
func AssetFS() http.FileSystem {
	return assets
}
