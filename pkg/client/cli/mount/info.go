package mount

import (
	"context"
	"strings"

	"github.com/telepresenceio/telepresence/v2/pkg/agentconfig"
	"github.com/telepresenceio/telepresence/v2/pkg/client"
)

type Info struct {
	LocalDir  string                    `json:"local_dir,omitempty"     yaml:"local_dir,omitempty"`
	RemoteDir string                    `json:"remote_dir,omitempty"    yaml:"remote_dir,omitempty"`
	Error     string                    `json:"error,omitempty"         yaml:"error,omitempty"`
	PodIP     string                    `json:"pod_ip,omitempty"        yaml:"pod_ip,omitempty"`
	Port      int32                     `json:"port,omitempty"          yaml:"port,omitempty"`
	Mounts    agentconfig.MountPolicies `json:"mounts,omitempty"        yaml:"mounts,omitempty"`
	ReadOnly  bool                      `json:"read_only,omitempty"     yaml:"read_only,omitempty"`
}

func NewInfo(ctx context.Context, env map[string]string, ftpPort, sftpPort int32, localDir, remoteDir, podIP string, mounts agentconfig.MountPolicies, ro bool) *Info {
	var port int32
	if client.GetConfig(ctx).Intercept().UseFtp {
		port = ftpPort
	} else {
		port = sftpPort
	}
	if mounts == nil {
		// Older traffic-managers will not provide the mount-path -> mount-policy map, so we must
		// create it from the TELEPRESENCE_MOUNTS environment.
		if tpMounts := env["TELEPRESENCE_MOUNTS"]; tpMounts != "" {
			// This is a Unix path, so we cannot use filepath.SplitList
			paths := strings.Split(tpMounts, ":")
			mounts = make(agentconfig.MountPolicies, len(paths))
			mp := agentconfig.MountPolicyRemote
			if ro {
				mp = agentconfig.MountPolicyRemoteReadOnly
			}
			for _, path := range paths {
				if path == "/tmp" && mp == agentconfig.MountPolicyRemoteReadOnly {
					// A read-only mount of /tmp is probably pointless
					mounts[path] = agentconfig.MountPolicyLocal
				} else {
					mounts[path] = mp
				}
			}
		}
	}
	return &Info{
		LocalDir:  localDir,
		RemoteDir: remoteDir,
		PodIP:     podIP,
		Port:      port,
		Mounts:    mounts,
		ReadOnly:  ro,
	}
}
