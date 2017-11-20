package server

import (
	"github.com/golang/protobuf/jsonpb"
	"net/http"
	"github.com/gogo/protobuf/proto"
	"fmt"
)

type JSONPBBinding struct {
}

func (b *JSONPBBinding) Name() string {
	return "jsonpb"
}

func (b *JSONPBBinding) Bind(r *http.Request, msg interface{}) error {
	pmsg, ok := msg.(proto.Message)

	if !ok {
		return fmt.Errorf("invalid message, not a protobuf ")
	}

	err := jsonpb.Unmarshal(r.Body, pmsg)
	if err != nil {
		return err
	}

	return nil

}

type JSONPBRender struct {
	Msg proto.Message
}

func (b *JSONPBRender) Render(rr http.ResponseWriter) error {
	m := &jsonpb.Marshaler{}

	return m.Marshal(rr, b.Msg)
}
func (b *JSONPBRender) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{"application/json"}
	}
}
