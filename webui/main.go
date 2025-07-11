package main

import (
	"encoding/base64"
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
		"mul": func(a, b int) int {
			return a * b
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
		"base64": func(data []byte) string {
			return base64.StdEncoding.EncodeToString(data)
		},
		"dict": func(values ...any) map[string]any {
			if len(values)%2 != 0 {
				panic("dict requires an even number of arguments")
			}
			dict := make(map[string]any)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					panic("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict
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

	// Create document services aggregate
	docServices := documentsview.DocumentServices{
		DocumentManager:     svc.DocumentManager,
		DocumentFileManager: svc.DocumentFileManager,
		DocumentListService: svc.DocumentListService,
		TagManager:          svc.TagManager,
		NoteManager:         svc.NoteManager,
	}
	documentsview.RegisterRoutes(router, svc.SignInManager, docServices, svc.MekStore, svc.EncryptionService, svc.Logger)

	login.RegisterRoutes(router, svc.SignInManager)
	register.RegisterRoutes(router, svc.UserManager)
	recovery.RegisterRoutes(router, svc.SignInManager)
	account.RegisterRoutes(router, svc.UserManager, svc.SignInManager)
}
