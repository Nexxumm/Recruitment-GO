-- name: CreateApplication :one
INSERT INTO applications 
(user_id, job_posting_id, status, applied_at) 
VALUES 
($1, $2, 'submitted', NOW())
RETURNING id, user_id, job_posting_id, status, applied_at; 

-- name: CheckApplicationExists :one
SELECT id 
FROM applications
WHERE user_id = $1 AND job_posting_id = $2
LIMIT 1;

-- name: GetApplicationsByUserID :many
SELECT 
    a.id AS application_id,
    a.status AS application_status,
    a.applied_at,
    j.id AS job_posting_id,
    j.title AS job_title
FROM applications a
JOIN job_postings j ON a.job_posting_id = j.id
WHERE a.user_id = $1
ORDER BY a.applied_at DESC; -- Show most recent applications first