package main

import (
	"log"
	"os"
	"os/signal"
	ttlcache "redbls/internal/database"
	v1 "redbls/internal/handlers/v1"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func main() {
	app := fiber.New()

	cache := ttlcache.New(time.Second)

	err := ttlcache.InitPostgresTransactionLogger(cache)
	if err != nil {
		log.Fatal(err.Error())
	}

	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err.Error())
	}
	s := l.Sugar()

	v1.NewRouter(app, cache, s)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		s.Info("Shutting down gracefully...")
		cache.Close()
		os.Exit(0)
	}()

	s.Fatal(app.Listen(":8080"))
}
