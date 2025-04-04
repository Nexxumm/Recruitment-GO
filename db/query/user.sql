-- name: CreateUser :one
INSERT INTO users 
(
    google_id,email, name, role
) VALUES 
(
    $1 , $2 , $3 , $4
) RETURNING ID, name, email;

-- name: GetUser :one
SELECT ID, name, email,role,google_id FROM users 
WHERE id = $1 LIMIT 1;

-- name: GetGoogleID :one
SELECT google_id FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserID :one
SELECT id FROM users
WHERE name = $1 LIMIT 1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: GetUserByGoogleID :one
SELECT id, google_id, email, name, role
FROM users
WHERE google_id = $1 
LIMIT 1; 