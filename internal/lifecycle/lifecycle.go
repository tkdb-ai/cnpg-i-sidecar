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

package lifecycle

import (
	"context"
	"errors"

	apiv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/common"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/decoder"
	"github.com/cloudnative-pg/cnpg-i-machinery/pkg/pluginhelper/object"
	"github.com/cloudnative-pg/cnpg-i/pkg/lifecycle"
	"github.com/cloudnative-pg/machinery/pkg/log"
	corev1 "k8s.io/api/core/v1"

	"github.com/tkdb-ai/cnpg-i-supercronic-sidecar/internal/config"
	"github.com/tkdb-ai/cnpg-i-supercronic-sidecar/internal/utils"
	"github.com/tkdb-ai/cnpg-i-supercronic-sidecar/pkg/metadata"
)

const (
	sidecarContainerName = "supercronic"
	scriptsVolumeName    = "supercronic-scripts"
)

// Implementation is the implementation of the lifecycle handler
type Implementation struct {
	lifecycle.UnimplementedOperatorLifecycleServer
}

// GetCapabilities exposes the lifecycle capabilities
func (impl Implementation) GetCapabilities(
	_ context.Context,
	_ *lifecycle.OperatorLifecycleCapabilitiesRequest,
) (*lifecycle.OperatorLifecycleCapabilitiesResponse, error) {
	return &lifecycle.OperatorLifecycleCapabilitiesResponse{
		LifecycleCapabilities: []*lifecycle.OperatorLifecycleCapabilities{
			{
				Group: "",
				Kind:  "Pod",
				OperationTypes: []*lifecycle.OperatorOperationType{
					{
						Type: lifecycle.OperatorOperationType_TYPE_CREATE,
					},
					{
						Type: lifecycle.OperatorOperationType_TYPE_EVALUATE,
					},
				},
			},
		},
	}, nil
}

// LifecycleHook is called by the operator before creating (or reconciling)
// the object passed in the request.
func (impl Implementation) LifecycleHook(
	ctx context.Context,
	request *lifecycle.OperatorLifecycleRequest,
) (*lifecycle.OperatorLifecycleResponse, error) {
	kind, err := utils.GetKind(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}
	operation := request.GetOperationType().GetType().Enum()
	if operation == nil {
		return nil, errors.New("no operation set")
	}

	//nolint: gocritic
	switch kind {
	case "Pod":
		switch *operation {
		case lifecycle.OperatorOperationType_TYPE_CREATE, lifecycle.OperatorOperationType_TYPE_EVALUATE:
			return impl.reconcilePod(ctx, request)
		}
	}

	return &lifecycle.OperatorLifecycleResponse{}, nil
}

// reconcilePod is called when creating (or reconciling) an instance Pod.
// It injects a long-running "native sidecar" container running supercronic,
// wired up with the same Postgres connection details the main instance
// uses, plus full access to the volumes (PGDATA, the local unix socket)
// used by the postgres container itself.
func (impl Implementation) reconcilePod(
	ctx context.Context,
	request *lifecycle.OperatorLifecycleRequest,
) (*lifecycle.OperatorLifecycleResponse, error) {
	cluster, err := decoder.DecodeClusterLenient(request.GetClusterDefinition())
	if err != nil {
		return nil, err
	}

	logger := log.FromContext(ctx).WithName("supercronic_sidecar")
	helper := common.NewPlugin(
		*cluster,
		metadata.PluginName,
	)

	configuration, valErrs := config.FromParameters(helper)
	if len(valErrs) > 0 {
		return nil, valErrs[0]
	}

	pod, err := decoder.DecodePodJSON(request.GetObjectDefinition())
	if err != nil {
		return nil, err
	}

	mutatedPod := pod.DeepCopy()

	sidecar := &corev1.Container{
		Name:            sidecarContainerName,
		Image:           configuration.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"supercronic", configuration.CrontabPath},
		Env:             connectionEnv(cluster),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      scriptsVolumeName,
				MountPath: configuration.MountPath,
				ReadOnly:  true,
			},
		},
	}

	// injectPostgresVolumeMounts=true also appends the pgdata and
	// scratch-data (local unix socket) mounts used by the postgres
	// container itself, and injects the sidecar as a native sidecar
	// (an init container with restartPolicy: Always) so it runs for the
	// whole lifetime of the Pod.
	if err := object.InjectPluginInitContainerSidecarSpec(&mutatedPod.Spec, sidecar, true); err != nil {
		return nil, err
	}

	ensureScriptsVolume(&mutatedPod.Spec, configuration.ConfigMapName)

	patch, err := object.CreatePatch(mutatedPod, pod)
	if err != nil {
		return nil, err
	}

	logger.Debug("generated patch", "content", string(patch), "configuration", configuration)

	return &lifecycle.OperatorLifecycleResponse{
		JsonPatch: patch,
	}, nil
}

// connectionEnv builds the PG* environment variables pointing at the local
// instance, using the same credentials the Cluster itself is configured
// with: the superuser if enabled (needed for most maintenance scripts),
// otherwise the application user created at bootstrap time.
func connectionEnv(cluster *apiv1.Cluster) []corev1.EnvVar {
	var secretName, userName string

	switch {
	case cluster.Spec.EnableSuperuserAccess != nil && *cluster.Spec.EnableSuperuserAccess &&
		cluster.Spec.SuperuserSecret != nil:
		secretName = cluster.Spec.SuperuserSecret.Name
		userName = "postgres"
	case cluster.Spec.Bootstrap != nil &&
		cluster.Spec.Bootstrap.InitDB != nil &&
		cluster.Spec.Bootstrap.InitDB.Secret != nil:
		secretName = cluster.Spec.Bootstrap.InitDB.Secret.Name
		userName = cluster.Spec.Bootstrap.InitDB.Owner
	}

	env := []corev1.EnvVar{
		{Name: "PGHOST", Value: "/controller/run"},
		{Name: "PGPORT", Value: "5432"},
	}

	if userName != "" {
		env = append(env, corev1.EnvVar{Name: "PGUSER", Value: userName})
	}
	if secretName != "" {
		env = append(env, corev1.EnvVar{
			Name: "PGPASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
					Key:                  "password",
				},
			},
		})
	}

	return env
}

// ensureScriptsVolume adds the ConfigMap-backed volume holding the crontab
// and scripts to the Pod spec, if it isn't already present (e.g. from a
// previous reconciliation of the same Pod object).
func ensureScriptsVolume(spec *corev1.PodSpec, configMapName string) {
	for _, v := range spec.Volumes {
		if v.Name == scriptsVolumeName {
			return
		}
	}

	// ConfigMap-mounted files default to 0644, which isn't executable. The
	// crontab calls scripts directly (e.g. "/etc/supercronic/foo.sh"), so
	// the mount needs an executable default mode.
	executableMode := int32(0o555)

	spec.Volumes = append(spec.Volumes, corev1.Volume{
		Name: scriptsVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				DefaultMode:          &executableMode,
			},
		},
	})
}
