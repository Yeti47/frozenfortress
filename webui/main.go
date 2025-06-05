package main

import (
	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/webui/views/login"
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

	// Load HTML templates contained in the "views" directory
	router.LoadHTMLGlob("views/**/*.html")

	// Serve static files for images
	router.Static("/img", "./img")

	// Home page route - protected by authentication
	router.GET("/", AuthMiddleware(svc.SignInManager), func(c *gin.Context) {
		// Get current user for display
		user, err := svc.SignInManager.GetCurrentUser(c.Request)
		if err != nil {
			c.Redirect(302, "/login")
			return
		}

		c.HTML(200, "index.html", gin.H{
			"Title":    "Frozen Fortress - Home",
			"Username": user.UserName,
			"Version":  AppVersion,
		})
	})

	login.RegisterRoutes(router, svc.SignInManager)
}
