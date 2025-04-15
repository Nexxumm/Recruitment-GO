package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	db "Recruitment-GO/internal/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

func (app *App) getJobPostingFormHandler(c *gin.Context) {
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
		c.String(http.StatusInternalServerError, "Internal Server Error: %v", err)
		c.Abort()
		return
	}
	if user.Role != RoleRecruiter {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Only recruiters can post jobs</body></html>")
		c.Abort()
		return
	}

	formHTML := `
        <h2>Create New Job Posting</h2>
        <form method="POST" action="/jobs">
            <div>
                <label for="title">Job Title:</label><br>
                <input type="text" id="title" name="title" required>
            </div>
            <br>
            <div>
                <label for="salary_min">Minimum Salary (Optional):</label><br>
                <input type="number" step="0.01" id="salary_min" name="salary_min" placeholder="e.g., 50000.00">
            </div>
            <br>
            <div>
                <label for="salary_max">Maximum Salary (Optional):</label><br>
                <input type="number" step="0.01" id="salary_max" name="salary_max" placeholder="e.g., 80000.00">
            </div>
            <br>
            <button type="submit">Create Job Posting</button>
        </form>
        <br>
        <p><a href="/recruiter/dashboard">Back to Dashboard</a></p>
    `

	fullHTML := fmt.Sprintf(`
        <!DOCTYPE html><html><head><title>New Job Posting</title></head><body>
        <nav>...</nav><hr>
        %s
        <hr><footer>...</footer></body></html>`, formHTML)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) createJobPostingHandler(c *gin.Context) {
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
		c.String(http.StatusInternalServerError, "Internal Server Error: %v", err)
		c.Abort()
		return
	}
	if user.Role != RoleRecruiter {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Only recruiters can post jobs</body></html>")
		c.Abort()
		return
	}

	title := c.PostForm("title")
	salaryMinStr := c.PostForm("salary_min")
	salaryMaxStr := c.PostForm("salary_max")

	if strings.TrimSpace(title) == "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>Job title is required.</body></html>")
		return
	}

	var salaryMinPg pgtype.Numeric
	var salaryMaxPg pgtype.Numeric
	var decMin, decMax decimal.Decimal
	var minSet, maxSet bool

	if salaryMinStr != "" {
		decMin, err = decimal.NewFromString(salaryMinStr)
		if err != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusBadRequest, "<html><body>Invalid Minimum Salary format.</body></html>")
			return
		} else {
			err = salaryMinPg.Scan(decMin.String())
			if err != nil {
				c.String(http.StatusBadRequest, "<html><body>Error processing Minimum Salary: %v</body></html>", err)
				c.Abort()
				return
			} else {
				minSet = true
			}
		}
	}

	if salaryMaxStr != "" {
		decMax, err = decimal.NewFromString(salaryMaxStr)
		if err != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusBadRequest, "<html><body>Invalid Maximum Salary format.</body></html>")
			return
		} else {
			err = salaryMaxPg.Scan(decMax.String())
			if err != nil {
				c.String(http.StatusBadRequest, "<html><body>Error processing Maximum Salary: %v</body></html>", err)
				c.Abort()
				return
			} else {
				maxSet = true
			}
		}
	}

	if minSet && maxSet && decMin.GreaterThan(decMax) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>Minimum Salary cannot be greater than Maximum Salary.</body></html>")
		return
	}

	params := db.CreateJobPostingParams{
		RecruiterID: pgID,
		Title:       title,
		SalaryMin:   salaryMinPg,
		SalaryMax:   salaryMaxPg,
		Status:      "active",
	}

	_, dbErr := app.db.CreateJobPosting(c.Request.Context(), params)
	if dbErr != nil {
		fmt.Printf("Create Job Posting DB Error: %v\n", dbErr)
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusInternalServerError, "<html><body>Failed to create job posting. Please try again.</body></html>")
		return
	}

	fmt.Printf("Successfully created job posting '%s' by recruiter %s\n", title, pgID.String())
	c.Redirect(http.StatusSeeOther, "/recruiter/dashboard")
}

