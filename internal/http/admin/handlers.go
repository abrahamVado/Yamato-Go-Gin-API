package admin

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/authorization"
)

// 1.- Permission slugs describing the capabilities guarded by the admin API.
const (
	PermissionManageUsers       = "admin.users.manage"
	PermissionManageRoles       = "admin.roles.manage"
	PermissionManagePermissions = "admin.permissions.manage"
	PermissionManageTeams       = "admin.teams.manage"
)

// 1.- Pagination carries common paging parameters shared across listing handlers.
type Pagination struct {
	Page    int
	PerPage int
}

// 1.- User represents the serialized form returned by admin handlers.
type User struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
	Teams []string `json:"teams"`
}

// 1.- Role captures the role representation exposed over HTTP.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// 1.- Permission models a single capability that can be assigned to roles.
type Permission struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// 1.- Team represents a logical grouping of users.
type Team struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// 1.- Authorizer abstracts the RBAC policy used by the handlers and middleware.
type Authorizer interface {
	Authorize(internalauth.Principal, authorization.Gate) error
}

// 1.- UserService defines the persistence contract for user operations.
type UserService interface {
	Create(ctx context.Context, payload User) (User, error)
	Update(ctx context.Context, id string, payload User) (User, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pagination Pagination) ([]User, int, error)
}

// 1.- RoleService defines the persistence contract for role operations.
type RoleService interface {
	Create(ctx context.Context, payload Role) (Role, error)
	Update(ctx context.Context, id string, payload Role) (Role, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pagination Pagination) ([]Role, int, error)
}

// 1.- PermissionService defines the persistence contract for permission operations.
type PermissionService interface {
	Create(ctx context.Context, payload Permission) (Permission, error)
	Update(ctx context.Context, id string, payload Permission) (Permission, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pagination Pagination) ([]Permission, int, error)
}

// 1.- TeamService defines the persistence contract for team operations.
type TeamService interface {
	Create(ctx context.Context, payload Team) (Team, error)
	Update(ctx context.Context, id string, payload Team) (Team, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pagination Pagination) ([]Team, int, error)
}

// 1.- Handler aggregates service dependencies and the RBAC policy for admin routes.
type Handler struct {
	authorizer  Authorizer
	users       UserService
	roles       RoleService
	permissions PermissionService
	teams       TeamService
}

// 1.- NewHandler wires the admin services and policy into a reusable Handler.
func NewHandler(authorizer Authorizer, users UserService, roles RoleService, permissions PermissionService, teams TeamService) Handler {
	return Handler{
		authorizer:  authorizer,
		users:       users,
		roles:       roles,
		permissions: permissions,
		teams:       teams,
	}
}

// 1.- successEnvelope implements the standard success payload wrapper.
type successEnvelope struct {
	Data interface{}    `json:"data"`
	Meta map[string]any `json:"meta"`
}

// 1.- errorEnvelope implements the standard error payload wrapper.
type errorEnvelope struct {
	Message string                 `json:"message"`
	Errors  map[string]interface{} `json:"errors"`
}

// 1.- paginationContextKey stores parsed pagination on the Gin context.
const paginationContextKey = "admin.pagination"

// 1.- rbacFailure describes the common error emitted when authorization fails.
var rbacFailure = map[string]interface{}{"reason": "forbidden"}

// 1.- writeSuccess serializes a successful response using the envelope structure.
func writeSuccess(ctx *gin.Context, status int, data interface{}, meta map[string]any) {
	if meta == nil {
		meta = map[string]any{}
	}
	ctx.JSON(status, successEnvelope{Data: data, Meta: meta})
}

// 1.- writeError serializes a failure response using the envelope structure.
func writeError(ctx *gin.Context, status int, message string, errs map[string]interface{}) {
	if errs == nil {
		errs = map[string]interface{}{}
	}
	ctx.JSON(status, errorEnvelope{Message: message, Errors: errs})
}

