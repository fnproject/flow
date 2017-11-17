package blobs

import (
	"github.com/gin-gonic/gin"
)

const headerContentType = "Content-Type"

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

	if contentType == "" {

	}
	prefix := c.Param("prefix")

	s.store.Create(prefix, contentType, c.Request.Body)

}

func (s *Server) getBlob(c *gin.Context) {

}

func (s *Server) headBlob(c *gin.Context) {

}

func createBlobAPI(s *Server) {

	blobs := s.Engine.Group("/blobs")
	{
		blobs.POST("/:prefix", s.createBlob)
		blobs.GET("/:prefix/:blobId", s.getBlob)
		blobs.HEAD("/:prefix/:blobId", s.headBlob)
	}
}
