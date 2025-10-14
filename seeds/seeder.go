package seeds

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Seeder knows how to populate foundational data for the application database.
type Seeder struct {
	db *sql.DB
}

// NewSeeder constructs a Seeder instance for the provided database handle.
func NewSeeder(db *sql.DB) (*Seeder, error) {
	//1.- Ensure callers provide a valid database connection to avoid panics.
	if db == nil {
		return nil, errors.New("database handle is required")
	}

	//2.- Return the configured seeder so callers can execute the bootstrap data flow.
	return &Seeder{db: db}, nil
}

// Run executes every seed routine inside a single transaction for consistency.
func (s *Seeder) Run(ctx context.Context) error {
	//1.- Verify the seeder has been initialized correctly before accessing the database.
	if s == nil || s.db == nil {
		return errors.New("seeder is not initialized")
	}

	//2.- Start a transaction to ensure the seeding process is atomic.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	committed := false
	defer func() {
		//3.- Roll back the transaction when an error occurs to keep the database clean.
		if !committed {
			_ = tx.Rollback()
		}
	}()

	//4.- Create or refresh the administrator role so critical permissions have an owner.
	adminRoleID, err := s.seedAdminRole(ctx, tx)
	if err != nil {
		return err
	}

	//5.- Populate the permission catalog and capture the resulting identifiers.
	permissionIDs, err := s.seedPermissionCatalog(ctx, tx)
	if err != nil {
		return err
	}

	//6.- Attach the full permission catalog to the administrator role for full access.
	if err := s.seedRolePermissions(ctx, tx, adminRoleID, permissionIDs); err != nil {
		return err
	}

	//7.- Ensure the administrator user exists and is bound to the administrator role.
	adminUserID, err := s.seedAdminUser(ctx, tx)
	if err != nil {
		return err
	}

	//8.- Ensure the user to role relationship exists for the administrator account.
	if err := s.seedUserRole(ctx, tx, adminUserID, adminRoleID); err != nil {
		return err
	}

	//9.- Insert baseline application settings that the API relies on.
	if err := s.seedDefaultSettings(ctx, tx); err != nil {
		return err
	}

	//10.- Commit the transaction after every seed completed successfully.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit seed transaction: %w", err)
	}
	committed = true

	//11.- Return nil to signal successful completion to the caller.
	return nil
}

// seedAdminRole guarantees the administrator role exists and captures its identifier.
func (s *Seeder) seedAdminRole(ctx context.Context, tx *sql.Tx) (int64, error) {
	//1.- Define the desired role payload for the administrator persona.
	const (
		roleName        = "admin"
		roleDescription = "System administrator with full access"
	)

	//2.- Upsert the role while retrieving the identifier for downstream relationships.
	query := `
INSERT INTO roles (name, description)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE
SET description = EXCLUDED.description,
    updated_at = TIMEZONE('UTC', NOW())
RETURNING id`

	var roleID int64
	if err := tx.QueryRowContext(ctx, query, roleName, roleDescription).Scan(&roleID); err != nil {
		return 0, fmt.Errorf("failed to seed admin role: %w", err)
	}

	//3.- Return the administrator role identifier to the caller.
	return roleID, nil
}

// seedPermissionCatalog upserts the baseline permission set and returns their identifiers.
func (s *Seeder) seedPermissionCatalog(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	//1.- List the permissions that should exist in a fresh installation.
	permissions := []struct {
		Name        string
		Description string
	}{
		{Name: "users.read", Description: "Read user records"},
		{Name: "users.write", Description: "Create or update user records"},
		{Name: "teams.manage", Description: "Manage teams and team memberships"},
		{Name: "settings.manage", Description: "Update global application settings"},
		{Name: "notifications.send", Description: "Send system notifications"},
	}

	//2.- Prepare the SQL statement that keeps permission descriptions in sync.
	query := `
INSERT INTO permissions (name, description)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE
SET description = EXCLUDED.description,
    updated_at = TIMEZONE('UTC', NOW())
RETURNING id`

	//3.- Execute the statement for every permission and collect their identifiers.
	identifiers := make(map[string]int64, len(permissions))
	for _, permission := range permissions {
		var permissionID int64
		if err := tx.QueryRowContext(ctx, query, permission.Name, permission.Description).Scan(&permissionID); err != nil {
			return nil, fmt.Errorf("failed to seed permission %s: %w", permission.Name, err)
		}
		identifiers[permission.Name] = permissionID
	}

	//4.- Return the map to downstream routines that require permission references.
	return identifiers, nil
}

