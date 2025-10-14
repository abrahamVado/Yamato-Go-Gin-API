package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/authorization"
	"github.com/example/Yamato-Go-Gin-API/internal/http/admin"
)

// 1.- testAuthorizer allows scenarios to toggle authorization outcomes per request.
type testAuthorizer struct {
	// 2.- allow toggles whether calls should succeed based on the principal payload.
	allow bool
}

// 1.- Authorize applies the success flag and validates the permission payload.
func (a *testAuthorizer) Authorize(principal internalauth.Principal, gate authorization.Gate) error {
	required := ""
	if len(gate.AllPermissions) > 0 {
		required = gate.AllPermissions[0]
	}
	for _, permission := range principal.Permissions {
		if permission == required {
			if a.allow {
				return nil
			}
			break
		}
	}
	return authorization.ErrForbidden
}

// 1.- testUserService captures interactions performed by the handler under test.
type testUserService struct {
	created admin.User
	updated admin.User
	deleted string
}

func (s *testUserService) Create(_ context.Context, payload admin.User) (admin.User, error) {
	s.created = payload
	payload.ID = "user-1"
	return payload, nil
}

func (s *testUserService) Update(_ context.Context, id string, payload admin.User) (admin.User, error) {
	s.updated = payload
	s.updated.ID = id
	return s.updated, nil
}

func (s *testUserService) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func (s *testUserService) List(_ context.Context, _ admin.Pagination) ([]admin.User, int, error) {
	return nil, 0, nil
}

// 1.- testRoleService captures role interactions invoked by the handler.
type testRoleService struct {
	created admin.Role
	updated admin.Role
	deleted string
}

func (s *testRoleService) Create(_ context.Context, payload admin.Role) (admin.Role, error) {
	s.created = payload
	payload.ID = "role-1"
	return payload, nil
}

func (s *testRoleService) Update(_ context.Context, id string, payload admin.Role) (admin.Role, error) {
	s.updated = payload
	s.updated.ID = id
	return s.updated, nil
}

func (s *testRoleService) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func (s *testRoleService) List(_ context.Context, _ admin.Pagination) ([]admin.Role, int, error) {
	return nil, 0, nil
}

// 1.- testPermissionService captures permission operations performed by handlers.
type testPermissionService struct {
	created admin.Permission
	updated admin.Permission
	deleted string
}

func (s *testPermissionService) Create(_ context.Context, payload admin.Permission) (admin.Permission, error) {
	s.created = payload
	payload.ID = "permission-1"
	return payload, nil
}

func (s *testPermissionService) Update(_ context.Context, id string, payload admin.Permission) (admin.Permission, error) {
	s.updated = payload
	s.updated.ID = id
	return s.updated, nil
}

func (s *testPermissionService) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func (s *testPermissionService) List(_ context.Context, _ admin.Pagination) ([]admin.Permission, int, error) {
	return nil, 0, nil
}

// 1.- testTeamService captures team operations performed by handlers.
type testTeamService struct {
	created admin.Team
	updated admin.Team
	deleted string
}

func (s *testTeamService) Create(_ context.Context, payload admin.Team) (admin.Team, error) {
	s.created = payload
	payload.ID = "team-1"
	return payload, nil
}

func (s *testTeamService) Update(_ context.Context, id string, payload admin.Team) (admin.Team, error) {
	s.updated = payload
	s.updated.ID = id
	return s.updated, nil
}

func (s *testTeamService) Delete(_ context.Context, id string) error {
	s.deleted = id
	return nil
}

func (s *testTeamService) List(_ context.Context, _ admin.Pagination) ([]admin.Team, int, error) {
	return nil, 0, nil
}

// 1.- applyPrincipal middleware injects a preconfigured principal for the test.
func applyPrincipal(principal internalauth.Principal) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		internalauth.SetPrincipal(ctx, principal)
		ctx.Next()
	}
}

