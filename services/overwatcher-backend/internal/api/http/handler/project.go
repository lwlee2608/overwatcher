package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/project"
)

type ProjectHandler struct {
	svc *project.Service
}

func NewProjectHandler(svc *project.Service) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) List(c *gin.Context) {
	if userID := c.Query("user_id"); userID != "" {
		entries, err := h.svc.ListProjectsByUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, listResp(entries))
		return
	}
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	entries, err := h.svc.ListProjectsForUser(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, listResp(entries))
}

// requireAccess resolves the caller's role on a project. Returns role + ok;
// when not ok, the handler has already written an error response.
func (h *ProjectHandler) requireAccess(c *gin.Context, projectID string) (string, bool) {
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return "", false
	}
	role, err := h.svc.Access(c.Request.Context(), callerID, projectID)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		case errors.Is(err, project.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return "", false
	}
	return role, true
}

func (h *ProjectHandler) requireOwner(c *gin.Context, projectID string) bool {
	role, ok := h.requireAccess(c, projectID)
	if !ok {
		return false
	}
	if role != project.RoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "owner only"})
		return false
	}
	return true
}

func (h *ProjectHandler) Get(c *gin.Context) {
	id := c.Param("id")
	role, ok := h.requireAccess(c, id)
	if !ok {
		return
	}
	p, err := h.svc.GetProject(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	services, err := h.svc.ListComposeServicesByProject(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := projectToDTO(*p)
	resp.Role = role
	resp.Services = composeServicesToDTO(services)
	c.JSON(http.StatusOK, resp)
}

func (h *ProjectHandler) Create(c *gin.Context) {
	var req dto.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p, err := h.svc.CreateProject(c.Request.Context(), project.CreateProjectParams{
		UserID:      req.UserID,
		Name:        req.Name,
		Description: req.Description,
		ComposeFile: req.ComposeFile,
		Environment: req.Environment,
		Enabled:     enabled,
	})
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "project name already used by this user"})
			return
		}
		if isForeignKeyViolation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, projectToDTO(*p))
}

func (h *ProjectHandler) Update(c *gin.Context) {
	if !h.requireOwner(c, c.Param("id")) {
		return
	}
	var req dto.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p, err := h.svc.UpdateProject(c.Request.Context(), c.Param("id"), project.UpdateProjectParams{
		Name:        req.Name,
		Description: req.Description,
		ComposeFile: req.ComposeFile,
		Environment: req.Environment,
		Enabled:     enabled,
	})
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "project name already used by this user"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projectToDTO(*p))
}

func (h *ProjectHandler) Delete(c *gin.Context) {
	if !h.requireOwner(c, c.Param("id")) {
		return
	}
	if err := h.svc.DeleteProject(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, project.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProjectHandler) ListServices(c *gin.Context) {
	if _, ok := h.requireAccess(c, c.Param("id")); !ok {
		return
	}
	services, err := h.svc.ListComposeServicesByProject(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.ComposeServiceListResponse{Services: composeServicesToDTO(services)})
}

func (h *ProjectHandler) CreateService(c *gin.Context) {
	if _, ok := h.requireAccess(c, c.Param("id")); !ok {
		return
	}
	var req dto.CreateComposeServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	svc, err := h.svc.CreateComposeService(c.Request.Context(), project.CreateComposeServiceParams{
		ProjectID:     c.Param("id"),
		Name:          req.Name,
		Repo:          req.Repo,
		RootDirectory: req.RootDirectory,
		Branch:        req.Branch,
		Image:         req.Image,
		Tag:           req.Tag,
		Workflow:      req.Workflow,
		Position:      req.Position,
	})
	if err != nil {
		if errors.Is(err, project.ErrInvalidRepo) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "service name already exists in this project"})
			return
		}
		if isForeignKeyViolation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, composeServiceToDTO(*svc))
}

func (h *ProjectHandler) ReplaceServices(c *gin.Context) {
	if _, ok := h.requireAccess(c, c.Param("id")); !ok {
		return
	}
	var req dto.ReplaceComposeServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	params := make([]project.CreateComposeServiceParams, len(req.Services))
	for i, s := range req.Services {
		params[i] = project.CreateComposeServiceParams{
			Name:          s.Name,
			Repo:          s.Repo,
			RootDirectory: s.RootDirectory,
			Branch:        s.Branch,
			Image:         s.Image,
			Tag:           s.Tag,
			Workflow:      s.Workflow,
			Position:      i,
		}
	}
	out, err := h.svc.ReplaceComposeServices(c.Request.Context(), c.Param("id"), params)
	if err != nil {
		if errors.Is(err, project.ErrInvalidRepo) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isForeignKeyViolation(err) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.ComposeServiceListResponse{Services: composeServicesToDTO(out)})
}

