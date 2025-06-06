package main

import (
	"fmt"
	"html/template"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/views/login"
	secretsview "github.com/Yeti47/frozenfortress/frozenfortress/webui/views/secrets"
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

	// Serve static files for images
	router.Static("/img", "./img")

	// Register routes from modules
	secretsview.RegisterRoutes(router, svc.SignInManager, svc.SecretManager, svc.MekStore, svc.EncryptionService, svc.Logger)
	login.RegisterRoutes(router, svc.SignInManager)
}
