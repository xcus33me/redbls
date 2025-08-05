package main

import (
	"log"
	ttlcache "redbls/internal/database"
	v1 "redbls/internal/handlers/v1"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func main() {
	app := fiber.New()

	cache := ttlcache.New(time.Second)

	l, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err.Error())
		return
	}
	s := l.Sugar()

	v1.NewRouter(app, cache, s)

	s.Fatal(app.Listen(":8080"))
}