// 1.- executeRequest sends an HTTP request to the configured Gin engine.
func executeRequest(router *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// 1.- parseEnvelope decodes JSON envelopes for response assertions.
func parseEnvelope(t *testing.T, body *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(body.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return envelope
}

// 1.- newHandler composes the admin handler with test doubles.
func newHandler(authorizer admin.Authorizer, users *testUserService, roles *testRoleService, permissions *testPermissionService, teams *testTeamService) admin.Handler {
	return admin.NewHandler(authorizer, users, roles, permissions, teams)
}

func TestHandler_UserFlows(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		method     string
		path       string
		route      string
		permission string
		body       any
		principal  internalauth.Principal
		expectCode int
		assert     func(t *testing.T, svc *testUserService, resp *httptest.ResponseRecorder)
	}{
		{
			name:       "create user succeeds",
			method:     http.MethodPost,
			path:       "/admin/users",
			permission: admin.PermissionManageUsers,
			body:       admin.User{Email: "user@example.com", Roles: []string{"admin"}},
			principal:  internalauth.Principal{Subject: "admin", Permissions: []string{admin.PermissionManageUsers}},
			expectCode: http.StatusCreated,
			assert: func(t *testing.T, svc *testUserService, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.created.Email != "user@example.com" {
					t.Fatalf("expected service to receive normalized email, got %q", svc.created.Email)
				}
				envelope := parseEnvelope(t, resp)
				data := envelope["data"].(map[string]any)
				if data["id"].(string) == "" {
					t.Fatalf("expected created user id to be present")
				}
			},
		},
		{
			name:       "authorization failure prevents create",
			method:     http.MethodPost,
			path:       "/admin/users",
			permission: admin.PermissionManageUsers,
			body:       admin.User{Email: "user@example.com"},
			principal:  internalauth.Principal{Subject: "member", Permissions: []string{}},
			expectCode: http.StatusForbidden,
			assert: func(t *testing.T, svc *testUserService, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.created.Email != "" {
					t.Fatalf("expected service not to be invoked")
				}
				envelope := parseEnvelope(t, resp)
				if envelope["message"].(string) != "forbidden" {
					t.Fatalf("unexpected message: %v", envelope["message"])
				}
			},
		},
		{
			name:       "update user succeeds",
			method:     http.MethodPut,
			path:       "/admin/users/user-1",
			route:      "/admin/users/:id",
			permission: admin.PermissionManageUsers,
			body:       admin.User{Email: "edited@example.com", Roles: []string{"viewer"}},
			principal:  internalauth.Principal{Subject: "admin", Permissions: []string{admin.PermissionManageUsers}},
			expectCode: http.StatusOK,
			assert: func(t *testing.T, svc *testUserService, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.updated.Email != "edited@example.com" {
					t.Fatalf("expected service to receive payload, got %q", svc.updated.Email)
				}
				envelope := parseEnvelope(t, resp)
				data := envelope["data"].(map[string]any)
				if data["email"].(string) != "edited@example.com" {
					t.Fatalf("unexpected email in response: %v", data["email"])
				}
			},
		},
		{
			name:       "delete user succeeds",
			method:     http.MethodDelete,
			path:       "/admin/users/user-1",
			route:      "/admin/users/:id",
			permission: admin.PermissionManageUsers,
			body:       nil,
			principal:  internalauth.Principal{Subject: "admin", Permissions: []string{admin.PermissionManageUsers}},
			expectCode: http.StatusOK,
			assert: func(t *testing.T, svc *testUserService, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.deleted != "user-1" {
					t.Fatalf("expected delete to receive id, got %q", svc.deleted)
				}
				envelope := parseEnvelope(t, resp)
				if len(envelope["data"].(map[string]any)) != 0 {
					t.Fatalf("expected empty data envelope")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			router := gin.New()

			authorizer := &testAuthorizer{allow: tc.principal.Subject == "admin"}
			users := &testUserService{}
			handler := newHandler(authorizer, users, &testRoleService{}, &testPermissionService{}, &testTeamService{})

			router.Use(applyPrincipal(tc.principal))
			var bodyBytes []byte
			if tc.body != nil {
				bodyBytes, _ = json.Marshal(tc.body)
			}
			route := tc.route
			if route == "" {
				route = tc.path
			}
			switch tc.method {
			case http.MethodPost:
				router.POST(route, handler.RBAC(tc.permission), handler.CreateUser)
			case http.MethodPut:
				router.PUT(route, handler.RBAC(tc.permission), handler.UpdateUser)
			case http.MethodDelete:
				router.DELETE(route, handler.RBAC(tc.permission), handler.DeleteUser)
			}

			resp := executeRequest(router, tc.method, tc.path, bodyBytes)
			if resp.Code != tc.expectCode {
				t.Fatalf("expected status %d, got %d", tc.expectCode, resp.Code)
			}
			tc.assert(t, users, resp)
		})
	}
}

func TestHandler_RolePermissionTeamFlows(t *testing.T) {
	t.Parallel()

	type scenario struct {
		name       string
		method     string
		path       string
		route      string
		permission string
		body       any
		principal  internalauth.Principal
		expectCode int
		assert     func(t *testing.T, services servicesBundle, resp *httptest.ResponseRecorder)
	}

	cases := []scenario{
		{
			name:       "create role succeeds",
			method:     http.MethodPost,
			path:       "/admin/roles",
			permission: admin.PermissionManageRoles,
			body:       admin.Role{Name: "platform", Permissions: []string{"deploy"}},
			principal:  internalauth.Principal{Subject: "admin", Permissions: []string{admin.PermissionManageRoles}},
			expectCode: http.StatusCreated,
			assert: func(t *testing.T, svc servicesBundle, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.roles.created.Name != "platform" {
					t.Fatalf("expected role to be created")
				}
				envelope := parseEnvelope(t, resp)
				data := envelope["data"].(map[string]any)
				if data["id"].(string) == "" {
					t.Fatalf("expected role id in response")
				}
			},
		},
		{
			name:       "update permission forbidden",
			method:     http.MethodPut,
			path:       "/admin/permissions/perm-1",
			route:      "/admin/permissions/:id",
			permission: admin.PermissionManagePermissions,
			body:       admin.Permission{Name: "pipelines.write"},
			principal:  internalauth.Principal{Subject: "member", Permissions: []string{}},
			expectCode: http.StatusForbidden,
			assert: func(t *testing.T, svc servicesBundle, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.permissions.updated.Name != "" {
					t.Fatalf("expected update not to be invoked")
				}
				envelope := parseEnvelope(t, resp)
				if envelope["message"].(string) != "forbidden" {
					t.Fatalf("unexpected message: %v", envelope["message"])
				}
			},
		},
		{
			name:       "delete team succeeds",
			method:     http.MethodDelete,
			path:       "/admin/teams/team-1",
			route:      "/admin/teams/:id",
			permission: admin.PermissionManageTeams,
			body:       nil,
			principal:  internalauth.Principal{Subject: "admin", Permissions: []string{admin.PermissionManageTeams}},
			expectCode: http.StatusOK,
			assert: func(t *testing.T, svc servicesBundle, resp *httptest.ResponseRecorder) {
				t.Helper()
				if svc.teams.deleted != "team-1" {
					t.Fatalf("expected team deletion to receive id")
				}
				envelope := parseEnvelope(t, resp)
				if len(envelope["data"].(map[string]any)) != 0 {
					t.Fatalf("expected empty payload on delete")
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			router := gin.New()

			services := servicesBundle{
				roles:       &testRoleService{},
				permissions: &testPermissionService{},
				teams:       &testTeamService{},
			}
			handler := newHandler(&testAuthorizer{allow: tc.principal.Subject == "admin"}, &testUserService{}, services.roles, services.permissions, services.teams)

			router.Use(applyPrincipal(tc.principal))

			var bodyBytes []byte
			if tc.body != nil {
				bodyBytes, _ = json.Marshal(tc.body)
			}

			route := tc.route
			if route == "" {
				route = tc.path
			}
			switch tc.method {
			case http.MethodPost:
				router.POST(route, handler.RBAC(tc.permission), handler.CreateRole)
			case http.MethodPut:
				router.PUT(route, handler.RBAC(tc.permission), handler.UpdatePermission)
			case http.MethodDelete:
				router.DELETE(route, handler.RBAC(tc.permission), handler.DeleteTeam)
			}

			resp := executeRequest(router, tc.method, tc.path, bodyBytes)
			if resp.Code != tc.expectCode {
				t.Fatalf("expected status %d, got %d", tc.expectCode, resp.Code)
			}
			tc.assert(t, services, resp)
		})
	}
}

type servicesBundle struct {
	roles       *testRoleService
	permissions *testPermissionService
	teams       *testTeamService
}
