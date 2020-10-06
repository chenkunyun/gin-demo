package controller

import (
	"errors"
	"gin-demo/pkg/springcloud"
	"github.com/gin-gonic/gin"
)

type EurekaController struct {
}

func (controller *EurekaController) Handle(r *gin.Engine) {
	eureka := springcloud.NewEureka("http://localhost:1111/eureka/", "gin-demo", 30, true, true)
	err := eureka.Start()
	if err != nil {
		panic("eureka start failed: " + err.Error())
	}
	eurekaGroup := r.Group("eureka")
	{
		eurekaGroup.GET("/apps", func(context *gin.Context) {
			responseJson(context, func() (data interface{}, err error) {
				return eureka.GetApplications()
			})
		})
		eurekaGroup.GET("/apps/:appId", func(context *gin.Context) {
			responseJson(context, func() (data interface{}, err error) {
				appId := context.Param("appId")
				application, exist := eureka.GetApplication(appId)
				if exist {
					return application, nil
				}

				return nil, errors.New(appId + " not found")
			})
		})
	}
}