// 1.- requirePrincipal fetches the current principal or responds with 401.
func requirePrincipal(ctx *gin.Context) (internalauth.Principal, bool) {
	// 2.- Attempt to extract the principal from the Gin context bag.
	principal, ok := internalauth.PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, http.StatusUnauthorized, "missing principal", nil)
		return internalauth.Principal{}, false
	}
	return principal, true
}

// 1.- validateRequired enforces that the provided fields contain non-empty values.
func validateRequired(fields map[string]string) map[string]interface{} {
	// 2.- Allocate lazily to avoid nil map panics when no errors occur.
	var errs map[string]interface{}
	for field, value := range fields {
		if strings.TrimSpace(value) == "" {
			if errs == nil {
				errs = make(map[string]interface{})
			}
			errs[field] = "is required"
		}
	}
	return errs
}

// 1.- normalizeUser trims whitespace and lowercases the email field.
func normalizeUser(payload *User) {
	payload.Email = strings.ToLower(strings.TrimSpace(payload.Email))
	payload.Roles = dedupeStrings(payload.Roles)
	payload.Teams = dedupeStrings(payload.Teams)
}

// 1.- normalizeRole ensures the role payload uses canonical formatting.
func normalizeRole(payload *Role) {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Permissions = dedupeStrings(payload.Permissions)
}

// 1.- normalizePermission ensures the permission payload uses canonical formatting.
func normalizePermission(payload *Permission) {
	payload.Name = strings.TrimSpace(payload.Name)
}

// 1.- normalizeTeam ensures the team payload uses canonical formatting.
func normalizeTeam(payload *Team) {
	payload.Name = strings.TrimSpace(payload.Name)
}

// 1.- dedupeStrings removes duplicate entries while preserving order.
func dedupeStrings(values []string) []string {
	// 2.- Track encountered values to skip duplicates.
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

// 1.- parsePagination reads the page and per_page query parameters with defaults.
func parsePagination(ctx *gin.Context) (Pagination, error) {
	// 2.- Start with safe defaults that ensure deterministic responses.
	pagination := Pagination{Page: 1, PerPage: 20}
	pageParam := ctx.Query("page")
	if pageParam != "" {
		pageValue, err := strconv.Atoi(pageParam)
		if err != nil || pageValue <= 0 {
			return Pagination{}, errors.New("page must be a positive integer")
		}
		pagination.Page = pageValue
	}
	perPageParam := ctx.Query("per_page")
	if perPageParam != "" {
		perPageValue, err := strconv.Atoi(perPageParam)
		if err != nil || perPageValue <= 0 {
			return Pagination{}, errors.New("per_page must be a positive integer")
		}
		pagination.PerPage = perPageValue
	}
	return pagination, nil
}

// 1.- RBAC builds middleware that enforces permissions and captures pagination.
func (h Handler) RBAC(permission string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 2.- Ensure the request contains a valid principal before authorizing.
		principal, ok := requirePrincipal(ctx)
		if !ok {
			ctx.Abort()
			return
		}

		// 3.- Delegate authorization checks to the configured policy.
		gate := authorization.Gate{AllPermissions: []string{permission}}
		if err := h.authorizer.Authorize(principal, gate); err != nil {
			writeError(ctx, http.StatusForbidden, "forbidden", rbacFailure)
			ctx.Abort()
			return
		}

		// 4.- Parse pagination for GET requests so downstream handlers can reuse it.
		if ctx.Request.Method == http.MethodGet {
			pagination, err := parsePagination(ctx)
			if err != nil {
				writeError(ctx, http.StatusBadRequest, "invalid pagination", map[string]interface{}{"pagination": err.Error()})
				ctx.Abort()
				return
			}
			ctx.Set(paginationContextKey, pagination)
		}

		ctx.Next()
	}
}