func (app *App) getApplicantProfileByRecruiterHandler(c *gin.Context) {
	recruiterUserID, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	recruiterPgID, ok := recruiterUserID.(pgtype.UUID)
	if !ok || !recruiterPgID.Valid {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	recruiterUser, err := app.db.GetUser(c.Request.Context(), recruiterPgID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error: %v", err)
		c.Abort()
		return
	}
	if recruiterUser.Role != RoleRecruiter {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Access denied</body></html>")
		c.Abort()
		return
	}

	applicantIDStr := c.Param("applicantID")
	applicantUUID, err := uuid.Parse(applicantIDStr)
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>Invalid Applicant ID </body></html>")
		return
	}
	applicantPgID := pgtype.UUID{Bytes: applicantUUID, Valid: true}

	applicantUser, err := app.db.GetUser(c.Request.Context(), applicantPgID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusNotFound, "<html><body>Applicant not does not exist</body></html>")
		} else {
			fmt.Printf("Applicant Profile View: DB error fetching applicant %s: %v\n", applicantIDStr, err)
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusInternalServerError, "<html><body>Error fetching applicant data</body></html>")
		}
		return
	}

	if applicantUser.Role != RoleApplicant {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusForbidden, "<html><body>Forbidden: Cannot fetch details for this user</body></html>")
		c.Abort()
		return
	}

	skillNames, err := app.db.GetUserSkillNames(c.Request.Context(), applicantPgID)
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Applicant Profile View: Failed to get skills for %s: %v\n", applicantIDStr, err)
	}
	var skillsHTML string
	if len(skillNames) == 0 {
		skillsHTML = "<p>No skills listed.</p>"
	} else {
		skillsHTML = "<ul>"
		for _, name := range skillNames {
			skillsHTML += fmt.Sprintf("<li>%s</li>", name)
		}
		skillsHTML += "</ul>"
	}

	parsedResume, err := app.db.GetParsedResume(c.Request.Context(), applicantPgID)
	parsedResumeHtml := ""
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Applicant Profile View: Error checking resume for %s: %v\n", applicantIDStr, err)
		parsedResumeHtml = "<p style='color:red;'>Error checking resume status.</p>"
	} else if len(parsedResume) == 0 || err == sql.ErrNoRows {
		parsedResumeHtml = "<p>No resume uploaded.</p>"
	} else {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, parsedResume, "", "    "); err == nil {
			parsedResumeHtml = fmt.Sprintf("<pre><code>%s</code></pre>", prettyJSON.String())

		} else {
			parsedResumeHtml = fmt.Sprintf("<pre><code>%s</code></pre>", prettyJSON.String())
		}

	}

	applicantProfileHTML := fmt.Sprintf(`
        <h2>Applicant Profile</h2>
        <p><strong>Name:</strong> %s</p>
        <p><strong>Email:</strong> %s</p>
        <hr>
        <h3>Skills</h3>
        %s
        <hr>
        <h3>Resume</h3>
        %s
        <hr>
        `,
		applicantUser.Name,
		applicantUser.Email,
		skillsHTML,
		parsedResumeHtml,
	)

	fullHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Applicant Profile - %s</title></head><body>
		<nav>...</nav><hr>
		%s
		<hr>
		<p><a href="/recruiter/search">Back to Search Results</a></p>
        <p><a href="/recruiter/dashboard">Back to Dashboard</a></p>
		</body></html>`,
		applicantUser.Name,
		applicantProfileHTML)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) listJobsHandler(c *gin.Context) {
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

	postings, err := app.db.ListActiveJobPostings(c.Request.Context())
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("List Jobs: DB error listing active jobs: %v\n", err)
	}

	var jobsListHTML strings.Builder
	jobsListHTML.WriteString("<h2>Available Job Postings</h2>")

	if err != nil && err != sql.ErrNoRows {
		jobsListHTML.WriteString("<p style='color:red;'>Error loading job postings.</p>")
	} else if len(postings) == 0 {
		jobsListHTML.WriteString("<p>There are currently no active job postings.</p>")
	} else {
		jobsListHTML.WriteString("<table border='1' style='border-collapse: collapse; width: 80%;'>")
		jobsListHTML.WriteString("<thead><tr><th>Title</th><th>Recruiter</th><th>Salary Min</th><th>Salary Max</th><th>Status</th><th>Action</th></tr></thead>")
		jobsListHTML.WriteString("<tbody>")
		for _, posting := range postings {
			var jobIDStr string
			if posting.ID.Valid {
				jobIDStr = uuid.UUID(posting.ID.Bytes).String()
			} else {
				continue
			}

			salaryMinVal, err := posting.SalaryMin.Value()
			if err != nil {
				salaryMinVal = ""
			}
			salaryMaxVal, err := posting.SalaryMax.Value()
			if err != nil {
				salaryMaxVal = ""
			}

			applyLink := fmt.Sprintf("/jobs/%s/apply", jobIDStr)

			jobsListHTML.WriteString("<tr>")
			jobsListHTML.WriteString(fmt.Sprintf("<td>%s</td>", posting.Title))
			jobsListHTML.WriteString(fmt.Sprintf("<td>%s</td>", posting.Status))
			jobsListHTML.WriteString(fmt.Sprintf("<td>%s</td>", salaryMinVal))
			jobsListHTML.WriteString(fmt.Sprintf("<td>%v</td>", salaryMaxVal))
			jobsListHTML.WriteString(fmt.Sprintf("<td>%v</td>", posting.Status))
			jobsListHTML.WriteString(fmt.Sprintf(`<td><a href="%s">Apply</a></td>`, applyLink))
			jobsListHTML.WriteString("</tr>")
		}
		jobsListHTML.WriteString("</tbody></table>")
	}

	backLink := "/"
	if err == nil {
		if user.Role == RoleApplicant {
			backLink = "/applicant/dashboard"
		} else if user.Role == RoleRecruiter {
			backLink = "/recruiter/dashboard"
		}
	}

	fullHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Available Jobs</title></head><body>
		<nav>...</nav><hr>
		%s 
		<hr>
		<p><a href="%s">Back to Dashboard</a></p>
		<p><a href="/logout">Logout</a></p>
		<footer>...</footer></body></html>`,
		jobsListHTML.String(),
		backLink,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) getJobApplicationsHandler(c *gin.Context) {

	jobIDStr := c.Param("jobID")
	jobUUID, err := uuid.Parse(jobIDStr)
	if err != nil {

		return
	}
	jobPgID := pgtype.UUID{Bytes: jobUUID, Valid: true}

	job, err := app.db.GetJobPostingByID(c.Request.Context(), jobPgID)
	if err != nil {
		if err == sql.ErrNoRows {
			c.String(http.StatusNotFound, "Job posting not found")
		} else {
			c.String(http.StatusInternalServerError, "Error fetching job data")
		}
		return
	}

	applications, err := app.db.GetApplicationsForJobPosting(c.Request.Context(), jobPgID)
	if err != nil && err != sql.ErrNoRows {
		fmt.Printf("Manage Applications GET: DB error fetching applications for job %s: %v\n", jobIDStr, err)
		c.String(http.StatusInternalServerError, "Failed to fetch applications for job posting: %v", err)
	}

	var applicationsHTML strings.Builder
	applicationsHTML.WriteString(fmt.Sprintf("<h2>Applications for: %s</h2>", job.Title))

	if err != nil && err != sql.ErrNoRows {
		applicationsHTML.WriteString("<p style='color:red;'>Error loading applications.</p>")
	} else if len(applications) == 0 {
		applicationsHTML.WriteString("<p>No applications received yet.</p>")
	} else {
		applicationsHTML.WriteString("<table border='1' style='border-collapse: collapse;'>")
		applicationsHTML.WriteString("<thead><tr><th>Applicant Name</th><th>Email</th><th>Status</th><th>Applied At</th><th>Actions</th></tr></thead>")
		applicationsHTML.WriteString("<tbody>")
		for _, application := range applications {
			var appIDStr string
			if application.ApplicationID.Valid {
				appIDStr = uuid.UUID(application.ApplicationID.Bytes).String()
			} else {
				continue
			}

			appliedAtStr := "N/A"
			if application.AppliedAt.Valid {
				appliedAtStr = application.AppliedAt.Time.Format(time.RFC822)
			}

			rejectForm := fmt.Sprintf(`<form method="POST" action="/recruiter/jobs/%s/applications/%s/reject" style="display:inline;"><button type="submit">Reject</button></form>`, jobIDStr, appIDStr)
			interviewForm := fmt.Sprintf(`<form method="POST" action="/recruiter/jobs/%s/applications/%s/request_interview" style="display:inline;"><button type="submit">Request Interview</button></form>`, jobIDStr, appIDStr)

			if application.ApplicationStatus == "rejected" {

				rejectForm = "<span>Rejected</span>"
				interviewForm = ""
			}

			applicationsHTML.WriteString("<tr>")
			applicationsHTML.WriteString(fmt.Sprintf("<td>%s</td>", application.UserName))
			applicationsHTML.WriteString(fmt.Sprintf("<td>%s</td>", application.UserEmail))
			applicationsHTML.WriteString(fmt.Sprintf("<td>%s</td>", application.ApplicationStatus))
			applicationsHTML.WriteString(fmt.Sprintf("<td>%s</td>", appliedAtStr))
			applicationsHTML.WriteString(fmt.Sprintf("<td>%s %s</td>", rejectForm, interviewForm))
			applicationsHTML.WriteString("</tr>")
		}
		applicationsHTML.WriteString("</tbody></table>")
	}

	fullHTML := fmt.Sprintf(`
		<!DOCTYPE html><html><head><title>Manage Applications</title></head><body>
		<nav>...</nav><hr>
		%s
		<hr>
		<p><a href="/recruiter/dashboard">Back to Dashboard</a></p>
		</body></html>`,
		applicationsHTML.String(),
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, fullHTML)
}

