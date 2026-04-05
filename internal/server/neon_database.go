package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/harshul/octo-cli/internal/blueprint"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NeonDB handles all database operations with Neon PostgreSQL
type NeonDB struct {
	pool *pgxpool.Pool
}

// NewNeonDB creates a new Neon database client
func NewNeonDB(databaseURL string) (*NeonDB, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &NeonDB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *NeonDB) Close() {
	db.pool.Close()
}

// --- User Operations ---

// CreateOrUpdateUser creates or updates a user from GitHub OAuth data
func (db *NeonDB) CreateOrUpdateUser(ctx context.Context, githubID, email, name, avatarURL, githubUsername, accessToken string) (*User, error) {
	userID := uuid.New().String()
	
	query := `
		INSERT INTO users (id, github_id, email, name, avatar_url, github_username, access_token, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (github_id) 
		DO UPDATE SET
			email = EXCLUDED.email,
			name = EXCLUDED.name,
			avatar_url = EXCLUDED.avatar_url,
			github_username = EXCLUDED.github_username,
			access_token = EXCLUDED.access_token,
			updated_at = NOW()
		RETURNING id, github_id, email, name, avatar_url, github_username, access_token
	`

	var user User
	err := db.pool.QueryRow(ctx, query, userID, githubID, email, name, avatarURL, githubUsername, accessToken).
		Scan(&user.ID, &user.GitHubID, &user.Email, &user.Name, &user.AvatarURL, &user.GitHubUsername, &user.GitHubToken)
	
	if err != nil {
		return nil, fmt.Errorf("failed to create/update user: %w", err)
	}

	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (db *NeonDB) GetUserByID(ctx context.Context, userID string) (*User, error) {
	query := `SELECT id, github_id, email, name, avatar_url, github_username, access_token FROM users WHERE id = $1`
	
	var user User
	err := db.pool.QueryRow(ctx, query, userID).
		Scan(&user.ID, &user.GitHubID, &user.Email, &user.Name, &user.AvatarURL, &user.GitHubUsername, &user.GitHubToken)
	
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &user, nil
}

// GetUserAccessToken retrieves a user's GitHub access token
func (db *NeonDB) GetUserAccessToken(ctx context.Context, userID string) (string, error) {
	var token string
	query := `SELECT access_token FROM users WHERE id = $1`
	err := db.pool.QueryRow(ctx, query, userID).Scan(&token)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	return token, nil
}

// --- Session Operations ---

// CreateSession creates a new session for a user
func (db *NeonDB) CreateSession(ctx context.Context, userID string, expiresAt time.Time) (string, error) {
	sessionToken := uuid.New().String()
	sessionID := uuid.New().String()

	query := `
		INSERT INTO sessions (id, user_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := db.pool.Exec(ctx, query, sessionID, userID, sessionToken, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionToken, nil
}

// ValidateSession validates a session token and returns the user ID
func (db *NeonDB) ValidateSession(ctx context.Context, token string) (string, error) {
	var userID string
	var expiresAt time.Time

	query := `SELECT user_id, expires_at FROM sessions WHERE token = $1`
	err := db.pool.QueryRow(ctx, query, token).Scan(&userID, &expiresAt)
	
	if err != nil {
		return "", fmt.Errorf("invalid session: %w", err)
	}

	if time.Now().After(expiresAt) {
		// Delete expired session
		db.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
		return "", fmt.Errorf("session expired")
	}

	return userID, nil
}

// DeleteSession deletes a session (logout)
func (db *NeonDB) DeleteSession(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := db.pool.Exec(ctx, query, token)
	return err
}

// CleanupExpiredSessions removes all expired sessions
func (db *NeonDB) CleanupExpiredSessions(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	_, err := db.pool.Exec(ctx, query)
	return err
}

// --- Project Operations ---

// ListProjects retrieves all projects for a user
func (db *NeonDB) ListProjects(ctx context.Context, userID string) ([]Project, error) {
	query := `
		SELECT id, name, repo_url, description, language, is_public, stars, 
		       user_id, user_name, avatar_url, blueprint, created_at, updated_at
		FROM projects 
		WHERE user_id = $1 
		ORDER BY updated_at DESC
	`

	rows, err := db.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var blueprintJSON []byte

		err := rows.Scan(
			&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
			&p.UserID, &p.UserName, &p.AvatarURL, &blueprintJSON, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(blueprintJSON) > 0 {
			var bp blueprint.Blueprint
			if err := json.Unmarshal(blueprintJSON, &bp); err == nil {
				p.Blueprint = &bp
			}
		}

		projects = append(projects, p)
	}

	return projects, nil
}

// GetProject retrieves a project by ID for a specific user
func (db *NeonDB) GetProject(ctx context.Context, id, userID string) (*Project, error) {
	query := `
		SELECT id, name, repo_url, description, language, is_public, stars, 
		       user_id, user_name, avatar_url, blueprint, created_at, updated_at
		FROM projects 
		WHERE id = $1 AND user_id = $2
	`

	var p Project
	var blueprintJSON []byte

	err := db.pool.QueryRow(ctx, query, id, userID).Scan(
		&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
		&p.UserID, &p.UserName, &p.AvatarURL, &blueprintJSON, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	if len(blueprintJSON) > 0 {
		var bp blueprint.Blueprint
		if err := json.Unmarshal(blueprintJSON, &bp); err == nil {
			p.Blueprint = &bp
		}
	}

	return &p, nil
}

// CreateProject creates a new project
func (db *NeonDB) CreateProject(ctx context.Context, p *Project) (*Project, error) {
	projectID := uuid.New().String()
	
	blueprintJSON, _ := json.Marshal(p.Blueprint)

	query := `
		INSERT INTO projects (id, name, repo_url, description, language, is_public, stars, user_id, blueprint, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, name, repo_url, description, language, is_public, stars, user_id, user_name, avatar_url, created_at, updated_at
	`

	err := db.pool.QueryRow(ctx, query,
		projectID, p.Name, p.RepoURL, p.Description, p.Language, p.IsPublic, p.Stars, p.UserID, blueprintJSON,
	).Scan(
		&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
		&p.UserID, &p.UserName, &p.AvatarURL, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	return p, nil
}

// UpdateProject updates a project
func (db *NeonDB) UpdateProject(ctx context.Context, id, userID string, update map[string]interface{}) (*Project, error) {
	// Build dynamic update query
	query := `
		UPDATE projects 
		SET name = COALESCE($3, name),
		    description = COALESCE($4, description),
		    is_public = COALESCE($5, is_public),
		    updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, name, repo_url, description, language, is_public, stars, user_id, user_name, avatar_url, blueprint, created_at, updated_at
	`

	var p Project
	var blueprintJSON []byte

	err := db.pool.QueryRow(ctx, query, id, userID, update["name"], update["description"], update["is_public"]).
		Scan(&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
			&p.UserID, &p.UserName, &p.AvatarURL, &blueprintJSON, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}

	if len(blueprintJSON) > 0 {
		var bp blueprint.Blueprint
		if err := json.Unmarshal(blueprintJSON, &bp); err == nil {
			p.Blueprint = &bp
		}
	}

	return &p, nil
}

// DeleteProject deletes a project
func (db *NeonDB) DeleteProject(ctx context.Context, id, userID string) error {
	query := `DELETE FROM projects WHERE id = $1 AND user_id = $2`
	result, err := db.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// UpdateBlueprint updates a project's blueprint
func (db *NeonDB) UpdateBlueprint(ctx context.Context, id, userID string, bp *blueprint.Blueprint) error {
	blueprintJSON, _ := json.Marshal(bp)

	query := `
		UPDATE projects 
		SET blueprint = $3, language = $4, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
	`

	result, err := db.pool.Exec(ctx, query, id, userID, blueprintJSON, bp.Language)
	if err != nil {
		return fmt.Errorf("failed to update blueprint: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}

// UpsertProject creates or updates a project (based on repo_url)
func (db *NeonDB) UpsertProject(ctx context.Context, p *Project) (*Project, error) {
	projectID := uuid.New().String()
	blueprintJSON, _ := json.Marshal(p.Blueprint)

	query := `
		INSERT INTO projects (id, name, repo_url, description, language, is_public, stars, user_id, blueprint, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (user_id, repo_url) WHERE repo_url != ''
		DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			language = EXCLUDED.language,
			blueprint = EXCLUDED.blueprint,
			updated_at = NOW()
		RETURNING id, name, repo_url, description, language, is_public, stars, user_id, user_name, avatar_url, created_at, updated_at
	`

	err := db.pool.QueryRow(ctx, query,
		projectID, p.Name, p.RepoURL, p.Description, p.Language, p.IsPublic, p.Stars, p.UserID, blueprintJSON,
	).Scan(
		&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
		&p.UserID, &p.UserName, &p.AvatarURL, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to upsert project: %w", err)
	}

	return p, nil
}

// --- Public Project Operations ---

// ListPublicProjects retrieves public projects with filters
func (db *NeonDB) ListPublicProjects(ctx context.Context, language, search, sortBy string, limit int) ([]Project, error) {
	query := `
		SELECT id, name, repo_url, description, language, is_public, stars, 
		       user_id, user_name, avatar_url, blueprint, created_at, updated_at
		FROM projects 
		WHERE is_public = true
	`

	args := []interface{}{}
	argIdx := 1

	if language != "" {
		query += fmt.Sprintf(" AND language = $%d", argIdx)
		args = append(args, language)
		argIdx++
	}

	if search != "" {
		query += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	switch sortBy {
	case "stars":
		query += " ORDER BY stars DESC"
	case "newest":
		query += " ORDER BY created_at DESC"
	default:
		query += " ORDER BY updated_at DESC"
	}

	if limit == 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list public projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		var blueprintJSON []byte

		err := rows.Scan(
			&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
			&p.UserID, &p.UserName, &p.AvatarURL, &blueprintJSON, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if len(blueprintJSON) > 0 {
			var bp blueprint.Blueprint
			if err := json.Unmarshal(blueprintJSON, &bp); err == nil {
				p.Blueprint = &bp
			}
		}

		projects = append(projects, p)
	}

	return projects, nil
}

// GetPublicProject retrieves a public project by ID
func (db *NeonDB) GetPublicProject(ctx context.Context, id string) (*Project, error) {
	query := `
		SELECT id, name, repo_url, description, language, is_public, stars, 
		       user_id, user_name, avatar_url, blueprint, created_at, updated_at
		FROM projects 
		WHERE id = $1 AND is_public = true
	`

	var p Project
	var blueprintJSON []byte

	err := db.pool.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.RepoURL, &p.Description, &p.Language, &p.IsPublic, &p.Stars,
		&p.UserID, &p.UserName, &p.AvatarURL, &blueprintJSON, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	if len(blueprintJSON) > 0 {
		var bp blueprint.Blueprint
		if err := json.Unmarshal(blueprintJSON, &bp); err == nil {
			p.Blueprint = &bp
		}
	}

	return &p, nil
}

// --- Activity Log Operations ---

// LogActivity records a user action
func (db *NeonDB) LogActivity(ctx context.Context, userID, projectID, action string, metadata map[string]interface{}) error {
	activityID := uuid.New().String()
	metadataJSON, _ := json.Marshal(metadata)

	query := `
		INSERT INTO activity_log (id, user_id, project_id, action, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`

	var projID interface{} = projectID
	if projectID == "" {
		projID = nil
	}

	_, err := db.pool.Exec(ctx, query, activityID, userID, projID, action, metadataJSON)
	return err
}
