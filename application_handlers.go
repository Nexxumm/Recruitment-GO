package main

import (
	"Recruitment-GO/internal/db"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/smtp"
	"os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *App) getApplyFormHandler(c *gin.Context) {
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
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Only applicants can apply for jobs</body></html>")
		c.Abort()
		return
	}

	jobIDStr := c.Param("jobID")
	jobUUID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>Invalid Job ID format</body></html>")
		return
	}
	jobPgID := pgtype.UUID{Bytes: jobUUID, Valid: true}

	job, err := app.db.GetJobPostingByID(c.Request.Context(), jobPgID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusNotFound, "<html><body>Job posting not found</body></html>")
		} else {
			fmt.Printf("Apply GET: DB error fetching job %s: %v\n", jobIDStr, err)
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusInternalServerError, "<html><body>Error fetching job data</body></html>")
			c.Abort()
		}

	}

	if job.Status != "active" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>This job posting is no longer active.</body></html>")
		return
	}

	_, checkErr := app.db.CheckApplicationExists(c.Request.Context(), db.CheckApplicationExistsParams{
		UserID:       pgID,
		JobPostingID: jobPgID,
	})

	if checkErr == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body><h1>Already applied for this job posting</h1></body></html>"+
			"<p><a href=\"/applicant/dashboard\">Back to Dashboard</a></p>")
		return
	} else if !errors.Is(checkErr, sql.ErrNoRows) {
		fmt.Printf("Apply GET: DB error checking existing application for user %s, job %s: %v\n", pgID.String(), jobPgID.String(), checkErr)
		return
	}
	salaryMinVal, err := job.SalaryMin.Value()
	if err != nil {
		salaryMinVal = ""
	}
	salaryMaxVal, err := job.SalaryMax.Value()
	if err != nil {
		salaryMaxVal = ""
	}
	applyPageHTML := fmt.Sprintf(`
		<h2>Apply for Job</h2>
		<h3>%s</h3>
		<p><strong>Recruiter:</strong> %s</p>
		<p><strong>Salary Range:</strong> %v - %v</p>
		<p><strong>Status:</strong> %s</p>
		<hr>
		<p>Click below to submit your application using your stored resume (if available).</p>
		
		<form method="POST" action="/jobs/%s/apply">
			<button type="submit">Confirm Application</button>
		</form>
		<br>
		<p><a href="/jobs">Back to Job List</a></p>
        <p><a href="/applicant/dashboard">Back to Dashboard</a></p>
		`,
		job.Title,
		job.RecruiterName,
		salaryMinVal,
		salaryMaxVal,
		job.Status,
		jobIDStr,
	)

	fullHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Apply: %s</title></head><body>
		<nav>...</nav><hr>
		%s
		<hr><footer>...</footer></body></html>`,
		html.EscapeString(job.Title),
		applyPageHTML,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) postApplyHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	session := sessions.Default(c)

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
	userParsedResume, err := app.db.GetParsedResume(c.Request.Context(), pgID)
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>You must upload a resume before applying for jobs.</body></html>")
		return
	}
	if userParsedResume == nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>You must upload a resume before applying for jobs.OR error with resume Please upload again</body></html>")
		return
	}

	jobIDStr := c.Param("jobID")
	jobUUID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>Invalid Job ID format</body></html>")
		return
	}
	jobPgID := pgtype.UUID{Bytes: jobUUID, Valid: true}

	_, err = app.db.GetJobPostingByID(c.Request.Context(), jobPgID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.String(http.StatusNotFound, "Job not found.")
		} else {
			c.String(http.StatusInternalServerError, "Error verifying job.")
		}
		return
	}

	params := db.CreateApplicationParams{
		UserID:       pgID,
		JobPostingID: jobPgID,
	}

	_, err = app.db.CreateApplication(c.Request.Context(), params)
	if err != nil {
		pgErr := err.(*pgconn.PgError)
		if pgErr.Code == "23505" {
			fmt.Printf("Apply POST: User %s already applied for job %s (unique violation detected via errors.As)\n", pgID.String(), jobPgID.String())
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusConflict, "<html><body>You have already applied for this job. <a href='/applicant/dashboard'>View Applications</a></body></html>")
			return

		} else {
			fmt.Printf("Apply POST: Error creating application for user %s to job %s: %v\n", pgID.String(), jobPgID.String(), err)
			c.Header("Content-Type", "text/html; charset=utf-8")
		}

		fmt.Printf("Successfully created application for user %s to job %s\n", user.Name, jobPgID.String())

		if saveErr := session.Save(); saveErr != nil {
			fmt.Printf("Apply POST: Error saving session before redirect: %v\n", saveErr)
		} else {
			fmt.Println("Apply POST: Session saved explicitly before redirect.")
		}

		c.Redirect(http.StatusTemporaryRedirect, "/applicant/dashboard")
	}
}

func (app *App) requestInterviewHandler(c *gin.Context) {
	recruiterUserID, exists := c.Get("userID")
	if !exists {
		fmt.Println("Request Interview : No user ID found in context.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	recruiterPgID, ok := recruiterUserID.(pgtype.UUID)
	if !ok || !recruiterPgID.Valid {
		fmt.Println("Request Interview : Invalid user ID format.")
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	recruiterUser, err := app.db.GetUser(c.Request.Context(), recruiterPgID)
	if err != nil {
		fmt.Printf("Request Interview : Failed to get user %s: %v\n", recruiterPgID.String(), err)
		c.String(http.StatusInternalServerError, "Failed to get user from DB: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	if recruiterUser.Role != RoleRecruiter {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Access denied</body></html>")
		c.Abort()
		return
	}

	jobIDStr := c.Param("jobID")
	jobUUID, err := uuid.Parse(jobIDStr)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	jobPgID := pgtype.UUID{Bytes: jobUUID, Valid: true}

	applicationIDStr := c.Param("applicationID")
	appUUID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	appPgID := pgtype.UUID{Bytes: appUUID, Valid: true}

	application, err := app.db.GetApplicationByID(c.Request.Context(), appPgID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.String(http.StatusNotFound, "Application not found.")
		} else {
			fmt.Printf("Request Interview POST: DB error fetching application %s: %v\n", applicationIDStr, err)
			c.String(http.StatusInternalServerError, "Error verifying application.")
		}
		return
	}

	if !application.JobPostingID.Valid || application.JobPostingID.Bytes != jobPgID.Bytes {
		fmt.Printf("Request Interview POST: Application %s does not belong to Job %s\n", applicationIDStr, jobIDStr)
		c.String(http.StatusBadRequest, "Application/Job mismatch.")
		return
	}

	if !application.RecruiterID.Valid || application.RecruiterID.Bytes != recruiterPgID.Bytes {
		fmt.Printf("Request Interview POST: Recruiter %s does not own job %s\n", recruiterPgID.String(), jobIDStr)
		c.String(http.StatusForbidden, "Forbidden: You do not own the job posting for this application.")
		return
	}

	currentStatus := application.Status
	if currentStatus != "submitted" {
		fmt.Printf("Request Interview POST: Cannot request interview for application %s with status '%s'\n", applicationIDStr, currentStatus)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, fmt.Sprintf("<html><body>Cannot request interview for application with status '%s'. <a href='/recruiter/jobs/%s/applications'>Back</a></body></html>", html.EscapeString(currentStatus), jobIDStr))
		return
	}

	newStatus := "accepted"
	params := db.UpdateApplicationStatusParams{
		ID:     appPgID,
		Status: newStatus,
	}
	err = app.db.UpdateApplicationStatus(c.Request.Context(), params)
	if err != nil {
		fmt.Printf("Request Interview POST: DB error updating status for app %s: %v\n", applicationIDStr, err)
		c.String(http.StatusInternalServerError, "Failed to update application status.")
		return
	}
	fmt.Printf("Application %s status updated to '%s' by recruiter %s\n", applicationIDStr, newStatus, recruiterPgID.String())

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpFrom := os.Getenv("SMTP_FROM_EMAIL")
	applicantEmail := application.ApplicantEmail
	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPassword == "" || smtpFrom == "" || applicantEmail == "" {
		fmt.Printf("Request Interview POST: Skipping email notification for app %s due to missing SMTP config or applicant email.\n", applicationIDStr)
		c.String(http.StatusInternalServerError, "SMTP configuration or applicant email is missing.")
		return
	} else {

		subject := "Interview Request for Job Application"
		body := fmt.Sprintln("Hello,\n\nYou have been invited for an interview for a job you applied for.\nPlease log in to your dashboard to view details.\n\nRegards,\nRecruitment Team")
		msg := []byte("To: " + applicantEmail + "\r\n" +
			"From: " + smtpFrom + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"\r\n" +
			body + "\r\n")

		auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpHost)

		smtpAddr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
		err := smtp.SendMail(smtpAddr, auth, smtpFrom, []string{applicantEmail}, msg)
		if err != nil {

			fmt.Printf("Request Interview POST: FAILED to send email notification to %s for app %s: %v\n", applicantEmail, applicationIDStr, err)
		} else {
			fmt.Printf("Request Interview POST: Successfully sent email notification to %s for app %s\n", applicantEmail, applicationIDStr)
		}
	}

	redirectURL := fmt.Sprintf("/recruiter/jobs/%s/applications", jobIDStr)
	c.Redirect(http.StatusSeeOther, redirectURL)
}
