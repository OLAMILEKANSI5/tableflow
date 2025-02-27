package web

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/guregu/null"
	"github.com/samber/lo"
	"net/http"
	"strings"
	"tableflow/go/pkg/db"
	"tableflow/go/pkg/model"
	"tableflow/go/pkg/tf"
	"tableflow/go/pkg/types"
	"tableflow/go/pkg/util"
)

type ImporterCreateRequest struct {
	Name        string `json:"name" example:"Test Importer"`
	WorkspaceID string `json:"workspace_id" example:"b2079476-261a-41fe-8019-46eb51c537f7"`
}

type ImporterEditRequest struct {
	Name           *string   `json:"name" example:"Test Importer"`
	AllowedDomains *[]string `json:"allowed_domains" example:"example.com"`
	WebhookURL     *string   `json:"webhook_url" example:"https://example.com/webhook"`
}

// createImporter
//
//	@Summary		Create importer
//	@Description	Create an importer
//	@Tags			Importer
//	@Success		200	{object}	model.Importer
//	@Failure		400	{object}	types.Res
//	@Router			/admin/v1/importer [post]
//	@Param			body	body	ImporterCreateRequest	true	"Request body"
func createImporter(c *gin.Context, getWorkspaceUser func(*gin.Context, string) (string, error)) {
	req := ImporterCreateRequest{}
	if err := c.BindJSON(&req); err != nil {
		tf.Log.Warnw("Could not bind JSON", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	userID, err := getWorkspaceUser(c, req.WorkspaceID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.Res{Err: err.Error()})
		return
	}
	user := model.User{ID: model.ParseID(userID)}
	if len(req.Name) == 0 {
		req.Name = "My Importer"
	}
	importer := model.Importer{
		ID:          model.NewID(),
		WorkspaceID: model.ParseID(req.WorkspaceID),
		Name:        req.Name,
		CreatedBy:   user.ID,
		UpdatedBy:   user.ID,
	}
	err = tf.DB.Create(&importer).Error
	if err != nil {
		tf.Log.Errorw("Could not create importer", "error", err, "workspace_id", req.WorkspaceID)
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	// Right now, templates are 1:1 with importers. Create a default template to be used by the importer
	template := model.Template{
		ID:          model.NewID(),
		WorkspaceID: importer.WorkspaceID,
		ImporterID:  importer.ID,
		Name:        "Default Template",
		CreatedBy:   user.ID,
		UpdatedBy:   user.ID,
	}
	err = tf.DB.Create(&template).Error
	if err != nil {
		tf.Log.Errorw("Could not create template for importer", "error", err, "workspace_id", req.WorkspaceID)
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	c.JSON(http.StatusOK, &importer)
}

// getImporter
//
//	@Summary		Get importer
//	@Description	Get a single importer
//	@Tags			Importer
//	@Success		200	{object}	model.Importer
//	@Failure		400	{object}	types.Res
//	@Router			/admin/v1/importer/{id} [get]
//	@Param			id	path	string	true	"Importer ID"
func getImporter(c *gin.Context, getWorkspaceUser func(*gin.Context, string) (string, error)) {
	id := c.Param("id")
	if len(id) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: "No importer ID provided"})
		return
	}
	importer, err := db.GetImporterWithUsers(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	_, err = getWorkspaceUser(c, importer.WorkspaceID.String())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.Res{Err: err.Error()})
		return
	}
	c.JSON(http.StatusOK, importer)
}

// getImporters
//
//	@Summary		Get importers
//	@Description	Get a list of importers
//	@Tags			Importer
//	@Success		200	{object}	[]model.Importer
//	@Failure		400	{object}	types.Res
//	@Router			/admin/v1/importers/{workspace-id} [get]
//	@Param			workspace-id	path	string	true	"Workspace ID"
func getImporters(c *gin.Context, getWorkspaceUser func(*gin.Context, string) (string, error)) {
	workspaceID := c.Param("workspace-id")
	if len(workspaceID) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: "No workspace ID provided"})
		return
	}
	_, err := getWorkspaceUser(c, workspaceID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.Res{Err: err.Error()})
		return
	}
	importers, err := db.GetImportersWithUsers(workspaceID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	c.JSON(http.StatusOK, importers)
}

// editImporter
//
//	@Summary		Edit importer
//	@Description	Edit an importer
//	@Tags			Importer
//	@Success		200	{object}	model.Importer
//	@Failure		400	{object}	types.Res
//	@Router			/admin/v1/importer/{id} [post]
//	@Param			id		path	string				true	"Importer ID"
//	@Param			body	body	ImporterEditRequest	true	"Request body"
func editImporter(c *gin.Context, getWorkspaceUser func(*gin.Context, string) (string, error)) {
	id := c.Param("id")
	if len(id) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: "No importer ID provided"})
		return
	}
	req := ImporterEditRequest{}
	if err := c.BindJSON(&req); err != nil {
		tf.Log.Warnw("Could not bind JSON", "error", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	importer, err := db.GetImporter(id)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
		return
	}
	_, err = getWorkspaceUser(c, importer.WorkspaceID.String())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.Res{Err: err.Error()})
		return
	}

	// Change any field that exists on the request and are different
	save := false
	if req.Name != nil && *req.Name != importer.Name {
		importer.Name = *req.Name
		save = true
	}
	if req.WebhookURL != nil && *req.WebhookURL != importer.WebhookURL.String {
		if len(*req.WebhookURL) == 0 {
			importer.WebhookURL = null.NewString("", false)
		} else {
			if !util.IsValidURL(*req.WebhookURL) {
				c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: "Invalid webhook URL"})
				return
			}
			importer.WebhookURL = null.StringFromPtr(req.WebhookURL)
		}
		save = true
	}
	if req.AllowedDomains != nil && !util.EqualContents(*req.AllowedDomains, importer.AllowedDomains) {
		// Loosely validate the domains
		invalidDomains := make([]string, 0)
		for _, d := range *req.AllowedDomains {
			if !util.IsValidDomain(d) {
				invalidDomains = append(invalidDomains, d)
			}
		}
		if len(invalidDomains) != 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{
				Err: fmt.Sprintf("Invalid domain%s: %s - Domains must be in the format 'example.com' or 'www.example.com' and not complete URLs.",
					lo.Ternary(len(invalidDomains) == 1, "", "s"), strings.Join(invalidDomains, ", ")),
			})
			return
		}
		importer.AllowedDomains = *req.AllowedDomains
		save = true
	}

	if save {
		err = tf.DB.Save(importer).Error
		if err != nil {
			tf.Log.Errorw("Could not save importer", "error", err, "importer_id", importer.ID)
			c.AbortWithStatusJSON(http.StatusBadRequest, types.Res{Err: err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, importer)
}
