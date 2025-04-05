-- name: CreateJobPosting :one
INSERT INTO job_postings 
(recruiter_id, title, salary_min, salary_max, status) 
VALUES 
($1, $2, $3, $4, $5) 
RETURNING id, recruiter_id, title, salary_min, salary_max, status; 

-- name: ListJobPostingsByRecruiter :many
SELECT id, title, status, salary_min, salary_max 
FROM job_postings
WHERE recruiter_id = $1
ORDER BY salary_max DESC; 