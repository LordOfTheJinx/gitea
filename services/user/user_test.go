// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m, filepath.Join("..", ".."))
}

func TestDeleteUser(t *testing.T) {
	test := func(userID int64) {
		assert.NoError(t, unittest.PrepareTestDatabase())
		user := unittest.AssertExistsAndLoadBean(t, &models.User{ID: userID}).(*models.User)

		ownedRepos := make([]*models.Repository, 0, 10)
		assert.NoError(t, db.GetEngine(db.DefaultContext).Find(&ownedRepos, &models.Repository{OwnerID: userID}))
		if len(ownedRepos) > 0 {
			err := DeleteUser(user)
			assert.Error(t, err)
			assert.True(t, models.IsErrUserOwnRepos(err))
			return
		}

		orgUsers := make([]*models.OrgUser, 0, 10)
		assert.NoError(t, db.GetEngine(db.DefaultContext).Find(&orgUsers, &models.OrgUser{UID: userID}))
		for _, orgUser := range orgUsers {
			if err := models.RemoveOrgUser(orgUser.OrgID, orgUser.UID); err != nil {
				assert.True(t, models.IsErrLastOrgOwner(err))
				return
			}
		}
		assert.NoError(t, DeleteUser(user))
		unittest.AssertNotExistsBean(t, &models.User{ID: userID})
		unittest.CheckConsistencyFor(t, &models.User{}, &models.Repository{})
	}
	test(2)
	test(4)
	test(8)
	test(11)

	org := unittest.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)
	assert.Error(t, DeleteUser(org))
}

func TestCreateUser(t *testing.T) {
	user := &models.User{
		Name:               "GiteaBot",
		Email:              "GiteaBot@gitea.io",
		Passwd:             ";p['////..-++']",
		IsAdmin:            false,
		Theme:              setting.UI.DefaultTheme,
		MustChangePassword: false,
	}

	assert.NoError(t, models.CreateUser(user))

	assert.NoError(t, DeleteUser(user))
}

func TestCreateUser_Issue5882(t *testing.T) {
	// Init settings
	_ = setting.Admin

	passwd := ".//.;1;;//.,-=_"

	tt := []struct {
		user               *models.User
		disableOrgCreation bool
	}{
		{&models.User{Name: "GiteaBot", Email: "GiteaBot@gitea.io", Passwd: passwd, MustChangePassword: false}, false},
		{&models.User{Name: "GiteaBot2", Email: "GiteaBot2@gitea.io", Passwd: passwd, MustChangePassword: false}, true},
	}

	setting.Service.DefaultAllowCreateOrganization = true

	for _, v := range tt {
		setting.Admin.DisableRegularOrgCreation = v.disableOrgCreation

		assert.NoError(t, models.CreateUser(v.user))

		u, err := models.GetUserByEmail(v.user.Email)
		assert.NoError(t, err)

		assert.Equal(t, !u.AllowCreateOrganization, v.disableOrgCreation)

		assert.NoError(t, DeleteUser(v.user))
	}
}
