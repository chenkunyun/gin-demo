package controller

import "github.com/gin-gonic/gin"

type controller interface {
	Handle(r *gin.Engine)
}