// 1.- paginationFromContext retrieves parsed pagination or falls back to defaults.
func paginationFromContext(ctx *gin.Context) Pagination {
	// 2.- Attempt to read pagination stored by the RBAC middleware.
	value, exists := ctx.Get(paginationContextKey)
	if !exists {
		return Pagination{Page: 1, PerPage: 20}
	}
	pagination, ok := value.(Pagination)
	if !ok {
		return Pagination{Page: 1, PerPage: 20}
	}
	return pagination
}

// 1.- CreateUser persists a new user and returns the stored representation.
func (h Handler) CreateUser(ctx *gin.Context) {
	// 2.- Bind the incoming JSON payload to the user structure.
	var payload User
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}

	// 3.- Normalize and validate the payload prior to persistence.
	normalizeUser(&payload)
	if errs := validateRequired(map[string]string{"email": payload.Email}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}

	// 4.- Delegate to the user service and handle unexpected failures.
	created, err := h.users.Create(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not create user", nil)
		return
	}

	writeSuccess(ctx, http.StatusCreated, created, map[string]any{})
}

// 1.- UpdateUser updates an existing user based on the path parameter.
func (h Handler) UpdateUser(ctx *gin.Context) {
	// 2.- Capture the resource identifier from the request path.
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}

	// 3.- Bind and normalize the request payload.
	var payload User
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizeUser(&payload)
	if errs := validateRequired(map[string]string{"email": payload.Email}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}

	// 4.- Persist the update through the user service.
	updated, err := h.users.Update(ctx.Request.Context(), id, payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not update user", nil)
		return
	}

	writeSuccess(ctx, http.StatusOK, updated, map[string]any{})
}

// 1.- DeleteUser removes a user using the provided identifier.
func (h Handler) DeleteUser(ctx *gin.Context) {
	// 2.- Ensure the identifier is present before delegating to the service.
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}

	if err := h.users.Delete(ctx.Request.Context(), id); err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not delete user", nil)
		return
	}

	writeSuccess(ctx, http.StatusOK, gin.H{}, map[string]any{})
}

// 1.- ListUsers retrieves users using pagination metadata captured by middleware.
func (h Handler) ListUsers(ctx *gin.Context) {
	// 2.- Reuse pagination parsed by the RBAC middleware for deterministic output.
	pagination := paginationFromContext(ctx)

	// 3.- Fetch the paginated records from the user service.
	users, total, err := h.users.List(ctx.Request.Context(), pagination)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not list users", nil)
		return
	}

	meta := map[string]any{"page": pagination.Page, "per_page": pagination.PerPage, "total": total}
	writeSuccess(ctx, http.StatusOK, users, meta)
}

// 1.- CreateRole persists a new role entity.
func (h Handler) CreateRole(ctx *gin.Context) {
	var payload Role
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizeRole(&payload)
	if errs := validateRequired(map[string]string{"name": payload.Name}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}
	created, err := h.roles.Create(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not create role", nil)
		return
	}
	writeSuccess(ctx, http.StatusCreated, created, map[string]any{})
}

// 1.- UpdateRole updates an existing role.
func (h Handler) UpdateRole(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}
	var payload Role
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizeRole(&payload)
	if errs := validateRequired(map[string]string{"name": payload.Name}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}
	updated, err := h.roles.Update(ctx.Request.Context(), id, payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not update role", nil)
		return
	}
	writeSuccess(ctx, http.StatusOK, updated, map[string]any{})
}

// 1.- DeleteRole removes a role entity.
func (h Handler) DeleteRole(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}
	if err := h.roles.Delete(ctx.Request.Context(), id); err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not delete role", nil)
		return
	}
	writeSuccess(ctx, http.StatusOK, gin.H{}, map[string]any{})
}

