/*
Copyright © contributors to CloudNativePG, established as
CloudNativePG a Series of LF Projects, LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

SPDX-License-Identifier: Apache-2.0
*/

// Package metadata contains the metadata of this plugin
package metadata

import "github.com/cloudnative-pg/cnpg-i/pkg/identity"

// PluginName is the name of the plugin
const PluginName = "sidecar.tkdb.ai"

// Data is the metadata of this plugin
var Data = identity.GetPluginMetadataResponse{
	Name:          PluginName,
	Version:       "0.0.1",
	DisplayName:   "Sidecar injector",
	ProjectUrl:    "https://github.com/tkdb-ai/cnpg-i-sidecar",
	RepositoryUrl: "https://github.com/tkdb-ai/cnpg-i-sidecar",
	License:       "Proprietary",
	LicenseUrl:    "https://github.com/tkdb-ai/cnpg-i-sidecar/LICENSE",
	Maturity:      "alpha",
}
