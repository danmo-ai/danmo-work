package container

import "testing"

func TestParseDockerPortsJSONEmpty(t *testing.T) {
	ports, err := parseDockerPortsJSON("null")
	if err != nil || len(ports) != 0 {
		t.Fatalf("%v %v", ports, err)
	}
}