// 1.- ListRoles returns paginated roles.
func (h Handler) ListRoles(ctx *gin.Context) {
	pagination := paginationFromContext(ctx)
	roles, total, err := h.roles.List(ctx.Request.Context(), pagination)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not list roles", nil)
		return
	}
	meta := map[string]any{"page": pagination.Page, "per_page": pagination.PerPage, "total": total}
	writeSuccess(ctx, http.StatusOK, roles, meta)
}

// 1.- CreatePermission persists a new permission entity.
func (h Handler) CreatePermission(ctx *gin.Context) {
	var payload Permission
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizePermission(&payload)
	if errs := validateRequired(map[string]string{"name": payload.Name}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}
	created, err := h.permissions.Create(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not create permission", nil)
		return
	}
	writeSuccess(ctx, http.StatusCreated, created, map[string]any{})
}

// 1.- UpdatePermission updates an existing permission entity.
func (h Handler) UpdatePermission(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}
	var payload Permission
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizePermission(&payload)
	if errs := validateRequired(map[string]string{"name": payload.Name}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}
	updated, err := h.permissions.Update(ctx.Request.Context(), id, payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not update permission", nil)
		return
	}
	writeSuccess(ctx, http.StatusOK, updated, map[string]any{})
}

// 1.- DeletePermission removes an existing permission entity.
func (h Handler) DeletePermission(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}
	if err := h.permissions.Delete(ctx.Request.Context(), id); err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not delete permission", nil)
		return
	}
	writeSuccess(ctx, http.StatusOK, gin.H{}, map[string]any{})
}

// 1.- ListPermissions returns paginated permissions.
func (h Handler) ListPermissions(ctx *gin.Context) {
	pagination := paginationFromContext(ctx)
	permissions, total, err := h.permissions.List(ctx.Request.Context(), pagination)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not list permissions", nil)
		return
	}
	meta := map[string]any{"page": pagination.Page, "per_page": pagination.PerPage, "total": total}
	writeSuccess(ctx, http.StatusOK, permissions, meta)
}

// 1.- CreateTeam persists a new team entity.
func (h Handler) CreateTeam(ctx *gin.Context) {
	var payload Team
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizeTeam(&payload)
	if errs := validateRequired(map[string]string{"name": payload.Name}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}
	created, err := h.teams.Create(ctx.Request.Context(), payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not create team", nil)
		return
	}
	writeSuccess(ctx, http.StatusCreated, created, map[string]any{})
}

// 1.- UpdateTeam updates an existing team entity.
func (h Handler) UpdateTeam(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}
	var payload Team
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		writeError(ctx, http.StatusBadRequest, "invalid request payload", nil)
		return
	}
	normalizeTeam(&payload)
	if errs := validateRequired(map[string]string{"name": payload.Name}); len(errs) > 0 {
		writeError(ctx, http.StatusBadRequest, "validation failed", errs)
		return
	}
	updated, err := h.teams.Update(ctx.Request.Context(), id, payload)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not update team", nil)
		return
	}
	writeSuccess(ctx, http.StatusOK, updated, map[string]any{})
}

// 1.- DeleteTeam removes an existing team entity.
func (h Handler) DeleteTeam(ctx *gin.Context) {
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" {
		writeError(ctx, http.StatusBadRequest, "validation failed", map[string]interface{}{"id": "is required"})
		return
	}
	if err := h.teams.Delete(ctx.Request.Context(), id); err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not delete team", nil)
		return
	}
	writeSuccess(ctx, http.StatusOK, gin.H{}, map[string]any{})
}

// 1.- ListTeams returns paginated teams.
func (h Handler) ListTeams(ctx *gin.Context) {
	pagination := paginationFromContext(ctx)
	teams, total, err := h.teams.List(ctx.Request.Context(), pagination)
	if err != nil {
		writeError(ctx, http.StatusInternalServerError, "could not list teams", nil)
		return
	}
	meta := map[string]any{"page": pagination.Page, "per_page": pagination.PerPage, "total": total}
	writeSuccess(ctx, http.StatusOK, teams, meta)
}
