package controller

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net"
	"net/http"
)

type apiResponse struct {
	Code int64       `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type serviceHandler func() (data interface{}, err error)

func responseJson(c *gin.Context, handlerFunc serviceHandler) {
	data, err := handlerFunc()
	serviceData := wrapServiceResult(data, err)
	c.JSON(http.StatusOK, serviceData)
}

func parameterValidationError(err error) (interface{}, error) {
	return nil, err
}

func wrapServiceResult(data interface{}, err error) apiResponse {
	if err == nil {
		return apiResponse{0, "success", data}
	}

	if _, ok := err.(validator.ValidationErrors); ok {
		return apiResponse{-1, "parameter error", nil}
	}

	var be *businessError
	if errors.As(err, &be) {
		return apiResponse{be.code, be.msg, nil}
	}

	var opError *net.OpError
	if errors.As(err, &opError) {
		return apiResponse{-1, opError.Error(), nil}
	}

	return apiResponse{-1, err.Error(), nil}
}

type businessError struct {
	code int64
	msg  string
}

func (error *businessError) Error() string {
	return fmt.Sprintf("code:%d, msg:%s", error.code, error.msg)
}
