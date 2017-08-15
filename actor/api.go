package actor

import (
	"time"

	"github.com/AsynkronIT/protoactor-go/actor"

	"github.com/fnproject/completer/model"
	"github.com/sirupsen/logrus"
)

type GraphManager interface {
	CreateGraph(*model.CreateGraphRequest, time.Duration) *actor.Future
	AddStage(interface{}, time.Duration) *actor.Future
	GetStageResult(*model.GetStageResultRequest, time.Duration) *actor.Future
	CompleteStageExternally(*model.CompleteStageExternallyRequest, time.Duration) *actor.Future
	Commit(*model.CommitGraphRequest, time.Duration) *actor.Future
}

type actorManager struct {
	log *logrus.Entry
	pid *actor.PID
}

func NewGraphManager() GraphManager {
	decider := func(reason interface{}) actor.Directive {
		log.Warnf("Graph actor child failed %v", reason)
		return actor.StopDirective
	}
	strategy := actor.NewOneForOneStrategy(10, 1000, decider)
	props := actor.FromInstance(&graphSupervisor{}).WithSupervisor(strategy)
	pid, _ := actor.SpawnNamed(props, "supervisor")
	return &actorManager{
		log: logrus.WithField("logger", "graphManager"),
		pid: pid,
	}
}

func (m *actorManager) CreateGraph(req *model.CreateGraphRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Creating graph")
	return m.forwardRequest(req, timeout)
}
func (m *actorManager) AddStage(req interface{}, timeout time.Duration) *actor.Future {
	m.log.Debug("Adding stage")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) GetStageResult(req *model.GetStageResultRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Getting stage result")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) CompleteStageExternally(req *model.CompleteStageExternallyRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Completing stage externally")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) Commit(req *model.CommitGraphRequest, timeout time.Duration) *actor.Future {
	m.log.WithFields(logrus.Fields{"graph_id": req.GraphId}).Debug("Committing graph")
	return m.forwardRequest(req, timeout)
}

func (m *actorManager) forwardRequest(req interface{}, timeout time.Duration) *actor.Future {
	return m.pid.RequestFuture(req, timeout)
}
