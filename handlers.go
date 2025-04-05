package main

import (
	db "Recruitment-GO/internal/db"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	// Check user role and redirect
	if user.Role == RoleRecruiter {
		c.Redirect(http.StatusTemporaryRedirect, "/recruiter/dashboard")
		c.Abort()
	} else if user.Role == RoleApplicant {
		c.Redirect(http.StatusTemporaryRedirect, "/applicant/dashboard")
		c.Abort()
	} else {
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
		return
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		fmt.Println("Recruiter Dashboard: Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	if user.Role != RoleRecruiter {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Access denied</body></html>")
		c.Abort()
		return
	}

	userName := user.Name

	postings, err := app.db.ListJobPostingsByRecruiter(c.Request.Context(), pgID)

	var jobsHtmlBuilder strings.Builder
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Recruiter Dashboard: Failed to list job postings for %s: %v\n", pgID.String(), err)
		jobsHtmlBuilder.WriteString("<p style='color:red;'>Error loading job postings.</p>")
	} else if len(postings) == 0 {
		jobsHtmlBuilder.WriteString("<p>You have not posted any jobs yet.</p>")
	} else {
		jobsHtmlBuilder.WriteString("<ul>")
		for _, posting := range postings {
			// Assuming each posting has a Title and Status field

			jobsHtmlBuilder.WriteString(fmt.Sprintf(
				`<li>%s (Status: %s) </li>`,
				posting.Title,
				posting.Status,
			))
		}
		jobsHtmlBuilder.WriteString("</ul>")
	}

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
		userName, jobsHtmlBuilder.String())

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)
}

func (app *App) applicantDashboardHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		fmt.Println("Applicant Dashboard: No user ID found in context.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		fmt.Println("Applicant Dashboard: Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		fmt.Printf("Applicant Dashboard: Failed to get user %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	if user.Role != RoleApplicant {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Access denied</body></html>")
		c.Abort()
		return
	}

	userName := user.Name

	applicationsHtml := "<p><i>(Your applications will appear here)</i></p>"
	skillNames, err := app.db.GetUserSkillNames(c.Request.Context(), pgID)
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Applicant Dashboard: Failed to get user skills for %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "Failed to get user skills from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	var skillsHtml strings.Builder
	if err != nil && err != sql.ErrNoRows {
		skillsHtml.WriteString("<p style='color:red;'>Error loading skills.</p>")
	} else if len(skillNames) == 0 {
		skillsHtml.WriteString("<p>You haven't added any skills yet.</p>")
	} else {
		skillsHtml.WriteString("<ul>")
		for _, name := range skillNames {
			skillsHtml.WriteString(fmt.Sprintf("<li>%s</li>", name))
		}
		skillsHtml.WriteString("</ul>")
	}

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
		userName, applicationsHtml, skillsHtml.String())

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, dashboardHTML)

}

func (app *App) getManageSkillsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		fmt.Println("Manage Skills GET: No user ID found in context.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		fmt.Println("Manage Skills GET: Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		fmt.Printf("Manage Skills GET: Failed to get user %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	if user.Role != RoleApplicant {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	allSkills, err := app.db.ListSkills(c.Request.Context())
	if err != nil {
		fmt.Printf("Manage Skills GET: Failed to list skills: %v\n", err)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, "<html><body>Error loading skills list</body></html>")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		return
	}

	currentUserSkills, err := app.db.GetUserSkillIDs(c.Request.Context(), pgID)
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Manage Skills GET: Failed to get user skills: %v\n", err)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, "<html><body>Error loading your skills</body></html>")
		return
	}
	if err == sql.ErrNoRows {
		currentUserSkills = []pgtype.UUID{}
	}
	currentUserSkillsMap := make(map[uuid.UUID]bool)
	for _, pgSkillID := range currentUserSkills {
		if pgSkillID.Valid {
			currentUserSkillsMap[uuid.UUID(pgSkillID.Bytes)] = true
		}
	}

	var skillsChecklistHTML strings.Builder
	skillsChecklistHTML.WriteString(`<form method="POST" action="/applicant/skills">`)
	skillsChecklistHTML.WriteString("<h3>Select your skills:</h3>")

	if len(allSkills) == 0 {
		skillsChecklistHTML.WriteString("<p>No skills available to select.</p>")
	} else {
		for _, skill := range allSkills {
			var skillUUID uuid.UUID
			var skillIDStr string
			if skill.ID.Valid {
				skillUUID = uuid.UUID(skill.ID.Bytes)
				skillIDStr = skillUUID.String()
			} else {
				continue
			}

			isChecked := currentUserSkillsMap[skillUUID]
			checkedAttr := ""
			if isChecked {
				checkedAttr = " checked"
			}

			skillsChecklistHTML.WriteString(fmt.Sprintf(
				`<div><input type="checkbox" id="skill_%s" name="skill_ids" value="%s"%s> <label for="skill_%s">%s</label></div>`,
				skillIDStr, skillIDStr, checkedAttr, skillIDStr, skill.Name,
			))
		}
		skillsChecklistHTML.WriteString(`<br><button type="submit">Update Skills</button>`)
	}
	skillsChecklistHTML.WriteString(`</form>`)

	fullHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Manage Skills</title></head><body>
		<nav>...</nav> <hr> 
		<h1>Manage Your Skills</h1>
		%s
		<hr>
		<p><a href="/applicant/dashboard">Back to Dashboard</a></p>
		</body></html>`,
		skillsChecklistHTML.String())

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) postManageSkillsHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		fmt.Println("Manage Skills POST: No user ID found in context.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	pgID, ok := userID.(pgtype.UUID)
	if !ok || !pgID.Valid {
		fmt.Println("Manage Skills POST: Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	user, err := app.db.GetUser(c.Request.Context(), pgID)
	if err != nil {
		fmt.Printf("Manage Skills POST: Failed to get user %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	if user.Role != RoleApplicant {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	submittedSkillIDStrings := c.PostFormArray("skill_ids")

	var selectedSkillPgUUIDs = []pgtype.UUID{}
	for _, idStr := range submittedSkillIDStrings {
		parsedUUID, err := uuid.Parse(idStr)
		if err != nil {
			fmt.Printf("Manage Skills POST: Received invalid UUID string: %s\n", idStr)
			continue
		}
		selectedSkillPgUUIDs = append(selectedSkillPgUUIDs, pgtype.UUID{Bytes: parsedUUID, Valid: true})
	}

	err = app.db.DeleteUserSkills(c.Request.Context(), pgID)
	if err != nil {
		fmt.Printf("Manage Skills POST: Failed to delete old skills for user %s: %v\n", pgID.String(), err)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, "<html><body>Error updating skills (step 1)</body></html>")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	for _, skillPgID := range selectedSkillPgUUIDs {
		if !skillPgID.Valid {
			continue
		}
		addParams := db.AddSkillToUserParams{
			UserID:  pgID,
			SkillID: skillPgID,
		}
		err = app.db.AddSkillToUser(c.Request.Context(), addParams)
		if err != nil {
			fmt.Printf("Manage Skills POST: Failed to add skill %s for user %s: %v\n", skillPgID.String(), pgID.String(), err)
			c.Redirect(http.StatusTemporaryRedirect, "/")
			c.Abort()
			return
		}
	}

	fmt.Printf("Successfully updated skills for user %s\n", pgID.String())

	c.Redirect(http.StatusSeeOther, "/applicant/dashboard")
}
