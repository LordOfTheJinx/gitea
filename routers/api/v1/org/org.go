// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package org

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/user"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/org"
)

func listUserOrgs(ctx *context.APIContext, u *models.User) {
	listOptions := utils.GetListOptions(ctx)
	showPrivate := ctx.IsSigned && (ctx.User.IsAdmin || ctx.User.ID == u.ID)

	var opts = models.FindOrgOptions{
		ListOptions:    listOptions,
		UserID:         u.ID,
		IncludePrivate: showPrivate,
	}
	orgs, err := models.FindOrgs(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindOrgs", err)
		return
	}
	maxResults, err := models.CountOrgs(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CountOrgs", err)
		return
	}

	apiOrgs := make([]*api.Organization, len(orgs))
	for i := range orgs {
		apiOrgs[i] = convert.ToOrganization(orgs[i])
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(int64(maxResults))
	ctx.JSON(http.StatusOK, &apiOrgs)
}

// ListMyOrgs list all my orgs
func ListMyOrgs(ctx *context.APIContext) {
	// swagger:operation GET /user/orgs organization orgListCurrentUserOrgs
	// ---
	// summary: List the current user's organizations
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"

	listUserOrgs(ctx, ctx.User)
}

// ListUserOrgs list user's orgs
func ListUserOrgs(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/orgs organization orgListUserOrgs
	// ---
	// summary: List a user's organizations
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"

	u := user.GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserOrgs(ctx, u)
}

// GetUserOrgsPermissions get user permissions in organization
func GetUserOrgsPermissions(ctx *context.APIContext) {
	// swagger:operation GET /users/{username}/orgs/{org}/permissions organization orgGetUserPermissions
	// ---
	// summary: Get user permissions in organization
	// produces:
	// - application/json
	// parameters:
	// - name: username
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// - name: org
	//   in: path
	//   description: name of the organization
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationPermissions"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	var u *models.User
	if u = user.GetUserByParams(ctx); u == nil {
		return
	}

	var o *models.User
	if o = user.GetUserByParamsName(ctx, ":org"); o == nil {
		return
	}

	op := api.OrganizationPermissions{}

	if !models.HasOrgOrUserVisible(o, u) {
		ctx.NotFound("HasOrgOrUserVisible", nil)
		return
	}

	org := models.OrgFromUser(o)
	authorizeLevel, err := org.GetOrgUserMaxAuthorizeLevel(u.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetOrgUserAuthorizeLevel", err)
		return
	}

	if authorizeLevel > models.AccessModeNone {
		op.CanRead = true
	}
	if authorizeLevel > models.AccessModeRead {
		op.CanWrite = true
	}
	if authorizeLevel > models.AccessModeWrite {
		op.IsAdmin = true
	}
	if authorizeLevel > models.AccessModeAdmin {
		op.IsOwner = true
	}

	op.CanCreateRepository, err = org.CanCreateOrgRepo(u.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CanCreateOrgRepo", err)
		return
	}

	ctx.JSON(http.StatusOK, op)
}

// GetAll return list of all public organizations
func GetAll(ctx *context.APIContext) {
	// swagger:operation Get /orgs organization orgGetAll
	// ---
	// summary: Get list of organizations
	// produces:
	// - application/json
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/OrganizationList"

	vMode := []api.VisibleType{api.VisibleTypePublic}
	if ctx.IsSigned {
		vMode = append(vMode, api.VisibleTypeLimited)
		if ctx.User.IsAdmin {
			vMode = append(vMode, api.VisibleTypePrivate)
		}
	}

	listOptions := utils.GetListOptions(ctx)

	publicOrgs, maxResults, err := models.SearchUsers(&models.SearchUserOptions{
		Actor:       ctx.User,
		ListOptions: listOptions,
		Type:        models.UserTypeOrganization,
		OrderBy:     models.SearchOrderByAlphabetically,
		Visible:     vMode,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SearchOrganizations", err)
		return
	}
	orgs := make([]*api.Organization, len(publicOrgs))
	for i := range publicOrgs {
		orgs[i] = convert.ToOrganization(models.OrgFromUser(publicOrgs[i]))
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.SetTotalCountHeader(maxResults)
	ctx.JSON(http.StatusOK, &orgs)
}

// Create api for create organization
func Create(ctx *context.APIContext) {
	// swagger:operation POST /orgs organization orgCreate
	// ---
	// summary: Create an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: organization
	//   in: body
	//   required: true
	//   schema: { "$ref": "#/definitions/CreateOrgOption" }
	// responses:
	//   "201":
	//     "$ref": "#/responses/Organization"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.CreateOrgOption)
	if !ctx.User.CanCreateOrganization() {
		ctx.Error(http.StatusForbidden, "Create organization not allowed", nil)
		return
	}

	visibility := api.VisibleTypePublic
	if form.Visibility != "" {
		visibility = api.VisibilityModes[form.Visibility]
	}

	org := &models.Organization{
		Name:                      form.UserName,
		FullName:                  form.FullName,
		Description:               form.Description,
		Website:                   form.Website,
		Location:                  form.Location,
		IsActive:                  true,
		Type:                      models.UserTypeOrganization,
		Visibility:                visibility,
		RepoAdminChangeTeamAccess: form.RepoAdminChangeTeamAccess,
	}
	if err := models.CreateOrganization(org, ctx.User); err != nil {
		if models.IsErrUserAlreadyExist(err) ||
			models.IsErrNameReserved(err) ||
			models.IsErrNameCharsNotAllowed(err) ||
			models.IsErrNamePatternNotAllowed(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "CreateOrganization", err)
		}
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToOrganization(org))
}

// Get get an organization
func Get(ctx *context.APIContext) {
	// swagger:operation GET /orgs/{org} organization orgGet
	// ---
	// summary: Get an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"

	if !models.HasOrgOrUserVisible(ctx.Org.Organization.AsUser(), ctx.User) {
		ctx.NotFound("HasOrgOrUserVisible", nil)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToOrganization(ctx.Org.Organization))
}

// Edit change an organization's information
func Edit(ctx *context.APIContext) {
	// swagger:operation PATCH /orgs/{org} organization orgEdit
	// ---
	// summary: Edit an organization
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: name of the organization to edit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/EditOrgOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Organization"
	form := web.GetForm(ctx).(*api.EditOrgOption)
	org := ctx.Org.Organization
	org.FullName = form.FullName
	org.Description = form.Description
	org.Website = form.Website
	org.Location = form.Location
	if form.Visibility != "" {
		org.Visibility = api.VisibilityModes[form.Visibility]
	}
	if form.RepoAdminChangeTeamAccess != nil {
		org.RepoAdminChangeTeamAccess = *form.RepoAdminChangeTeamAccess
	}
	if err := models.UpdateUserCols(db.DefaultContext, org.AsUser(),
		"full_name", "description", "website", "location",
		"visibility", "repo_admin_change_team_access",
	); err != nil {
		ctx.Error(http.StatusInternalServerError, "EditOrganization", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToOrganization(org))
}

//Delete an organization
func Delete(ctx *context.APIContext) {
	// swagger:operation DELETE /orgs/{org} organization orgDelete
	// ---
	// summary: Delete an organization
	// produces:
	// - application/json
	// parameters:
	// - name: org
	//   in: path
	//   description: organization that is to be deleted
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	if err := org.DeleteOrganization(ctx.Org.Organization); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteOrganization", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
