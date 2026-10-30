package container

import (
	"encoding/json"
	"strconv"
	"strings"
)

// parseDockerPortsJSON parses docker/podman `inspect -f '{{json .NetworkSettings.Ports}}'`.
// Shape: {"9222/tcp":[{"HostIp":"127.0.0.1","HostPort":"49152"}], ...}
func parseDockerPortsJSON(raw string) (PublishedPorts, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" {
		return PublishedPorts{}, nil
	}
	var m map[string][]struct {
		HostIP   string `json:"HostIp"`
		HostPort string `json:"HostPort"`
	}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	out := PublishedPorts{}
	for key, bindings := range m {
		cport := strings.Split(key, "/")[0]
		cp, err := strconv.Atoi(cport)
		if err != nil || cp <= 0 {
			continue
		}
		for _, b := range bindings {
			hp, err := strconv.Atoi(strings.TrimSpace(b.HostPort))
			if err != nil || hp <= 0 {
				continue
			}
			out[cp] = hp
			break
		}
	}
	return out, nil
}
