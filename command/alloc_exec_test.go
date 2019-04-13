package command

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/hashicorp/nomad/nomad/mock"
	"github.com/hashicorp/nomad/nomad/structs"
	"github.com/hashicorp/nomad/testutil"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// static check
var _ cli.Command = &AllocExecCommand{}

func TestAllocExecCommand_Fails(t *testing.T) {
	t.Parallel()
	srv, _, url := testServer(t, false, nil)
	defer srv.Shutdown()

	cases := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			"misuse",
			[]string{"bad"},
			commandErrorText(&AllocExecCommand{}),
		},
		{
			"connection failure",
			[]string{"-address=nope", "26470238-5CF2-438F-8772-DC67CFB0705C", "/bin/bash"},
			"Error querying allocation",
		},
		{
			"not found alloc",
			[]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C", "/bin/bash"},
			"No allocation(s) with prefix or id",
		},
		{
			"too short allocis",
			[]string{"-address=" + url, "2", "/bin/bash"},
			"Alloc ID must contain at least two characters",
		},
		{
			"missing command",
			[]string{"-address=" + url, "26470238-5CF2-438F-8772-DC67CFB0705C"},
			"A command is required",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ui := new(cli.MockUi)
			cmd := &AllocExecCommand{Meta: Meta{Ui: ui}}

			code := cmd.Run(c.args)
			require.Equal(t, 1, code)

			require.Contains(t, ui.ErrorWriter.String(), c.expectedError)

			ui.ErrorWriter.Reset()
			ui.OutputWriter.Reset()

		})
	}
}

func TestAllocExecCommand_AutocompleteArgs(t *testing.T) {
	assert := assert.New(t)
	t.Parallel()

	srv, _, url := testServer(t, true, nil)
	defer srv.Shutdown()

	ui := new(cli.MockUi)
	cmd := &AllocExecCommand{Meta: Meta{Ui: ui, flagAddress: url}}

	// Create a fake alloc
	state := srv.Agent.Server().State()
	a := mock.Alloc()
	assert.Nil(state.UpsertAllocs(1000, []*structs.Allocation{a}))

	prefix := a.ID[:5]
	args := complete.Args{Last: prefix}
	predictor := cmd.AutocompleteArgs()

	res := predictor.Predict(args)
	assert.Equal(1, len(res))
	assert.Equal(a.ID, res[0])
}

func TestAllocExecCommand_Run(t *testing.T) {
	t.Parallel()
	srv, client, url := testServer(t, true, nil)
	defer srv.Shutdown()

	// Wait for a node to be ready
	testutil.WaitForResult(func() (bool, error) {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return false, err
		}

		for _, node := range nodes {
			if _, ok := node.Drivers["mock_driver"]; ok &&
				node.Status == structs.NodeStatusReady {
				return true, nil
			}
		}
		return false, fmt.Errorf("no ready nodes")
	}, func(err error) {
		require.NoError(t, err)
	})

	jobID := uuid.Generate()
	job := testJob(jobID)
	job.TaskGroups[0].Tasks[0].Config = map[string]interface{}{
		"run_for": "10s",
		"exec_command": map[string]interface{}{
			"run_for":       "1ms",
			"exit_code":     21,
			"stdout_string": "sample stdout output\n",
			"stderr_string": "sample stderr output\n",
		},
	}
	resp, _, err := client.Jobs().Register(job, nil)
	require.NoError(t, err)

	evalUi := new(cli.MockUi)
	code := waitForSuccess(evalUi, client, fullId, t, resp.EvalID)
	require.Equal(t, 0, code, "failed to get status - output: %v", evalUi.ErrorWriter.String())

	allocId := ""

	testutil.WaitForResult(func() (bool, error) {
		allocs, _, err := client.Jobs().Allocations(jobID, false, nil)
		if err != nil {
			return false, fmt.Errorf("failed to get allocations: %v", err)
		}

		if len(allocs) < 0 {
			return false, fmt.Errorf("no allocations yet")
		}

		alloc := allocs[0]
		if alloc.ClientStatus != "running" {
			return false, fmt.Errorf("alloc is not running yet: %v", alloc.ClientStatus)
		}

		allocId = alloc.ID
		return true, nil
	}, func(err error) {
		require.NoError(t, err)

	})

	ui := new(cli.MockUi)
	var stdout, stderr bufferCloser

	cmd := &AllocExecCommand{
		Meta:   Meta{Ui: ui},
		Stdin:  bytes.NewReader(nil),
		Stdout: &stdout,
		Stderr: &stderr,
	}

	code = cmd.Run([]string{"-address=" + url, allocId, "simpelcommand"})
	assert.Equal(t, 21, code)
	assert.Contains(t, stdout.String(), "sample stdout output")
	assert.Contains(t, stderr.String(), "sample stderr output")
}

type bufferCloser struct {
	bytes.Buffer
}

func (b *bufferCloser) Close() error {
	return nil
}