package main

import (
	db "Recruitment-GO/internal/db"
	"database/sql"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxUploadSize = 5 * 1024 * 1024 // 5 MB

func (app *App) getResumeHandler(c *gin.Context) {
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
		c.String(http.StatusForbidden, "<html><body>Forbidden: Only applicants can manage resumes</body></html>")
		c.Abort()
		return
	}

	resumeBytes, err := app.db.GetUserResume(c.Request.Context(), pgID)
	resumeStatus := ""
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Get Resume Handler: DB error fetching resume status for %s: %v\n", pgID.String(), err)
		resumeStatus = `<p style="color:red;">Error checking current resume status.</p>`
	} else if err == sql.ErrNoRows {
		fmt.Printf("Get Resume Handler: User %s not found when checking resume status.\n", pgID.String())
		resumeStatus = `<p style="color:red;">Error checking user status.</p>`
	} else if len(resumeBytes) > 0 {
		resumeStatus = `<p>A resume is currently uploaded. Uploading a new file will replace it.</p>`
	} else {
		resumeStatus = `<p>No resume currently uploaded.</p>`
	}

	formHTML := fmt.Sprintf(`
		<h2>Manage Resume</h2>
		%s 
		<form method="POST" action="/applicant/resume" enctype="multipart/form-data">
			<div>
				<label for="resumeFile">Upload New Resume (PDF only):</label><br><br>
				<input type="file" id="resumeFile" name="resumeFile" accept=".pdf" required>
			</div>
			<br>
			<button type="submit">Upload Resume</button>
		</form>
		<br>
		<p><a href="/applicant/dashboard">Back to Dashboard</a></p>
	`, resumeStatus)

	fullHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Manage Resume</title></head><body>
		<nav>...</nav><hr>
		%s
		<hr><footer>...</footer></body></html>`, formHTML)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) postResumeHandler(c *gin.Context) {
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
		c.String(http.StatusForbidden, "<html><body>Forbidden: Only applicants can manage resumes</body></html>")
		c.Abort()
		return
	}

	fileHeader, err := c.FormFile("resumeFile")
	if err != nil {
		fmt.Printf("Post Resume Handler: Error getting form file: %v\n", err)
		c.String(http.StatusBadRequest, "<html><body>Error: No resume file uploaded or invalid field name.</body></html>")
		return
	}

	if fileHeader.Header.Get("Content-Type") != "application/pdf" {
		fmt.Printf("Post Resume Handler: Invalid file type: %s\n", fileHeader.Header.Get("Content-Type"))
		c.String(http.StatusBadRequest, "<html><body>Error: Invalid file type. Only PDF is allowed.</body></html>")
		return
	}

	if fileHeader.Size > maxUploadSize {
		fmt.Printf("Post Resume Handler: File too large: %d bytes\n", fileHeader.Size)
		c.String(http.StatusBadRequest, "<html><body>Error: File size exceeds limit (5MB).</body></html>")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		fmt.Printf("Post Resume Handler: Error opening uploaded file: %v\n", err)
		c.String(http.StatusInternalServerError, "<html><body>Error processing uploaded file.</body></html>")
		return
	}
	defer file.Close()

	resumeBytes, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("Post Resume Handler: Error reading uploaded file: %v\n", err)
		c.String(http.StatusInternalServerError, "<html><body>Error reading uploaded file content.</body></html>")
		return
	}

	params := db.UpdateUserResumeParams{
		ID:        pgID,
		ResumePdf: resumeBytes,
	}

	err = app.db.UpdateUserResume(c.Request.Context(), params)
	if err != nil {
		fmt.Printf("Post Resume Handler: DB error updating resume for user %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "<html><body>Error saving resume to database. Please try again.</body></html>")
		return
	}

	fmt.Printf("Successfully updated resume for user %s\n", pgID.String())

	c.Redirect(http.StatusSeeOther, "/applicant/dashboard")
}
