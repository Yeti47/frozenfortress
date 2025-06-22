package main

import (
	"fmt"
	"html/template"
	"os"
	"os/signal"
	"syscall"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/views/account"
	documentsview "github.com/Yeti47/frozenfortress/frozenfortress/webui/views/documents"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/views/login"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/views/recovery"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/views/register"
	secretsview "github.com/Yeti47/frozenfortress/frozenfortress/webui/views/secrets"
	tagsview "github.com/Yeti47/frozenfortress/frozenfortress/webui/views/tags"
	"github.com/gin-gonic/gin"
)

const AppVersion = "1.0.0"

func main() {

	config := ccc.LoadConfigFromEnv()

	db, err := ccc.SetupDatabase(config)
	if err != nil {
		panic("Failed to setup database: " + err.Error())
	}

	svc := configureServices(config, db)

	// Start the backup worker
	svc.BackupWorker.Start()

	// Set up graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		svc.Logger.Info("Shutting down backup worker...")
		svc.BackupWorker.Stop()
		os.Exit(0)
	}()

	router := gin.Default()

	registerRoutes(router, svc)

	router.Run(fmt.Sprintf(":%d", config.WebUiPort))
}

// registerRoutes registers all the routes for the web UI.
func registerRoutes(router *gin.Engine, svc services) {

	// Create template functions for pagination and utility
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
	}

	// Load HTML templates with functions
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob("views/**/*.html"))
	router.SetHTMLTemplate(tmpl)

	// Serve static files
	router.Static("/img", "./img")

	// Register routes from modules
	secretsview.RegisterRoutes(router, svc.SignInManager, svc.SecretManager, svc.MekStore, svc.EncryptionService, svc.Logger)
	tagsview.RegisterRoutes(router, svc.SignInManager, svc.TagManager, svc.Logger)
	documentsview.RegisterRoutes(router, svc.SignInManager, svc.DocumentManager, svc.TagManager, svc.MekStore, svc.EncryptionService, svc.Logger)
	login.RegisterRoutes(router, svc.SignInManager)
	register.RegisterRoutes(router, svc.UserManager)
	recovery.RegisterRoutes(router, svc.SignInManager)
	account.RegisterRoutes(router, svc.UserManager, svc.SignInManager)
}
