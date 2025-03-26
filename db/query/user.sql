-- name: CreateUser :one
INSERT INTO users 
(
    username, password, email
) VALUES 
(
    $1 , $2 , $3 
) RETURNING ID, username, email;

-- name: GetUser :one
SELECT ID, username, email FROM users 
WHERE id = $1 LIMIT 1;

-- name: GetPassword :one
SELECT password FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserID :one
SELECT id FROM users
WHERE username = $1 LIMIT 1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;