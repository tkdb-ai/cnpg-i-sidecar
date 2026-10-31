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

package config

import (
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/validation"
	"github.com/cloudnative-pg/cnpg-i/pkg/operator"
)

const (
	imageParameter         = "image"
	configMapNameParameter = "configMapName"
	mountPathParameter     = "mountPath"
	crontabPathParameter   = "crontabPath"
	exportPathParameter    = "exportPath"

	defaultMountPath   = "/etc/supercronic"
	defaultCrontabPath = "/etc/supercronic/crontab"
	defaultExportPath  = "/var/log/postgres-export"
)

// Configuration represents the plugin configuration parameters
type Configuration struct {
	// Image is the container image used for the injected sidecar. It must
	// contain the supercronic binary on $PATH - see sidecar-image/Dockerfile
	// in this repository for a buildable image. Required: there is no
	// public image to default to.
	Image string

	// ConfigMapName is the name of a ConfigMap, in the same namespace as the
	// Cluster, holding the crontab file and any scripts it invokes.
	// Required: there is no sensible default.
	ConfigMapName string

	// MountPath is where the ConfigMap is mounted inside the sidecar.
	MountPath string

	// CrontabPath is the path (inside MountPath) to the crontab file
	// passed to supercronic.
	CrontabPath string

	// ExportPath is a writable directory mounted into the sidecar (an
	// emptyDir, not shared with the postgres container) for scripts that
	// need to stage files - e.g. pulling the instance's logs via the
	// Kubernetes API (GET /api/v1/.../pods/<name>/log) and writing them out
	// as a real, truncatable file for a downstream log shipper, since the
	// pipes CNPG itself logs to cannot be read a second time or truncated.
	ExportPath string
}

// FromParameters builds a plugin configuration from the configuration parameters
func FromParameters(
	helper *common.Plugin,
) (*Configuration, []*operator.ValidationError) {
	validationErrors := make([]*operator.ValidationError, 0)

	configuration := &Configuration{
		Image:         helper.Parameters[imageParameter],
		ConfigMapName: helper.Parameters[configMapNameParameter],
		MountPath:     helper.Parameters[mountPathParameter],
		CrontabPath:   helper.Parameters[crontabPathParameter],
		ExportPath:    helper.Parameters[exportPathParameter],
	}

	if configuration.ConfigMapName == "" {
		validationErrors = append(
			validationErrors,
			validation.BuildErrorForParameter(
				helper, configMapNameParameter, "configMapName is required: it must name a "+
					"ConfigMap, in the same namespace as the Cluster, containing the crontab file "+
					"and any scripts it invokes",
			),
		)
	}

	if configuration.Image == "" {
		validationErrors = append(
			validationErrors,
			validation.BuildErrorForParameter(
				helper, imageParameter, "image is required: it must be a container image "+
					"containing the supercronic binary on $PATH (see sidecar-image/Dockerfile)",
			),
		)
	}

	configuration.applyDefaults()

	return configuration, validationErrors
}

// ValidateChanges validates the changes between the old configuration to the
// new configuration. Every field in Configuration is safe to change on an
// update: the lifecycle hook is re-evaluated on every reconciliation, so a
// new image, ConfigMap, or path just takes effect on the next Pod patch.
func ValidateChanges(
	_ *Configuration,
	_ *Configuration,
	_ *common.Plugin,
) []*operator.ValidationError {
	return nil
}

// ToParameters serialize the configuration to a map of plugin parameters
func (config *Configuration) ToParameters() (map[string]string, error) {
	return map[string]string{
		imageParameter:         config.Image,
		configMapNameParameter: config.ConfigMapName,
		mountPathParameter:     config.MountPath,
		crontabPathParameter:   config.CrontabPath,
		exportPathParameter:    config.ExportPath,
	}, nil
}

// applyDefaults fills the configuration with the defaults
func (config *Configuration) applyDefaults() {
	if config.MountPath == "" {
		config.MountPath = defaultMountPath
	}
	if config.CrontabPath == "" {
		config.CrontabPath = defaultCrontabPath
	}
	if config.ExportPath == "" {
		config.ExportPath = defaultExportPath
	}
}
