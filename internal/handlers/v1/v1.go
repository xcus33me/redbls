package v1

import (
	"net/http"
	ttlcache "redbls/internal/database"
	"time"

	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type V1 struct {
	c *ttlcache.Cache
	l *zap.SugaredLogger
}

func (api *V1) CacheGetHandler(ctx fiber.Ctx) error {
	keyParam := ctx.Params("key")
	if keyParam == "" {
		return ctx.Status(http.StatusBadRequest).SendString("key parameter is missing")
	}

	value, found := api.c.Get(keyParam)
	if !found {
		return ctx.Status(http.StatusNotFound).SendString("key not found")
	}

	return ctx.Status(http.StatusOK).SendString(value)
}

func (api *V1) CacheAddHandler(ctx fiber.Ctx) error {
	keyParam := ctx.Params("key")
	if keyParam == "" {
		return ctx.Status(http.StatusBadRequest).SendString("key parameter is missing")
	}

	value := ctx.Body()
	api.l.Infof("Cache - Add: keyParam=%s, ValueFromBody=%s", keyParam, string(value))

	api.c.Add(keyParam, string(value), 5 * time.Minute)

	return ctx.Status(http.StatusOK).SendString("OK")
}

func (api *V1) CacheDeleteHandler(ctx fiber.Ctx) error {
	keyParam := ctx.Params("key")
	if keyParam == "" {
		return ctx.Status(http.StatusBadRequest).SendString("key parameter is missing")
	}

	api.c.Delete(keyParam)

	return ctx.Status(http.StatusOK).SendString("OK")
}

func (api *V1) CacheGetAllKeysHandler(ctx fiber.Ctx) error {
	result := api.c.Dump()

	return ctx.Status(http.StatusOK).SendString(result)
}
