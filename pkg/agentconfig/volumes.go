package agentconfig

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	core "k8s.io/api/core/v1"

	"github.com/telepresenceio/telepresence/v2/pkg/dos"
)

type MountPolicy int

const (
	// MountPolicyRemote means that the client can (or in case of a docker-run, will) mount the
	// volume using a remote file system. Unless constrained by other mechanisms, the mount will
	// be read-write.
	MountPolicyRemote MountPolicy = iota
	// MountPolicyRemoteReadOnly is like MountPolicyRemote but will enforce a read-only mount.
	MountPolicyRemoteReadOnly

	// MountPolicyLocal means that the mount will be confined to the workstation. This is typically
	// the case for /tmp.
	MountPolicyLocal

	// MountPolicyIgnore means that the mount will be completely ignored by Telepresence.
	MountPolicyIgnore
)

var mountPolicyNames = []string{"Remote", "RemoteReadonly", "Local", "Ignore"} //nolint:gochecknoglobals // constant

func (mp MountPolicy) String() string {
	if mp >= 0 && int(mp) < len(mountPolicyNames) {
		return mountPolicyNames[mp]
	}
	return "Unknown"
}

func (mp MountPolicy) MarshalJSONTo(out *jsontext.Encoder, opts json.Options) error {
	return json.MarshalEncode(out, mp.String(), opts)
}

func (mp *MountPolicy) UnmarshalJSONFrom(in *jsontext.Decoder, opts json.Options) error {
	var s string
	err := json.UnmarshalDecode(in, &s, opts)
	if err == nil {
		if ix := slices.Index(mountPolicyNames, s); ix >= 0 {
			*mp = MountPolicy(ix)
		} else {
			err = fmt.Errorf("invalid mount policy: %q", s)
		}
	}
	return err
}

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

type MountPolicies map[string]MountPolicy

func (iv MountPolicies) AddAnnotations(annotations map[string]string) (MountPolicies, error) {
	ignores, err := iv.getIgnoreAnnotations(annotations)
	if err != nil {
		return nil, err
	}
	policies, err := iv.getPolicyAnnotations(annotations)
	if err != nil {
		return nil, err
	}
	if len(ignores) == 0 && len(policies) == 0 {
		return iv, nil
	}
	mps := maps.Clone(iv)
	for key, policy := range policies {
		mps[key] = policy
	}
	for _, key := range ignores {
		mps[key] = MountPolicyIgnore
	}
	return mps, nil
}

func MountPoliciesFromRPC(mr map[string]int32) MountPolicies {
	if mr == nil {
		return nil
	}
	mps := make(MountPolicies, len(mr))
	for k, v := range mr {
		mps[k] = MountPolicy(v)
	}
	return mps
}

func (iv MountPolicies) ToRPC() map[string]int32 {
	if len(iv) == 0 {
		return nil
	}
	mr := make(map[string]int32, len(iv))
	for key, policy := range iv {
		mr[key] = int32(policy)
	}
	return mr
}

func (iv MountPolicies) getPolicyAnnotations(annotations map[string]string) (mps MountPolicies, err error) {
	vma, ok := annotations[VolumeMountPolicies]
	if !ok {
		return nil, nil
	}
	vma = strings.TrimSpace(vma)
	if len(vma) == 0 {
		return nil, nil
	}

	// Unmarshalling into the clone overwrites existing entries in the clone. This is intentional. The
	// annotation has higher priority.
	err = json.Unmarshal([]byte(vma), &mps)
	return mps, err
}

func (iv MountPolicies) getIgnoreAnnotations(annotations map[string]string) (ignores []string, err error) {
	vma, ok := annotations[InjectIgnoreVolumeMounts]
	if !ok {
		return nil, nil
	}
	vma = strings.TrimSpace(vma)
	if len(vma) == 0 {
		return nil, nil
	}

	// We accept two formats.
	// 1. A JSON []string (all entries considered to be MountPolicyIgnore)
	// 2. A comma separated []string (all entries considered to be MountPolicyIgnore)
	switch vma[0] {
	case '[':
		err = json.Unmarshal([]byte(vma), &ignores)
	default:
		ignores = strings.Split(vma, ",")
		for i, vm := range ignores {
			ignores[i] = strings.TrimSpace(vm)
		}
	}
	return ignores, err
}

func (iv MountPolicies) Get(volumeName, mountPath string) MountPolicy {
	for key, policy := range iv {
		if key == volumeName || strings.HasPrefix(mountPath, key) {
			return policy
		}
	}
	return MountPolicyRemote
}

func (a *ContainerBuilder) appendVolumeMounts(app *core.Container, cc *Container, mounts []core.VolumeMount) []core.VolumeMount {
	pfx := EnvPrefixApp + cc.EnvPrefix
	for _, m := range app.VolumeMounts {
		mp := a.Config.MountPolicies.Get(m.Name, m.MountPath)
		switch mp {
		case MountPolicyIgnore, MountPolicyLocal:
		case MountPolicyRemoteReadOnly:
			if !m.ReadOnly {
				rco := core.RecursiveReadOnlyIfPossible
				m.ReadOnly = true
				m.RecursiveReadOnly = &rco
			}
			fallthrough
		default:
			m.Name = prefixInterpolated(m.Name, pfx)
			m.MountPath = prefixInterpolated(cc.MountPoint+"/"+strings.TrimPrefix(m.MountPath, "/"), pfx)
			m.SubPath = prefixInterpolated(m.SubPath, pfx)
			m.SubPathExpr = prefixInterpolated(m.SubPathExpr, pfx)
			mounts = append(mounts, m)
		}
	}
	return mounts
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