func (app *App) rejectApplicationHandler(c *gin.Context) {
	recruiterID, exists := c.Get("userID")
	if !exists {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}
	recruiterPgID, ok := recruiterID.(pgtype.UUID)
	if !ok || !recruiterPgID.Valid {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		return
	}

	recruiterUser, err := app.db.GetUser(c.Request.Context(), recruiterPgID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error: %v", err)
		c.Abort()
		return
	}
	if recruiterUser.Role != RoleRecruiter {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		c.Abort()
		return
	}

	jobIDStr := c.Param("jobID")
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/")
		return
	}

	applicationIDStr := c.Param("applicationID")
	appUUID, err := uuid.Parse(applicationIDStr)
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<html><body>Invalid Application ID format</body></html>")
		return
	}
	appPgID := pgtype.UUID{Bytes: appUUID, Valid: true}

	params := db.UpdateApplicationStatusParams{
		ID:     appPgID,
		Status: "rejected",
	}
	err = app.db.UpdateApplicationStatus(c.Request.Context(), params)
	if err != nil {
		fmt.Printf("Reject Application POST: DB error updating status for app %s: %v\n", applicationIDStr, err)
		c.String(http.StatusInternalServerError, "Failed to update application status.")
		return
	}

	fmt.Printf("Application %s rejected by recruiter %s\n", applicationIDStr, recruiterPgID.String())

	redirectURL := fmt.Sprintf("/recruiter/jobs/%s/applications", jobIDStr)
	c.Redirect(http.StatusSeeOther, redirectURL)
}
