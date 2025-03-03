package agentconfig

import (
	"strings"

	core "k8s.io/api/core/v1"

	"github.com/telepresenceio/telepresence/v2/pkg/dos"
)

func AgentVolumes(agentName string, pod *core.Pod) []core.Volume {
	volumes := []core.Volume{
		{
			Name: AnnotationVolumeName,
			VolumeSource: core.VolumeSource{
				DownwardAPI: &core.DownwardAPIVolumeSource{
					Items: []core.DownwardAPIVolumeFile{
						{
							FieldRef: &core.ObjectFieldSelector{
								APIVersion: "v1",
								FieldPath:  "metadata.annotations",
							},
							Path: "annotations",
						},
					},
				},
			},
		},
		{
			Name: ExportsVolumeName,
			VolumeSource: core.VolumeSource{
				EmptyDir: &core.EmptyDirVolumeSource{},
			},
		},
		{
			Name: TempVolumeName,
			VolumeSource: core.VolumeSource{
				EmptyDir: &core.EmptyDirVolumeSource{},
			},
		},
	}

	// The name of the TLS secret in the annotations might contain environment variable expansions. The expansions
	// allowed here are "$AGENT_NAME" and "$_TEL_AGENT_NAME". The latter is for backward compatibility with older
	// agents where this expansion happened in the traffic-agent.
	env := dos.MapEnv{
		"AGENT_NAME":      agentName,
		"_TEL_AGENT_NAME": agentName,
	}
	vCount := len(volumes)
	volumes = appendSecretVolume(env, TerminatingTLSSecretAnnotation, TerminatingTLSVolumeName, pod, volumes)
	volumes = appendSecretVolume(env, OriginatingTLSSecretAnnotation, OriginatingTLSVolumeName, pod, volumes)

	if vCount == len(volumes) {
		// Check for legacy names too.
		volumes = appendSecretVolume(env, LegacyTerminatingTLSSecretAnnotation, TerminatingTLSVolumeName, pod, volumes)
		volumes = appendSecretVolume(env, LegacyOriginatingTLSSecretAnnotation, OriginatingTLSVolumeName, pod, volumes)
	}
	return volumes
}

func appendSecretVolume(env dos.Env, annotation, volumeName string, pod *core.Pod, volumes []core.Volume) []core.Volume {
	if secret, ok := pod.ObjectMeta.Annotations[annotation]; ok {
		volumes = append(volumes, core.Volume{
			Name: volumeName,
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					SecretName: env.ExpandEnv(secret),
				},
			},
		})
	}
	return volumes
}

type IgnoredVolumeMounts []string

func (iv IgnoredVolumeMounts) IsVolumeIgnored(name, path string) bool {
	for _, ig := range iv {
		if name != "" && ig == name {
			return true
		}
		if path != "" && strings.HasPrefix(path, ig) {
			return true
		}
	}
	return false
}

func GetIgnoredVolumeMounts(annotations map[string]string) IgnoredVolumeMounts {
	if vma, ok := annotations[InjectIgnoreVolumeMounts]; ok {
		vmSlice := strings.Split(vma, ",")
		vms := make(IgnoredVolumeMounts, 0, len(vmSlice))
		for _, vm := range vmSlice {
			if vm = strings.TrimSpace(vm); vm != "" {
				vms = append(vms, vm)
			}
		}
		return vms
	}
	return nil
}
