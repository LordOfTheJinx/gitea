// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/private"

	"github.com/stretchr/testify/assert"
)

func TestAPIPrivateNoServ(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		key, user, err := private.ServNoCommand(ctx, 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), user.ID)
		assert.Equal(t, "user2", user.Name)
		assert.Equal(t, int64(1), key.ID)
		assert.Equal(t, "user2@localhost", key.Name)

		deployKey, err := models.AddDeployKey(1, "test-deploy", "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGXEEzWmm1dxb+57RoK5KVCL0w2eNv9cqJX2AGGVlkFsVDhOXHzsadS3LTK4VlEbbrDMJdoti9yM8vclA8IeRacAAAAEc3NoOg== nocomment", false)
		assert.NoError(t, err)

		key, user, err = private.ServNoCommand(ctx, deployKey.KeyID)
		assert.NoError(t, err)
		assert.Empty(t, user)
		assert.Equal(t, deployKey.KeyID, key.ID)
		assert.Equal(t, "test-deploy", key.Name)
	})
}

func TestAPIPrivateServ(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Can push to a repo we own
		results, err := private.ServCommand(ctx, 1, "user2", "repo1", models.AccessModeWrite, "git-upload-pack", "")
		assert.NoError(t, err)
		assert.False(t, results.IsWiki)
		assert.False(t, results.IsDeployKey)
		assert.Equal(t, int64(1), results.KeyID)
		assert.Equal(t, "user2@localhost", results.KeyName)
		assert.Equal(t, "user2", results.UserName)
		assert.Equal(t, int64(2), results.UserID)
		assert.Equal(t, "user2", results.OwnerName)
		assert.Equal(t, "repo1", results.RepoName)
		assert.Equal(t, int64(1), results.RepoID)

		// Cannot push to a private repo we're not associated with
		results, err = private.ServCommand(ctx, 1, "user15", "big_test_private_1", models.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Cannot pull from a private repo we're not associated with
		results, err = private.ServCommand(ctx, 1, "user15", "big_test_private_1", models.AccessModeRead, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Can pull from a public repo we're not associated with
		results, err = private.ServCommand(ctx, 1, "user15", "big_test_public_1", models.AccessModeRead, "git-upload-pack", "")
		assert.NoError(t, err)
		assert.False(t, results.IsWiki)
		assert.False(t, results.IsDeployKey)
		assert.Equal(t, int64(1), results.KeyID)
		assert.Equal(t, "user2@localhost", results.KeyName)
		assert.Equal(t, "user2", results.UserName)
		assert.Equal(t, int64(2), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_public_1", results.RepoName)
		assert.Equal(t, int64(17), results.RepoID)

		// Cannot push to a public repo we're not associated with
		results, err = private.ServCommand(ctx, 1, "user15", "big_test_public_1", models.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Add reading deploy key
		deployKey, err := models.AddDeployKey(19, "test-deploy", "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGXEEzWmm1dxb+57RoK5KVCL0w2eNv9cqJX2AGGVlkFsVDhOXHzsadS3LTK4VlEbbrDMJdoti9yM8vclA8IeRacAAAAEc3NoOg== nocomment", true)
		assert.NoError(t, err)

		// Can pull from repo we're a deploy key for
		results, err = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_1", models.AccessModeRead, "git-upload-pack", "")
		assert.NoError(t, err)
		assert.False(t, results.IsWiki)
		assert.True(t, results.IsDeployKey)
		assert.Equal(t, deployKey.KeyID, results.KeyID)
		assert.Equal(t, "test-deploy", results.KeyName)
		assert.Equal(t, "user15", results.UserName)
		assert.Equal(t, int64(15), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_private_1", results.RepoName)
		assert.Equal(t, int64(19), results.RepoID)

		// Cannot push to a private repo with reading key
		results, err = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_1", models.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Cannot pull from a private repo we're not associated with
		results, err = private.ServCommand(ctx, deployKey.ID, "user15", "big_test_private_2", models.AccessModeRead, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Cannot pull from a public repo we're not associated with
		results, err = private.ServCommand(ctx, deployKey.ID, "user15", "big_test_public_1", models.AccessModeRead, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Add writing deploy key
		deployKey, err = models.AddDeployKey(20, "test-deploy", "sk-ecdsa-sha2-nistp256@openssh.com AAAAInNrLWVjZHNhLXNoYTItbmlzdHAyNTZAb3BlbnNzaC5jb20AAAAIbmlzdHAyNTYAAABBBGXEEzWmm1dxb+57RoK5KVCL0w2eNv9cqJX2AGGVlkFsVDhOXHzsadS3LTK4VlEbbrDMJdoti9yM8vclA8IeRacAAAAEc3NoOg== nocomment", false)
		assert.NoError(t, err)

		// Cannot push to a private repo with reading key
		results, err = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_1", models.AccessModeWrite, "git-upload-pack", "")
		assert.Error(t, err)
		assert.Empty(t, results)

		// Can pull from repo we're a writing deploy key for
		results, err = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_2", models.AccessModeRead, "git-upload-pack", "")
		assert.NoError(t, err)
		assert.False(t, results.IsWiki)
		assert.True(t, results.IsDeployKey)
		assert.Equal(t, deployKey.KeyID, results.KeyID)
		assert.Equal(t, "test-deploy", results.KeyName)
		assert.Equal(t, "user15", results.UserName)
		assert.Equal(t, int64(15), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_private_2", results.RepoName)
		assert.Equal(t, int64(20), results.RepoID)

		// Can push to repo we're a writing deploy key for
		results, err = private.ServCommand(ctx, deployKey.KeyID, "user15", "big_test_private_2", models.AccessModeWrite, "git-upload-pack", "")
		assert.NoError(t, err)
		assert.False(t, results.IsWiki)
		assert.True(t, results.IsDeployKey)
		assert.Equal(t, deployKey.KeyID, results.KeyID)
		assert.Equal(t, "test-deploy", results.KeyName)
		assert.Equal(t, "user15", results.UserName)
		assert.Equal(t, int64(15), results.UserID)
		assert.Equal(t, "user15", results.OwnerName)
		assert.Equal(t, "big_test_private_2", results.RepoName)
		assert.Equal(t, int64(20), results.RepoID)

	})

}
