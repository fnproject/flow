package query

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/fnproject/completer/actor"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("logger", "query")

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSSHandler returns a gin handler function for websocket queries
func WSSHandler(manager actor.GraphManager, w gin.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	defer conn.Close()
	if err != nil {
		log.Debugf("Failed to set websocket upgrade: %+v", err)
		return
	}

	log.Debugf("Subscribing %v to stream", conn.RemoteAddr())
	// TODO handle subscription messages to a specific graph
	sub := manager.SubscribeStream("ignored", func(event interface{}) {
		json, err := json.Marshal(event)
		if err != nil {
			log.Warnf("Failed to convert to JSON: %s", err)
			return
		}
		msg := fmt.Sprintf("%s %s", reflect.TypeOf(event).String(), json)
		conn.WriteMessage(websocket.TextMessage, []byte(msg))
	})

	for {
		msgType, _, err := conn.ReadMessage()
		if err != nil || msgType == websocket.CloseMessage {
			break
		}
	}
	log.Debugf("Unsubscribing %v from stream", conn.RemoteAddr())
	manager.UnsubscribeStream(sub)
}
