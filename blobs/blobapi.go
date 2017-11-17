package blobs

import (
	"github.com/gin-gonic/gin"
)

const headerContentType = "Content-Type"

type BlobServer struct {
	store  Store
	Engine *gin.Engine
}

func NewFromEngine(store Store, engine *gin.Engine) *BlobServer {
	server := &BlobServer{
		store:  store,
		Engine: engine,
	}
	createBlobApi(server)
	return server
}

func (s *BlobServer) createBlob(c *gin.Context) {

	contentType := c.GetHeader(headerContentType)

	if contentType == "" {

	}
	prefix := c.Param("prefix")

	s.store.Create(prefix, contentType, c.Request.Body)

}

func (s *BlobServer) getBlob(c *gin.Context) {

}

func (s *BlobServer) headBlob(c *gin.Context) {

}

func createBlobApi(s *BlobServer) {

	blobs := s.Engine.Group("/blobs")
	{
		blobs.POST("/:prefix", s.createBlob)
		blobs.GET("/:prefix/:blobId", s.getBlob)
		blobs.HEAD("/:prefix/:blobId", s.headBlob)
	}
}
