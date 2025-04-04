package main

import (
	db "Recruitment-GO/internal/db"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
)

var (
	key    = os.Getenv("AUTH_KEY")
	MaxAge = 86400 * 30 // 30 days
	IsProd = os.Getenv("PROD")
)

type Service struct {
	queries *db.Queries
}

func NewService(queries *db.Queries) *Service {
	return &Service{queries: queries}
}
func (app *App) authMiddleware(c *gin.Context) {
	session := sessions.Default(c)

	rawuserID := session.Get(sessionUserKey)
	userID, ok := rawuserID.(pgtype.UUID)
	if rawuserID == nil || !ok || !userID.Valid {
		fmt.Println("Auth Middleware: No valid user ID found in session.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	c.Set("userID", userID)
	c.Next()
}

func (app *App) authProviderHandler(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get(sessionUserKey) != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/profile")
		return
	}
	if session.Get(sessionTempGothUserKey) != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/auth/choose-role")
		return
	}
	cp := gothic.GetContextWithProvider(c.Request, "google")
	gothic.BeginAuthHandler(c.Writer, cp)
}

func (app *App) authCallbackHandler(c *gin.Context) {
	session := sessions.Default(c)
	if session.Get(sessionUserKey) != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/profile")
		return
	}
	cp := gothic.GetContextWithProvider(c.Request, "google")
	// Complete the authe
	gothUser, err := gothic.CompleteUserAuth(c.Writer, cp)
	if err != nil {
		fmt.Printf("Callback Error: Failed to complete auth: %v\n", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Authentication failed: " + err.Error()})
		c.Abort()
		return
	}

	fmt.Printf("Goth User Info received: %+v\n", gothUser)

	// Check if user in our database
	dbUser, err := app.db.GetUserByGoogleID(c.Request.Context(), gothUser.UserID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Printf("User with Google ID %s not found. Redirecting to role selection.\n", gothUser.UserID)

			// Store temporary Goth user info in session
			session.Set(sessionTempGothUserKey, gothUser)
			if saveErr := session.Save(); saveErr != nil {
				fmt.Printf("Callback Error: Failed to save temporary session: %v\n", saveErr)
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Failed to save session state."})
				c.Abort()
				return
			}
			c.Redirect(http.StatusTemporaryRedirect, "/auth/choose-role")
			return

		} else {
			fmt.Printf("Callback Error: Database error checking user: %v\n", err)
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Database error during login."})
			c.Abort()
			return
		}
	}

	// User Exists  Log them in
	fmt.Printf("User %s found in DB (ID: %s, Role: %s). Logging in.\n", dbUser.Email, dbUser.ID.String(), dbUser.Role)
	session.Set(sessionUserKey, dbUser.ID) // Store DB User ID
	session.Delete(sessionTempGothUserKey) // Clean up temporary data just in case
	if saveErr := session.Save(); saveErr != nil {
		fmt.Printf("Callback Error: Failed to save session for existing user: %v\n", saveErr)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Failed to save session after login."})
		c.Abort()
		return
	}

	// Redirect existing user to their profile
	c.Redirect(http.StatusTemporaryRedirect, "/profile")
}

func (app *App) chooseRoleGetHandler(c *gin.Context) {
	session := sessions.Default(c)
	tempGothUserRaw := session.Get(sessionTempGothUserKey)

	if tempGothUserRaw == nil {
		fmt.Println("Choose Role GET: No temporary Goth user found in session.")
		c.Redirect(http.StatusTemporaryRedirect, "/") // Redirect home if state is missing
		c.Abort()
		return
	}

	_, ok := tempGothUserRaw.(goth.User)
	if !ok {
		fmt.Println("Choose Role GET: Invalid temporary user data in session.")
		session.Delete(sessionTempGothUserKey) // Clear bad data
		session.Save()
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	//role selection form

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `
        <!DOCTYPE html>
        <html>
        <head><title>Choose Role</title></head>
        <body>
            <h1>Choose Your Role</h1>
            <p>Welcome! To complete registration, please select your role:</p>
            <form action="/auth/choose-role" method="post">
                <input type="radio" id="applicant" name="role" value="`+RoleApplicant+`" required>
                <label for="applicant">Applicant</label><br>
                <input type="radio" id="recruiter" name="role" value="`+RoleRecruiter+`">
                <label for="recruiter">Recruiter</label><br><br>
                <input type="submit" value="Submit Role">
            </form>
        </body>
        </html>
    `)
}
func (app *App) chooseRolePostHandler(c *gin.Context) {
	session := sessions.Default(c)
	tempGothUserRaw := session.Get(sessionTempGothUserKey)

	if tempGothUserRaw == nil {
		fmt.Println("Choose Role POST: No temporary Goth user found.")
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"message": "Session expired or invalid state. Please try logging in again."})
		c.Abort()
		return
	}

	tempGothUser, ok := tempGothUserRaw.(goth.User)
	if !ok {
		fmt.Println("Choose Role POST: Invalid temporary user data type.")
		session.Delete(sessionTempGothUserKey)
		session.Save()
		c.HTML(http.StatusBadRequest, "error.html", gin.H{"message": "Invalid session state. Please try logging in again."})
		c.Abort()
		return
	}

	chosenRole := c.PostForm("role")

	if chosenRole != RoleApplicant && chosenRole != RoleRecruiter {
		fmt.Printf("Choose Role POST: Invalid role submitted: %s\n", chosenRole)
		c.String(http.StatusBadRequest, "Invalid role selected.")
		c.Abort()
		return
	}

	//parameters for the database
	newUserParams := db.CreateUserParams{
		GoogleID: tempGothUser.UserID,
		Email:    tempGothUser.Email,
		Name:     tempGothUser.Name,
		Role:     chosenRole,
	}

	fmt.Printf("Creating user %s (Google ID: %s) with role %s\n", newUserParams.Email, newUserParams.GoogleID, newUserParams.Role)
	newUser, err := app.db.CreateUser(c.Request.Context(), newUserParams)
	if err != nil {
		fmt.Printf("Choose Role POST: Failed to create user in database: %v\n", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Failed to register user. It's possible the email or account is already registered."})
		c.Abort()
		return
	}

	fmt.Printf("User created successfully with DB ID: %s\n", newUser.ID.String())

	session.Delete(sessionTempGothUserKey)  // Clean up temporary data
	session.Set(sessionUserKey, newUser.ID) // Store DB User ID
	if saveErr := session.Save(); saveErr != nil {
		fmt.Printf("Choose Role POST: User created (ID: %s) but failed to save final session: %v\n", newUser.ID.String(), saveErr)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"message": "Account created, but failed to log you in automatically. Please try logging in again."})
		c.Abort()
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/profile")
}

func (app *App) logoutHandler(c *gin.Context) {
	cp := gothic.GetContextWithProvider(c.Request, "google")
	err := gothic.Logout(c.Writer, cp)
	if err != nil {
		fmt.Printf("Error during gothic.Logout (potential redirect attempted): %v\n", err)
	}

	session := sessions.Default(c)
	session.Delete(sessionUserKey)
	session.Delete(sessionTempGothUserKey)
	session.Options(sessions.Options{MaxAge: -1}) // Expire cookie

	if saveErr := session.Save(); saveErr != nil {
		fmt.Printf("Logout Error: Failed to save session after clearing keys: %v\n", saveErr)
	}

	if !c.Writer.Written() {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	} else {
		fmt.Println("Logout Warning: Headers already written, cannot redirect cleanly.")

	}
}
