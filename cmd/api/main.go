package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/ghduuep/pingly/docs"
	"github.com/ghduuep/pingly/internal/api"
	"github.com/ghduuep/pingly/internal/database"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// @title Pingly API
// @version 1.0
// @description API for Pingly monitoring service.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@pingly.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Cannot load .env file.")
	}

	db := database.InitDB()

	rdb := database.InitRedis()

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(rate.Limit(20))))
	e.Use(middleware.CORS())

	e.Validator = &api.CustomValidator{Validator: validator.New()}

	api.SetupRotes(e, db, rdb)

	port := ":8080"

	go func() {
		log.Printf("API server is running on port %s", port)
		if err := e.Start(port); err != nil {
			e.Logger.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}

	db.Close()

	if err := rdb.Close(); err != nil {
		e.Logger.Fatal(err)
	}

	log.Println("Server gracefully stopped.")
}
