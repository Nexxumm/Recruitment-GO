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

-- name: ListActiveJobPostings :many
SELECT 
    j.id, 
    j.title, 
    j.status, 
    j.salary_min, 
    j.salary_max, 
    u.name AS recruiter_name 
FROM job_postings j
JOIN users u ON j.recruiter_id = u.id
WHERE j.status = 'active' 
ORDER BY j.title; 

-- name: GetJobPostingByID :one
SELECT 
    j.id, 
    j.title, 
    j.status, 
    j.salary_min, 
    j.salary_max, 
    u.name AS recruiter_name 
FROM job_postings j
JOIN users u ON j.recruiter_id = u.id
WHERE j.id = $1;

-- name: GetApplicationsForJobPosting :many
SELECT 
    a.id AS application_id,
    a.status AS application_status,
    a.applied_at,
    u.id AS user_id,
    u.name AS user_name,
    u.email AS user_email
FROM applications a
JOIN users u ON a.user_id = u.id
WHERE a.job_posting_id = $1
ORDER BY a.applied_at ASC; 

-- name: UpdateApplicationStatus :exec
UPDATE applications
SET status = $2
WHERE id = $1; 