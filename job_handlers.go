package main

import (
	"fmt"
	"net/http"
	"strings"

	db "Recruitment-GO/internal/db"

	"github.com/gin-gonic/gin"
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
