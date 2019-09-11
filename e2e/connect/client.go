package connect

import (
	"os"

	"github.com/hashicorp/nomad/e2e/e2eutil"
	"github.com/hashicorp/nomad/e2e/framework"
	"github.com/hashicorp/nomad/helper/uuid"
	"github.com/stretchr/testify/require"
)

type ConnectClientStateE2ETest struct {
	framework.TC
	jobIds []string
}

func (tc *ConnectClientStateE2ETest) BeforeAll(f *framework.F) {
	e2eutil.WaitForLeader(f.T(), tc.Nomad())
	e2eutil.WaitForNodesReady(f.T(), tc.Nomad(), 1)
}

func (tc *ConnectClientStateE2ETest) AfterEach(f *framework.F) {
	if os.Getenv("NOMAD_TEST_SKIPCLEANUP") == "1" {
		return
	}

	for _, id := range tc.jobIds {
		tc.Nomad().Jobs().Deregister(id, true, nil)
	}
	tc.jobIds = []string{}
	tc.Nomad().System().GarbageCollect()
}

func (tc *ConnectClientStateE2ETest) TestClientRestart(f *framework.F) {
	t := f.T()
	require := require.New(t)
	jobID := "connect" + uuid.Generate()[0:8]
	tc.jobIds = append(tc.jobIds, jobID)
	client := tc.Nomad()
	allocs := e2eutil.RegisterAndWaitForAllocs(t, client,
		"connect/input/demo.nomad", jobID)
	require.Equal(2, len(allocs))

	nodeID := allocs[0].NodeID

	// DEBUG
	err = e2eutil.NodeRestart(t, client, nodeID)
	require.NoError(err)

	// fmt.Println(resp)
	// fmt.Println(meta)

	// job.ID = helper.StringToPtr(jobID)

}
