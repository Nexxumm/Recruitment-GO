package main

import (
	"context"
	"fmt"

	"Recruitment-GO/api/user/profile"
	db "Recruitment-GO/internal/db"
	"log"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("INFO: Could not load .env file: %v", err)
	}
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatalf("FATAL: Environment variable is required for session security.")
	}

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleClientID == "" || googleClientSecret == "" {
		log.Println("WARNING: GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET environment variables not set.")

	}
	callbackURL := os.Getenv("CALLBACK_URL")
	if callbackURL == "" {
		log.Println("WARNING: CALLBACK_URL environment variable not set.")
	}
	// Database Credentials
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" {
		log.Fatalf("FATAL: Database connection details are missing")

	}
	//  Database Connection
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, "disable")

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatalf("FATAL: Unable to create connection : %v\n", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("FATAL: Failed to ping database: %v", err)
	}
	log.Println("Database connection established successfully")

	dbQueries := db.New(pool) // Create SQLC  instance

	sessionStore := cookie.NewStore([]byte(sessionSecret))
	sessionStore.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 30, // 30 days
		HttpOnly: true,
		Secure:   false,
	})

	goth.UseProviders(
		google.New(googleClientID, googleClientSecret, callbackURL),
	)
	gothic.Store = sessionStore

	app := &App{
		db:           dbQueries,
		sessionStore: sessionStore, // Pass the store
	}

	router := gin.Default()

	router.Use(sessions.Sessions("mysession", app.sessionStore))

	profileService := profile.NewService(dbQueries)
	profileService.RegisterHandlers(router)

	router.GET("/", app.homeHandler)

	authRoutes := router.Group("/auth")
	{
		authRoutes.GET("/:provider", app.authProviderHandler)
		authRoutes.GET("/:provider/callback", app.authCallbackHandler)
		authRoutes.GET("/choose-role", app.chooseRoleGetHandler)
		authRoutes.POST("/choose-role", app.chooseRolePostHandler)
	}

	router.GET("/logout", app.logoutHandler)

	protectedRoutes := router.Group("/profile")
	protectedRoutes.Use(app.authMiddleware)
	{
		protectedRoutes.GET("", app.profileHandler)
	}

	authenticated := router.Group("/")
	authenticated.Use(app.authMiddleware)
	{
		authenticated.GET("/dashboard", app.dashboardRedirectHandler)
		authenticated.GET("/recruiter/dashboard", app.recruiterDashboardHandler)
		authenticated.GET("/applicant/dashboard", app.applicantDashboardHandler)

		applicantRoutes := authenticated.Group("/applicant")
		{
			applicantRoutes.GET("/skills", app.getManageSkillsHandler)
			applicantRoutes.POST("/skills", app.postManageSkillsHandler)
			applicantRoutes.GET("/resume", app.getResumeHandler)
			applicantRoutes.POST("/resume", app.postResumeHandler)
		}

		jobsGroup := authenticated.Group("/jobs")
		{
			jobsGroup.GET("/new", app.getJobPostingFormHandler)
			jobsGroup.POST("", app.createJobPostingHandler)

		}
	}

	//  Start Server
	serverAddr := ":8000"
	log.Printf("Server starting on %s\n", serverAddr)
	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("FATAL: Failed to start server: %v", err)
	}
}
