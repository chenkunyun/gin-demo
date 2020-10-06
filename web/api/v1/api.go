package v1

import (
	"gin-demo/pkg/controller"
	"github.com/gin-gonic/gin"
)

type Api struct {
}

func (api *Api) Register(r *gin.Engine) {
	userController := &controller.UserController{}
	userController.Handle(r)

	eurekaController := &controller.EurekaController{}
	eurekaController.Handle(r)

	gatewayController, err := controller.NewGatewayController("http://localhost:1111/eureka/", "gin-demo")
	if err != nil {
		panic(err)
	}
	gatewayController.Handle(r)
}
