package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
