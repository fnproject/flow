package actor

import (
	"reflect"

	"github.com/AsynkronIT/protoactor-go/actor"
	"github.com/AsynkronIT/protoactor-go/persistence"
	"github.com/fnproject/completer/graph"
	"github.com/fnproject/completer/model"
	proto "github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("logger", "actor")
)

type graphSupervisor struct {
}

func (s *graphSupervisor) Receive(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		child, err := spawnGraphActor(msg.GraphId, context)
		if err != nil {
			log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Warn("Failed to spawn graph actor")
			return
		}
		child.Request(msg, context.Sender())

	default:
		if isGraphMessage(msg) {
			graphId := getGraphId(msg)
			child, found := findChild(context, graphId)
			if !found {
				log.WithFields(logrus.Fields{"graph_id": graphId}).Warn("No child actor found")
				context.Respond(NewGraphNotFoundError(graphId))
				return
			}
			child.Request(msg, context.Sender())
		}
	}
}

func isGraphMessage(msg interface{}) bool {
	return reflect.ValueOf(msg).Elem().FieldByName("GraphId").IsValid()
}

func getGraphId(msg interface{}) string {
	return reflect.ValueOf(msg).Elem().FieldByName("GraphId").String()
}

func findChild(context actor.Context, graphId string) (*actor.PID, bool) {
	fullId := context.Self().Id + "/" + graphId
	for _, pid := range context.Children() {
		if pid.Id == fullId {
			return pid, true
		}
	}
	return nil, false
}

// implements persistence.Provider
type Provider struct {
	providerState persistence.ProviderState
}

func (p *Provider) GetState() persistence.ProviderState {
	return p.providerState
}

func newInMemoryProvider(snapshotInterval int) persistence.Provider {
	return &Provider{
		providerState: persistence.NewInMemoryProvider(snapshotInterval),
	}
}

func spawnGraphActor(graphId string, context actor.Context) (*actor.PID, error) {
	provider := newInMemoryProvider(1)
	props := actor.FromInstance(&graphActor{}).WithMiddleware(persistence.Using(provider))
	return context.SpawnNamed(props, graphId)
}

type graphActor struct {
	persistence.Mixin
	graph *graph.CompletionGraph
}

func (g *graphActor) persist(event proto.Message) error {
	g.PersistReceive(event)
	return nil
}

func (g *graphActor) applyGraphCreatedEvent(event *model.GraphCreatedEvent) {

}

func (g *graphActor) applyNoop(event interface{}) {

}

// process events
func (g *graphActor) receiveRecover(context actor.Context) {
}

func (g *graphActor) validateStages(stageIDs []uint32) bool {
	stages := make([]graph.StageID, len(stageIDs))
	for i, id := range stageIDs {
		stages[i] = graph.StageID(id)
	}
	return g.graph.GetStages(stages) != nil
}

// if validation fails, this method will respond to the request with an appropriate error message
func (g *graphActor) validateCmd(cmd interface{}, context actor.Context) bool {
	if isGraphMessage(cmd) {
		graphId := getGraphId(cmd)
		if g.graph == nil {
			context.Respond(NewGraphNotFoundError(graphId))
			return false
		} else if g.graph.IsCompleted() {
			context.Respond(NewGraphCompletedError(graphId))
			return false
		}
	}

	switch msg := cmd.(type) {

	case *model.AddDelayStageRequest:

	case *model.AddChainedStageRequest:
		if g.validateStages(msg.Deps) {
			context.Respond(NewGraphCompletedError(msg.GraphId))
			return false
		}
	}

	return true
}

// process commands
func (g *graphActor) receiveStandard(context actor.Context) {
	switch msg := context.Message().(type) {

	case *model.CreateGraphRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Creating graph")
		event := &model.GraphCreatedEvent{GraphId: msg.GraphId, FunctionId: msg.FunctionId}
		err := g.persist(event)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyGraphCreatedEvent(event)
		context.Respond(&model.CreateGraphResponse{GraphId: msg.GraphId})

	case *model.AddChainedStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding chained stage")
		event := &model.StageAddedEvent{
			StageId:      g.graph.NextStageID(),
			Op:           msg.Operation,
			Closure:      msg.Closure,
			Dependencies: msg.Deps,
		}
		err := g.persist(event)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(event)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: event.StageId})

	case *model.AddCompletedValueStageRequest:
		if !g.validateCmd(msg, context) {
			return
		}
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding completed value stage")

		addedEvent := &model.StageAddedEvent{
			StageId: g.graph.NextStageID(),
			Op:      model.CompletionOperation_completedValue,
		}
		err := g.persist(addedEvent)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(addedEvent)

		completedEvent := &model.StageCompletedEvent{
			StageId: g.graph.NextStageID(),
			Result:  msg.Result,
		}
		err = g.persist(completedEvent)
		if err != nil {
			context.Respond(NewGraphEventPersistenceError(msg.GraphId))
			return
		}
		g.applyNoop(completedEvent)
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: addedEvent.StageId})

	case *model.AddDelayStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding delay stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: 1})

	case *model.AddExternalCompletionStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding external completion stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: 1})

	case *model.AddInvokeFunctionStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Adding invoke stage")
		context.Respond(&model.AddStageResponse{GraphId: msg.GraphId, StageId: 1})

	case *model.CompleteStageExternallyRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Completing stage externally")
		context.Respond(&model.CompleteStageExternallyResponse{GraphId: msg.GraphId, StageId: msg.StageId, Successful: true})

	case *model.CommitGraphRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Committing graph")
		context.Respond(&model.CommitGraphProcessed{GraphId: msg.GraphId})

	case *model.GetStageResultRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Retrieving stage result")
		datum := &model.Datum{
			Val: &model.Datum_Blob{
				Blob: &model.BlobDatum{ContentType: "text", DataString: []byte("foo")},
			},
		}
		result := &model.CompletionResult{Successful: true, Datum: datum}
		context.Respond(&model.GetStageResultResponse{GraphId: msg.GraphId, StageId: msg.StageId, Result: result})

	case *model.CompleteDelayStageRequest:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Completing delayed stage")

	case *model.FaasInvocationResponse:
		log.WithFields(logrus.Fields{"graph_id": msg.GraphId}).Debug("Received fn invocation response")

	default:
		log.Infof("snapshot internal state %v", reflect.TypeOf(msg))
	}
}

func (g *graphActor) Receive(context actor.Context) {
	if g.Recovering() {
		g.receiveRecover(context)
	} else {
		g.receiveStandard(context)
	}
}
