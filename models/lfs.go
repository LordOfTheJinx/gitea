// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"errors"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// LFSMetaObject stores metadata for LFS tracked files.
type LFSMetaObject struct {
	ID           int64 `xorm:"pk autoincr"`
	lfs.Pointer  `xorm:"extends"`
	RepositoryID int64              `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Existing     bool               `xorm:"-"`
	CreatedUnix  timeutil.TimeStamp `xorm:"created"`
}

func init() {
	db.RegisterModel(new(LFSMetaObject))
}

// LFSTokenResponse defines the JSON structure in which the JWT token is stored.
// This structure is fetched via SSH and passed by the Git LFS client to the server
// endpoint for authorization.
type LFSTokenResponse struct {
	Header map[string]string `json:"header"`
	Href   string            `json:"href"`
}

// ErrLFSObjectNotExist is returned from lfs models functions in order
// to differentiate between database and missing object errors.
var ErrLFSObjectNotExist = errors.New("LFS Meta object does not exist")

// NewLFSMetaObject stores a given populated LFSMetaObject structure in the database
// if it is not already present.
func NewLFSMetaObject(m *LFSMetaObject) (*LFSMetaObject, error) {
	var err error

	ctx, committer, err := db.TxContext()
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	has, err := db.GetByBean(ctx, m)
	if err != nil {
		return nil, err
	}

	if has {
		m.Existing = true
		return m, committer.Commit()
	}

	if err = db.Insert(ctx, m); err != nil {
		return nil, err
	}

	return m, committer.Commit()
}

// GetLFSMetaObjectByOid selects a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error. If the error is nil,
// the returned pointer is a valid LFSMetaObject.
func (repo *Repository) GetLFSMetaObjectByOid(oid string) (*LFSMetaObject, error) {
	if len(oid) == 0 {
		return nil, ErrLFSObjectNotExist
	}

	m := &LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}, RepositoryID: repo.ID}
	has, err := db.GetEngine(db.DefaultContext).Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSObjectNotExist
	}
	return m, nil
}

// RemoveLFSMetaObjectByOid removes a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error.
func (repo *Repository) RemoveLFSMetaObjectByOid(oid string) (int64, error) {
	if len(oid) == 0 {
		return 0, ErrLFSObjectNotExist
	}

	ctx, committer, err := db.TxContext()
	if err != nil {
		return 0, err
	}
	defer committer.Close()

	m := &LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}, RepositoryID: repo.ID}
	if _, err := db.DeleteByBean(ctx, m); err != nil {
		return -1, err
	}

	count, err := db.CountByBean(ctx, &LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
	if err != nil {
		return count, err
	}

	return count, committer.Commit()
}

// GetLFSMetaObjects returns all LFSMetaObjects associated with a repository
func (repo *Repository) GetLFSMetaObjects(page, pageSize int) ([]*LFSMetaObject, error) {
	sess := db.GetEngine(db.DefaultContext)

	if page >= 0 && pageSize > 0 {
		start := 0
		if page > 0 {
			start = (page - 1) * pageSize
		}
		sess.Limit(pageSize, start)
	}
	lfsObjects := make([]*LFSMetaObject, 0, pageSize)
	return lfsObjects, sess.Find(&lfsObjects, &LFSMetaObject{RepositoryID: repo.ID})
}

// CountLFSMetaObjects returns a count of all LFSMetaObjects associated with a repository
func (repo *Repository) CountLFSMetaObjects() (int64, error) {
	return db.GetEngine(db.DefaultContext).Count(&LFSMetaObject{RepositoryID: repo.ID})
}

// LFSObjectAccessible checks if a provided Oid is accessible to the user
func LFSObjectAccessible(user *User, oid string) (bool, error) {
	if user.IsAdmin {
		count, err := db.GetEngine(db.DefaultContext).Count(&LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
		return count > 0, err
	}
	cond := accessibleRepositoryCondition(user)
	count, err := db.GetEngine(db.DefaultContext).Where(cond).Join("INNER", "repository", "`lfs_meta_object`.repository_id = `repository`.id").Count(&LFSMetaObject{Pointer: lfs.Pointer{Oid: oid}})
	return count > 0, err
}

// LFSAutoAssociate auto associates accessible LFSMetaObjects
func LFSAutoAssociate(metas []*LFSMetaObject, user *User, repoID int64) error {
	ctx, committer, err := db.TxContext()
	if err != nil {
		return err
	}
	defer committer.Close()

	sess := db.GetEngine(ctx)

	oids := make([]interface{}, len(metas))
	oidMap := make(map[string]*LFSMetaObject, len(metas))
	for i, meta := range metas {
		oids[i] = meta.Oid
		oidMap[meta.Oid] = meta
	}

	cond := builder.NewCond()
	if !user.IsAdmin {
		cond = builder.In("`lfs_meta_object`.repository_id",
			builder.Select("`repository`.id").From("repository").Where(accessibleRepositoryCondition(user)))
	}
	newMetas := make([]*LFSMetaObject, 0, len(metas))
	if err := sess.Cols("oid").Where(cond).In("oid", oids...).GroupBy("oid").Find(&newMetas); err != nil {
		return err
	}
	for i := range newMetas {
		newMetas[i].Size = oidMap[newMetas[i].Oid].Size
		newMetas[i].RepositoryID = repoID
	}
	if err := db.Insert(ctx, newMetas); err != nil {
		return err
	}

	return committer.Commit()
}

// IterateLFS iterates lfs object
func IterateLFS(f func(mo *LFSMetaObject) error) error {
	var start int
	const batchSize = 100
	var e = db.GetEngine(db.DefaultContext)
	for {
		mos := make([]*LFSMetaObject, 0, batchSize)
		if err := e.Limit(batchSize, start).Find(&mos); err != nil {
			return err
		}
		if len(mos) == 0 {
			return nil
		}
		start += len(mos)

		for _, mo := range mos {
			if err := f(mo); err != nil {
				return err
			}
		}
	}
}
