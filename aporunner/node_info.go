package aporunner

import (
	"apollo/proto/gen/models"
	"apollo/proto/gen/restcli/node"
	"context"
)

func SubmitNodeInfo(r *RunnerContext) error {
	ctx := context.Background()

	info, err := r.Docker.Client.Info(ctx)
	if err != nil {
		return err
	}

	// We seriously need something better than this to support
	// detailed more detailed info on Mac OS X hosts.
	nodeInfo := models.NodeInfo{
		UptimeSeconds: 0,
		UptimeSecondsIDLE: 0,
		RAM: models.NodeInfoRAM{
			RAMTotalMb: info.MemTotal / 1024 / 1024,
		},
		CPU: models.NodeInfoCPU{
			CPUCount: int64(info.NCPU),
		},
		Disks: models.NodeInfoDisks{
		},
	}

	params := node.NewPostNodeStateParams()
	params.NodeState = nodeInfo
	var str = "11"
	params.NodeID = &str

	_, err = r.Client.Node.PostNodeState(params, nil)
	return err
}
