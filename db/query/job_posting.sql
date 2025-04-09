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