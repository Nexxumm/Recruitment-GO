-- name: ListSkills :many
SELECT id, name 
FROM skills
ORDER BY name;

-- name: GetUserSkillIDs :many
SELECT skill_id 
FROM user_skills
WHERE user_id = $1;

-- name: DeleteUserSkills :exec
DELETE FROM user_skills
WHERE user_id = $1;

-- name: AddSkillToUser :exec
INSERT INTO user_skills (user_id, skill_id)
VALUES ($1, $2)
ON CONFLICT (user_id, skill_id) DO NOTHING;

-- name: GetUserSkillNames :many
SELECT s.name 
FROM skills s
JOIN user_skills us ON s.id = us.skill_id
WHERE us.user_id = $1
ORDER BY s.name;


-- name: SearchApplicantsBySkills :many
SELECT u.id, u.name, u.email, u.role 
FROM users u
JOIN user_skills us ON u.id = us.user_id
WHERE u.role = 'applicant' 
AND us.skill_id = ANY ($1) 
GROUP BY u.id
HAVING COUNT(DISTINCT us.skill_id) = $2; 