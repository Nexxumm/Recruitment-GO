package main

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *App) homeHandler(c *gin.Context) {
	session := sessions.Default(c)
	userIDRaw := session.Get(sessionUserKey)
	loggedIn := userIDRaw != nil

	c.Header("Content-Type", "text/html; charset=utf-8")
	if loggedIn {
		c.String(http.StatusOK, `<h1>Welcome Back!</h1><p><a href="/profile">Profile</a></p><p><a href="/logout">Logout</a></p>`)
	} else {
		c.String(http.StatusOK, `<h1>Welcome!</h1><p><a href="/auth/google">Login with Google</a></p>`)
	}

}
func (app *App) profileHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	dbUserID := userID.(pgtype.UUID)
	if !dbUserID.Valid {
		c.String(http.StatusBadRequest, "Invalid user ID")
		c.Abort()
		return
	}

	// user details from DB
	user, err := app.db.GetUser(c.Request.Context(), dbUserID)
	if err != nil {
		fmt.Printf("Profile Handler: Failed to get user %s from DB: %v\n", dbUserID.String(), err)
		// Clear session if user not found in DB
		session := sessions.Default(c)
		session.Delete(sessionUserKey)
		session.Save()
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	// Display user information
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `
        <h1>Profile</h1>
        <p>Welcome, %s!</p>
        <p>Email: %s</p>
        <p>Your Role: <strong>%s</strong></p>
        <p>Google ID: %s</p>
        <p><a href="/">Home</a></p>
        <p><a href="/logout">Logout</a></p>
		<p><a href="/dashboard">Dashboard</a></p>
    `, user.Name, user.Email, user.Role, user.GoogleID)
}

func (app *App) dashboardRedirectHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		fmt.Printf("Dashboard Redirect: Failed to get user %s: %v\n", pgID.String(), err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	// Check user role and redirect accordingly
	if user.Role == RoleRecruiter {
		c.Redirect(http.StatusTemporaryRedirect, "/recruiter/dashboard")
		c.Abort()
	} else if user.Role == RoleApplicant {
		c.Redirect(http.StatusTemporaryRedirect, "/applicant/dashboard")
		c.Abort()
	} else {
		// Unknown role or user with no role? Redirect home.
		fmt.Printf("Dashboard Redirect: User %s has unknown role: %s\n", pgID.String(), user.Role)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}
}

func (app *App) recruiterDashboardHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		fmt.Println("Recruiter Dashboard: No user ID found in context.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		fmt.Println("Recruiter Dashboard: Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}

	if user.Role != RoleRecruiter {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Access denied</body></html>")
		c.Abort()
	}

	userName := user.Name

	jobsHtml := "<p><i>(Job postings list will appear here)</i></p>"

	dashboardHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Recruiter Dashboard</title></head><body>
		<h1>Recruiter Dashboard</h1>
		<p>Welcome, %s!</p>
		<hr>
		<h2>My Job Postings</h2>
		%s 
		<p><a href="/jobs/new">Create New Job Posting</a></p> 
        <hr>
        <h2>Other Actions</h2>
		<p><a href="/recruiter/search">Search Applicants By Skill</a></p>
        <p><a href="/logout">Logout</a></p>
		</body></html>`,
		userName, jobsHtml)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}

func (app *App) applicantDashboardHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		fmt.Println("Applicant Dashboard: No user ID found in context.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		fmt.Println("Applicant Dashboard: Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		fmt.Printf("Applicant Dashboard: Failed to get user %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
	}

	if user.Role != RoleApplicant {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Access denied</body></html>")
		c.Abort()
		return
	}

	userName := user.Name

	applicationsHtml := "<p><i>(Your applications will appear here)</i></p>"
	skillsHtml := "<p><i>(Your skills will appear here)</i></p>"

	dashboardHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Applicant Dashboard</title></head><body>
		<h1>Applicant Dashboard</h1>
		<p>Welcome, %s!</p>
		<hr>
		<h2>My Applications</h2>
        %s
		<p><a href="/jobs">Browse Open Jobs</a></p> 
        <hr>
        <h2>My Profile</h2>
		<p><a href="/applicant/resume">Manage Resume</a></p>
        <p><a href="/applicant/skills">Manage Skills</a></p>
        <h3>Current Skills:</h3>
        %s
        <hr>
        <p><a href="/logout">Logout</a></p>
		</body></html>`,
		userName, applicationsHtml, skillsHtml)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}
