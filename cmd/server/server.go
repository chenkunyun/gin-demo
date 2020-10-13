package main

import (
	"context"
	"fmt"
	"gin-demo/pkg/util/ginprom"
	"gin-demo/pkg/util/logger"
	v1 "gin-demo/web/api/v1"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	newLogger, err := logger.NewLogger(&logger.LogConfig{
		Level:      "Debug",
		Filename:   "server.log",
		MaxSize:    10,
		MaxAge:     3,
		MaxBackups: 3,
	})
	if err != nil {
		fmt.Println("failed to new logger, ", err)
		os.Exit(-1)
	}
	defer newLogger.Sync()

	logger.SetGlobalLogger(newLogger)

	//gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(logger.GinLogger(newLogger))
	r.Use(logger.GinRecovery(newLogger, true))

	pprof.Register(r, "debug/pprof")

	ginprom.Register(r, "/metrics")

	api := &v1.Api{}
	api.Register(r)

	listenAddress := ":8080"
	// r.Run()
	s := &http.Server{
		Addr:              listenAddress,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1 * 1024 * 1024, // 1MB
	}

	go func() {
		log.Println("listening on:", listenAddress)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 2)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
