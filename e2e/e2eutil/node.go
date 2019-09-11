package e2eutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec"
	"github.com/stretchr/testify/require"
)

func NodeRestart(t *testing.T, nomadClient *api.Client, nodeID string) error {

	ok := NodeIsUbuntu(t, client, nodeID)
	if !ok {
		// TODO(tgross): we're checking this because we want to use
		// systemd to restart the node, but this doesn't really work
		// for dev mode targets on Linux or Ubuntu either and should
		// return an error so we can skip it.
		return fmt.Error("NodeRestart only works against ubuntu targets")
	}

	// TODO(tgross): it seems like we should be able to use a
	// a -meta flag from parameterized jobs here?
	jobTempl := `
job "restart" {
  datacenters = ["dc1"]
  type        = "batch"

  group "restart" {
    constraint {
      attribute = "${node.unique.id}"
      value = "X"
    }

    task "restart" {
      driver = "raw_exec"

      config {
        command = "systemctl"
        args    = ["restart", "nomad", "--no-block"]
      }
    }
  }
}`

	rendered := strings.Replace(jobTempl, "X", nodeID, 1)
	r := strings.NewReader(rendered)

	job, err := jobspec.Parse(r)
	require.NoError(err)

	resp, meta, err := client.Jobs().Register(job, nil)
	require.NoError(err)

	// TODO: we need to wait until it executes successfully
}

func NodeIsUbuntu(t *testing.T, nomadClient *api.Client, nodeID string) bool {

	node, _, err := nomadClient.Nodes().Info(nodeID, nil)
	require := require.New(t)
	require.NoError(err)

	//fmt.Println(node.Attributes)

	if name, ok := node.Attributes["os.name"]; ok {
		return name == "ubuntu"
	}
	return false
}
