package blobs

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const headerContentType = "Content-Type"

var log = logrus.WithField("logger", "blob_store")

// Server encapsulates the blob server
type Server struct {
	store  Store
	Engine *gin.Engine
}

// NewFromEngine creates a new blob service from an existing gin engine
func NewFromEngine(store Store, engine *gin.Engine) *Server {
	server := &Server{
		store:  store,
		Engine: engine,
	}
	createBlobAPI(server)
	return server

}

func (s *Server) createBlob(c *gin.Context) {

	contentType := c.GetHeader(headerContentType)
	if len(contentType) == 0 {
		c.AbortWithError(400, fmt.Errorf("Missing %s header", headerContentType))
		return
	}

	prefix := c.Param("prefix")
	blob, err := s.store.Create(prefix, contentType, c.Request.Body)

	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	c.JSON(200, blob)

}

func (s *Server) getBlob(c *gin.Context) {

	prefix := c.Param("prefix")
	blobID := c.Param("blobId")

	r, err := s.store.Read(prefix, blobID)
	if err != nil {
		if err == ErrBlobNotFound {
			c.AbortWithError(404, err)
			return
		}
		log.WithError(err).Error("Error querying blobstore")
		c.AbortWithError(500, err)
		return
	}

	// avoid internal buffering
	if _, err = ioutil.ReadAll(io.TeeReader(r, c.Writer)); err != nil {
		log.WithError(err).Error("Error writing blobstore response")
		c.AbortWithError(500, err)
		return
	}

}

func createBlobAPI(s *Server) {

	blobs := s.Engine.Group("/blobs")
	{
		blobs.POST("/:prefix", s.createBlob)
		blobs.GET("/:prefix/:blobId", s.getBlob)
	}

}
