// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issue

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestIssue_AddLabels(t *testing.T) {
	var tests = []struct {
		issueID  int64
		labelIDs []int64
		doerID   int64
	}{
		{1, []int64{1, 2}, 2}, // non-pull-request
		{1, []int64{}, 2},     // non-pull-request, empty
		{2, []int64{1, 2}, 2}, // pull-request
		{2, []int64{}, 1},     // pull-request, empty
	}
	for _, test := range tests {
		assert.NoError(t, unittest.PrepareTestDatabase())
		issue := unittest.AssertExistsAndLoadBean(t, &models.Issue{ID: test.issueID}).(*models.Issue)
		labels := make([]*models.Label, len(test.labelIDs))
		for i, labelID := range test.labelIDs {
			labels[i] = unittest.AssertExistsAndLoadBean(t, &models.Label{ID: labelID}).(*models.Label)
		}
		doer := unittest.AssertExistsAndLoadBean(t, &models.User{ID: test.doerID}).(*models.User)
		assert.NoError(t, AddLabels(issue, doer, labels))
		for _, labelID := range test.labelIDs {
			unittest.AssertExistsAndLoadBean(t, &models.IssueLabel{IssueID: test.issueID, LabelID: labelID})
		}
	}
}

func TestIssue_AddLabel(t *testing.T) {
	var tests = []struct {
		issueID int64
		labelID int64
		doerID  int64
	}{
		{1, 2, 2}, // non-pull-request, not-already-added label
		{1, 1, 2}, // non-pull-request, already-added label
		{2, 2, 2}, // pull-request, not-already-added label
		{2, 1, 2}, // pull-request, already-added label
	}
	for _, test := range tests {
		assert.NoError(t, unittest.PrepareTestDatabase())
		issue := unittest.AssertExistsAndLoadBean(t, &models.Issue{ID: test.issueID}).(*models.Issue)
		label := unittest.AssertExistsAndLoadBean(t, &models.Label{ID: test.labelID}).(*models.Label)
		doer := unittest.AssertExistsAndLoadBean(t, &models.User{ID: test.doerID}).(*models.User)
		assert.NoError(t, AddLabel(issue, doer, label))
		unittest.AssertExistsAndLoadBean(t, &models.IssueLabel{IssueID: test.issueID, LabelID: test.labelID})
	}
}
