package controller

import (
	"gin-demo/pkg/database"
	"gin-demo/pkg/service"
	"github.com/gin-gonic/gin"
)

type UserController struct {
}

func (controller *UserController) Handle(r *gin.Engine) {
	userService := &service.UserService{Database: database.Database}
	user := r.Group("/user")
	{
		user.GET("/list", func(context *gin.Context) {
			responseJson(context, func() (data interface{}, err error) {
				type QueryLimit struct {
					MaxItems int `form:"count" binding:"required,max=20,min=1"`
				}
				var limit QueryLimit
				if err := context.ShouldBind(&limit); err != nil {
					return parameterValidationError(err)
				}

				return userService.UserList(limit.MaxItems)
			})
		})
	}
}
