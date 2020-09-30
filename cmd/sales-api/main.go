package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/williamnoble/garagesale/cmd/sales-api/internal/handlers"
	"github.com/williamnoble/garagesale/internal/platform/conf"
	"github.com/williamnoble/garagesale/internal/platform/database"
)

func main() {

	var cfg struct {
		Web struct {
			Address         string        `conf:"default:localhost:8000"`
			ReadTimeout     time.Duration `conf:"default:5s"`
			WriteTimeout    time.Duration `conf:"default:5s"`
			ShutdownTimeout time.Duration `conf:"default:5s"`
		}
		DB struct {
			User       string `conf:"default:postgres"`
			Password   string `conf:"default:postgres,noprint"`
			Host       string `conf:"default:localhost"`
			Name       string `conf:"default:postgres"`
			DisableTLS bool   `conf:"default:false"`
		}
	}

	if err := conf.Parse(os.Args[1:], "SALES", &cfg); err != nil {
		if err == conf.ErrHelpWanted {
			usage, err := conf.Usage("SALES", &cfg)
			if err != nil {
				log.Fatalf("error : generating config usage : %v", err)
			}
			fmt.Println(usage)
			return
		}
		log.Fatalf("error: parsing config: %s", err)
	}

	log.Println("main: started")
	defer log.Println("main: completed")

	db, err := database.Open()
	if err != nil {
		log.Fatalf("eror: connecting to db: %s", err)
	}
	defer db.Close()

	productsHandler := handlers.Products{DB: db}

	api := http.Server{
		Addr:         "localhost:8080",
		Handler:      http.HandlerFunc(productsHandler.List),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("main: API listening on %s", api.Addr)
		serverErrors <- api.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("error: listening and serving %s", err)
	case <-shutdown:
		log.Println("main: Start shutdown")

		const timeout = 5 * time.Second
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		err := api.Shutdown(ctx)
		if err != nil {
			log.Printf("main: graceful shutdown did not complete in %v: %v", timeout, err)
			err = api.Close()
		}

		if err != nil {
			log.Fatalf("main: could not stop server gracefully: %v", err)
		}
	}

}