// seedRolePermissions links the administrator role with every known permission.
func (s *Seeder) seedRolePermissions(ctx context.Context, tx *sql.Tx, roleID int64, permissionIDs map[string]int64) error {
	//1.- Define the insertion statement with idempotent conflict handling.
	query := `
INSERT INTO role_permissions (role_id, permission_id)
VALUES ($1, $2)
ON CONFLICT (role_id, permission_id) DO NOTHING`

	//2.- Attach each permission to the administrator role only once.
	for _, permissionID := range permissionIDs {
		if _, err := tx.ExecContext(ctx, query, roleID, permissionID); err != nil {
			return fmt.Errorf("failed to link role %d to permission %d: %w", roleID, permissionID, err)
		}
	}

	//3.- Return nil when every association has been processed successfully.
	return nil
}

// seedAdminUser ensures the administrator user exists and returns the account identifier.
func (s *Seeder) seedAdminUser(ctx context.Context, tx *sql.Tx) (int64, error) {
	//1.- Assemble the desired account fields for the administrator user.
	const (
		adminEmail      = "admin@example.com"
		adminFirstName  = "System"
		adminLastName   = "Administrator"
		adminStatus     = "active"
		defaultPassword = "ChangeMe123!"
	)

	//2.- Hash the default password so credentials are stored securely.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("failed to hash administrator password: %w", err)
	}

	//3.- Upsert the administrator user while returning the identifier for relations.
	query := `
INSERT INTO users (email, password_hash, first_name, last_name, status)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (email) DO UPDATE
SET first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    status = EXCLUDED.status,
    password_hash = EXCLUDED.password_hash,
    updated_at = TIMEZONE('UTC', NOW())
RETURNING id`

	var userID int64
	if err := tx.QueryRowContext(ctx, query, adminEmail, string(passwordHash), adminFirstName, adminLastName, adminStatus).Scan(&userID); err != nil {
		return 0, fmt.Errorf("failed to seed administrator user: %w", err)
	}

	//4.- Return the administrator user identifier so caller can create role bindings.
	return userID, nil
}

// seedUserRole guarantees the administrator user is bound to the administrator role.
func (s *Seeder) seedUserRole(ctx context.Context, tx *sql.Tx, userID, roleID int64) error {
	//1.- Insert the association while ignoring duplicates for idempotency.
	query := `
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING`

	//2.- Execute the statement with the provided identifiers.
	if _, err := tx.ExecContext(ctx, query, userID, roleID); err != nil {
		return fmt.Errorf("failed to link user %d to role %d: %w", userID, roleID, err)
	}

	//3.- Return nil to signal a successful relationship creation.
	return nil
}

// seedDefaultSettings provides a minimal configuration footprint for new environments.
func (s *Seeder) seedDefaultSettings(ctx context.Context, tx *sql.Tx) error {
	//1.- Describe the baseline settings that configure the application behaviour.
	settings := []struct {
		Key         string
		Value       map[string]any
		Description string
	}{
		{
			Key: "app.preferences",
			Value: map[string]any{
				"locale": "en",
				"theme":  "light",
			},
			Description: "Default user interface preferences",
		},
		{
			Key: "notifications.defaults",
			Value: map[string]any{
				"email": true,
				"sms":   false,
			},
			Description: "Baseline notification delivery preferences",
		},
	}

	//2.- Prepare the statement that keeps the JSON payload and metadata synchronized.
	query := `
INSERT INTO settings (key, value, description)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    description = EXCLUDED.description,
    updated_at = TIMEZONE('UTC', NOW())`

	//3.- Execute the upsert for each setting after marshaling the JSON payload.
	for _, setting := range settings {
		payload, err := json.Marshal(setting.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal setting %s: %w", setting.Key, err)
		}
		if _, err := tx.ExecContext(ctx, query, setting.Key, payload, setting.Description); err != nil {
			return fmt.Errorf("failed to seed setting %s: %w", setting.Key, err)
		}
	}

	//4.- Return nil after every setting has been processed successfully.
	return nil
}
