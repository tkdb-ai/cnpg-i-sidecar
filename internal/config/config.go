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
	"strings"

	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/validation"
	"github.com/cloudnative-pg/cnpg-i/pkg/operator"
)

const (
	imageParameter         = "image"
	containerNameParameter = "containerName"
	configMapNameParameter = "configMapName"
	mountPathParameter     = "mountPath"
	exportPathParameter    = "exportPath"

	// commandParameter, if set, overrides the sidecar's entrypoint. It is
	// run as a single shell command via "/bin/sh -c <value>", so multi-step
	// or looping commands (e.g. "while true; do ...; done") work without
	// needing a separate args list.
	commandParameter = "command"

	// envParameterPrefix: any parameter named "env.FOO" becomes an env var
	// FOO on the injected sidecar. Parameters are a flat map[string]string
	// (the CNPG-I protocol doesn't support nested/list parameter types),
	// so this prefix convention is how arbitrary extra env vars get in
	// without a new parameter per variable.
	envParameterPrefix = "env."

	defaultContainerName = "sidecar"
	defaultMountPath     = "/etc/sidecar"
	defaultExportPath    = "/var/lib/sidecar-export"
)

// Configuration represents the plugin configuration parameters
type Configuration struct {
	// Image is the container image used for the injected sidecar. Required:
	// there is no sensible default. By default no command/args are set on
	// the container - it runs its own image's ENTRYPOINT/CMD as-is - unless
	// Command overrides it.
	Image string

	// ContainerName is the name given to the injected sidecar container.
	ContainerName string

	// ConfigMapName, if set, names a ConfigMap (in the same namespace as the
	// Cluster) mounted into the sidecar at MountPath - e.g. for config
	// files or scripts the sidecar's own entrypoint reads. Optional: not
	// every sidecar needs mounted config.
	ConfigMapName string

	// MountPath is where ConfigMapName is mounted inside the sidecar, if set.
	MountPath string

	// ExportPath is a writable directory mounted into the sidecar (a plain
	// emptyDir, not shared with the postgres container) for the sidecar to
	// stage files in - e.g. pulling the instance's logs via the Kubernetes
	// API (GET /api/v1/.../pods/<name>/log) and writing them out as a real,
	// truncatable file for a downstream shipper, since the pipes CNPG
	// itself logs to cannot be read a second time or truncated.
	ExportPath string

	// Command, if set, overrides the sidecar's entrypoint entirely - run as
	// "/bin/sh -c <Command>". Leave empty to run the image's own
	// ENTRYPOINT/CMD unmodified (the original, still-supported behavior).
	Command string

	// ExtraEnv holds plain-value env vars to add to the sidecar, taken
	// from any "env.FOO" parameter. Avoid colliding with the reserved
	// names the plugin itself sets: PGHOST, PGPORT, PGUSER, PGPASSWORD,
	// POD_NAME, POD_NAMESPACE, EXPORT_PATH.
	ExtraEnv map[string]string
}

// FromParameters builds a plugin configuration from the configuration parameters
func FromParameters(
	helper *common.Plugin,
) (*Configuration, []*operator.ValidationError) {
	validationErrors := make([]*operator.ValidationError, 0)

	configuration := &Configuration{
		Image:         helper.Parameters[imageParameter],
		ContainerName: helper.Parameters[containerNameParameter],
		ConfigMapName: helper.Parameters[configMapNameParameter],
		MountPath:     helper.Parameters[mountPathParameter],
		ExportPath:    helper.Parameters[exportPathParameter],
		Command:       helper.Parameters[commandParameter],
		ExtraEnv:      make(map[string]string),
	}

	for key, value := range helper.Parameters {
		if name, ok := strings.CutPrefix(key, envParameterPrefix); ok && name != "" {
			configuration.ExtraEnv[name] = value
		}
	}

	if configuration.Image == "" {
		validationErrors = append(
			validationErrors,
			validation.BuildErrorForParameter(
				helper, imageParameter, "image is required: the container image to inject as a "+
					"sidecar",
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
	parameters := map[string]string{
		imageParameter:         config.Image,
		containerNameParameter: config.ContainerName,
		configMapNameParameter: config.ConfigMapName,
		mountPathParameter:     config.MountPath,
		exportPathParameter:    config.ExportPath,
		commandParameter:       config.Command,
	}

	// Round-trip env.* parameters: the defaulting webhook (MutateCluster)
	// replaces the Cluster's stored parameters with whatever this
	// returns, so anything left out here would get silently deleted from
	// the Cluster on the next reconciliation.
	for name, value := range config.ExtraEnv {
		parameters[envParameterPrefix+name] = value
	}

	return parameters, nil
}

// applyDefaults fills the configuration with the defaults
func (config *Configuration) applyDefaults() {
	if config.ContainerName == "" {
		config.ContainerName = defaultContainerName
	}
	if config.MountPath == "" {
		config.MountPath = defaultMountPath
	}
	if config.ExportPath == "" {
		config.ExportPath = defaultExportPath
	}
}
