package server

import "github.com/gin-gonic/gin"



func (s *Server) createBlob(c *gin.Context) {

}

func (s *Server) getBlob(c *gin.Context) {

}

func (s *Server) headBlob(c *gin.Context) {

}




func createBlobApi(s *Server) {

	blobs := s.Engine.Group("/blobs")
	{
		blobs.POST("/:graphId", s.createBlob)
		blobs.GET("/:graphId/:blobId", s.getBlob)
		blobs.HEAD("/:graphId/:blobId", s.headBlob)

	}
}
