package main

import (
	db "Recruitment-GO/internal/db"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

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

	parsedResume, err := sendPDFToGemini(resumeBytes)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error in Sending PDF to Gemini: %v", err)
		c.Abort()
		return
	}
	var jsonBytes []byte
	jsonBytes, err = json.Marshal(parsedResume)
	if err != nil {
		fmt.Printf("Post Resume Handler: Error marshalling parsed resume to JSON: %v\n", err)
		c.String(http.StatusInternalServerError, "<html><body>Error processing parsed resume data.</body></html>")
		c.Abort()
		return
	}

	paramsParsed := db.UpdateParsedResumeParams{
		ID:           pgID,
		ParsedResume: jsonBytes,
	}

	err = app.db.UpdateParsedResume(c.Request.Context(), paramsParsed)

	if err != nil {
		fmt.Printf("Post Resume Handler: DB error updating parsed resume for user %s: %v\n", pgID.String(), err)
		c.String(http.StatusInternalServerError, "<html><body>Error saving parsed resume to database. Please try again.</body></html>")
		return
	}

	fmt.Printf("Successfully updated Parsed resume for user %s\n", pgID.String())
	c.Redirect(http.StatusSeeOther, "/applicant/dashboard")
}

func sendPDFToGemini(fileBytes []byte) (map[string]interface{}, error) {
	apiKey := os.Getenv("API_KEY")
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=" + apiKey

	// Encode base64
	base64PDF := base64.StdEncoding.EncodeToString(fileBytes)

	// Build request
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": "Extract JSON with name, skills, work experience from this resume."},
					{
						"inlineData": map[string]interface{}{
							"mimeType": "application/pdf",
							"data":     base64PDF,
						},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}

	var parsed map[string]any
	if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {

		cleanText := strings.Trim(result.Candidates[0].Content.Parts[0].Text, "` \n")
		cleanText = strings.TrimPrefix(cleanText, "json")
		fmt.Println(cleanText)
		err := json.Unmarshal([]byte(cleanText), &parsed)
		if err != nil {
			return nil, err
		}
	}

	return parsed, nil
}
