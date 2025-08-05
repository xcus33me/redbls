package v1

import (
	ttlcache "redbls/internal/database"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"go.uber.org/zap"
)

func NewRouter(app *fiber.App, c *ttlcache.Cache, l *zap.SugaredLogger)  {
	app.Use(logger.New())

	api := V1 {
		c: c,
		l: l,
	}

	appV1Group := app.Group("/v1")
	{
		appV1Group.Get("/keys", api.CacheGetAllKeysHandler)
		appV1Group.Get("/key/:key", api.CacheGetHandler)
		appV1Group.Put("/key/:key", api.CacheAddHandler)
		appV1Group.Delete("/key/:key", api.CacheDeleteHandler)
	}
}
