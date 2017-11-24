package cluster

import (
	"golang.org/x/net/context"
	"github.com/fnproject/flow/model"
	"github.com/google/uuid"
	"github.com/golang/protobuf/ptypes"
	"time"
)

type clusterProxy struct {
	manager *Manager
}

// NewClusterProxy creates a proxy service that forwards requests to the appropriate nodes based on the request shard
func NewClusterProxy(manager *Manager) model.FlowServiceServer {
	return &clusterProxy{manager: manager}
}

func (c *clusterProxy) CreateGraph(ctx context.Context, r *model.CreateGraphRequest) (*model.CreateGraphResponse, error) {
	flowID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	r.FlowId = flowID.String()
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}

	log.Debug("Proxying create")
	return client.CreateGraph(ctx, r)
}

func (c *clusterProxy) AddStage(ctx context.Context, r *model.AddStageRequest) (*model.AddStageResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.AddStage(ctx, r)
}

func (c *clusterProxy) AddValueStage(ctx context.Context, r *model.AddCompletedValueStageRequest) (*model.AddStageResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.AddValueStage(ctx,r)

}


func (c *clusterProxy) AddInvokeFunction(ctx context.Context, r *model.AddInvokeFunctionStageRequest) (*model.AddStageResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.AddInvokeFunction(ctx, r)
}

func (c *clusterProxy) AddDelay(ctx context.Context, r *model.AddDelayStageRequest) (*model.AddStageResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.AddDelay(ctx, r)
}

func (c *clusterProxy) AwaitStageResult(ctx context.Context, r *model.AwaitStageResultRequest) (*model.AwaitStageResultResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.AwaitStageResult(ctx, r)
}

func (c *clusterProxy) CompleteStageExternally(ctx context.Context, r *model.CompleteStageExternallyRequest) (*model.CompleteStageExternallyResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.CompleteStageExternally(ctx, r)
}

func (c *clusterProxy) Commit(ctx context.Context, r *model.CommitGraphRequest) (*model.GraphRequestProcessedResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	return client.Commit(ctx, r)
}

func (c *clusterProxy) GetGraphState(ctx context.Context, r *model.GetGraphStateRequest) (*model.GetGraphStateResponse, error) {
	client, err := c.manager.GetClient(r.FlowId)
	if err != nil {
		return nil, err
	}
	log.Debug("Getting graph state")
	return client.GetGraphState(ctx, r)
}

func (c *clusterProxy) StreamLifecycle(lr *model.StreamLifecycleRequest, stream model.FlowService_StreamLifecycleServer) error {
	// TODO: implement streaming
	log.Debug("Streaming lifecycle events")

	for {
		select {
		case <-stream.Context().Done():
			return nil
		default:
			m := &model.GraphLifecycleEvent{Val: &model.GraphLifecycleEvent_GraphCreated{&model.GraphCreatedEvent{
				FlowId:"foo", FunctionId:"bar/baz", Ts: ptypes.TimestampNow()}}}
			err := stream.Send(m)
			if err != nil {
				return err
			}
			log.Debug("sent one event", m)
		}
		time.Sleep(time.Second)
	}
}

func (c *clusterProxy) StreamEvents(gr *model.StreamGraphRequest, stream model.FlowService_StreamEventsServer) error {
	client, err := c.manager.GetClient(gr.FlowId)

	if err != nil {
		return err
	}
	log.Debug("Getting graph events")
	far_end, err := client.StreamEvents(stream.Context(), gr)
	if err != nil {
		return err
	}
	for {
		event, err := far_end.Recv()
		if err != nil {
			return err
		}
		err = stream.Send(event)
		if err != nil {
			return err
		}
		log.Debug("cluster_proxy passed through one event", event)
	}
}