func (h *ProjectHandler) DeleteService(c *gin.Context) {
	if _, ok := h.requireAccess(c, c.Param("id")); !ok {
		return
	}
	if err := h.svc.DeleteComposeService(c.Request.Context(), c.Param("serviceID")); err != nil {
		if errors.Is(err, project.ErrServiceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProjectHandler) ListMembers(c *gin.Context) {
	if _, ok := h.requireAccess(c, c.Param("id")); !ok {
		return
	}
	members, err := h.svc.ListMembers(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := dto.ProjectMemberListResponse{Members: make([]dto.ProjectMemberResponse, len(members))}
	for i, m := range members {
		resp.Members[i] = memberToDTO(m)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *ProjectHandler) AddMember(c *gin.Context) {
	if !h.requireOwner(c, c.Param("id")) {
		return
	}
	var req dto.AddProjectMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	callerID, _ := middleware.UserID(c)
	m, err := h.svc.AddMemberByEmail(c.Request.Context(), c.Param("id"), req.Email, callerID)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "no user with that email"})
		case errors.Is(err, project.ErrAlreadyMember):
			c.JSON(http.StatusConflict, gin.H{"error": "user is already a member"})
		case errors.Is(err, project.ErrCannotAddOwner):
			c.JSON(http.StatusConflict, gin.H{"error": "user is the project owner"})
		case errors.Is(err, project.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, memberToDTO(*m))
}

func (h *ProjectHandler) RemoveMember(c *gin.Context) {
	projectID := c.Param("id")
	targetUserID := c.Param("userID")
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	role, err := h.svc.Access(c.Request.Context(), callerID, projectID)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		case errors.Is(err, project.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	// Owner can remove anyone; member can only remove themselves.
	if role != project.RoleOwner && callerID != targetUserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "owner only"})
		return
	}
	if err := h.svc.RemoveMember(c.Request.Context(), projectID, targetUserID); err != nil {
		if errors.Is(err, project.ErrMemberNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func memberToDTO(m project.Member) dto.ProjectMemberResponse {
	return dto.ProjectMemberResponse{
		UserID:    m.UserID,
		UserEmail: m.UserEmail,
		UserName:  m.UserName,
		Role:      m.Role,
		AddedBy:   m.AddedBy,
		CreatedAt: m.CreatedAt,
	}
}

func listResp(entries []project.Project) dto.ProjectListResponse {
	resp := dto.ProjectListResponse{Projects: make([]dto.ProjectResponse, len(entries))}
	for i, e := range entries {
		resp.Projects[i] = projectToDTO(e)
	}
	return resp
}

func projectToDTO(p project.Project) dto.ProjectResponse {
	return dto.ProjectResponse{
		ID:          p.ID,
		UserID:      p.UserID,
		UserEmail:   p.UserEmail,
		Name:        p.Name,
		Description: p.Description,
		ComposeFile: p.ComposeFile,
		Environment: p.Environment,
		Enabled:     p.Enabled,
		Role:        p.Role,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func composeServiceToDTO(s project.ComposeService) dto.ComposeServiceResponse {
	return dto.ComposeServiceResponse{
		ID:            s.ID,
		ProjectID:     s.ProjectID,
		Name:          s.Name,
		Repo:          s.Repo,
		RootDirectory: s.RootDirectory,
		Branch:        s.Branch,
		Image:         s.Image,
		Tag:           s.Tag,
		Workflow:      s.Workflow,
		Position:      s.Position,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
	}
}

func composeServicesToDTO(services []project.ComposeService) []dto.ComposeServiceResponse {
	out := make([]dto.ComposeServiceResponse, len(services))
	for i, s := range services {
		out[i] = composeServiceToDTO(s)
	}
	return out
}

func isForeignKeyViolation(err error) bool {
	// pgconn: 23503 foreign_key_violation
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23503"
	}
	return false
}
