// Copyright (C) 2019-2026 Chrystian Huot <chrystian.huot@saubeo.solutions>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>
//
// WebSocket API Access Policy:
// This WebSocket API is reserved exclusively for Saubeo Solutions and its native applications.
// Unauthorized access is strictly prohibited.
// See API_ACCESS_POLICY.md for full terms.

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

type MobileToken struct {
	TokenString string
	UserId      uint64
	CreatedAt   uint64
}

func NewMobileToken() *MobileToken {
	return &MobileToken{}
}

func (mobileToken *MobileToken) FromMap(m map[string]any) *MobileToken {
	switch v := m["tokenString"].(type) {
	case string:
		mobileToken.TokenString = v
	}

	switch v := m["userId"].(type) {
	case float64:
		mobileToken.UserId = uint64(v)
	}

	switch v := m["createdAt"].(type) {
	case float64:
		mobileToken.CreatedAt = uint64(v)
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			mobileToken.CreatedAt = uint64(t.Unix())
		}
	}

	return mobileToken
}

func (mobileToken *MobileToken) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"tokenString": mobileToken.TokenString,
		"userId":      mobileToken.UserId,
	}

	if mobileToken.CreatedAt > 0 {
		m["createdAt"] = time.Unix(int64(mobileToken.CreatedAt), 0)
	}

	return json.Marshal(m)
}

type MobileTokens struct {
	List  []*MobileToken
	mutex sync.Mutex
}

func NewMobileTokens() *MobileTokens {
	return &MobileTokens{
		List:  []*MobileToken{},
		mutex: sync.Mutex{},
	}
}

func (mobileTokens *MobileTokens) Add(mobileToken *MobileToken) (*MobileTokens, bool) {
	mobileTokens.mutex.Lock()
	defer mobileTokens.mutex.Unlock()

	added := true

	for _, t := range mobileTokens.List {
		if t.TokenString == mobileToken.TokenString {
			t.UserId = mobileToken.UserId
			t.CreatedAt = mobileToken.CreatedAt
			added = false
		}
	}

	if added {
		mobileTokens.List = append(mobileTokens.List, mobileToken)
	}

	return mobileTokens, added
}

func (mobileTokens *MobileTokens) FromMap(f []any) *MobileTokens {
	mobileTokens.mutex.Lock()
	defer mobileTokens.mutex.Unlock()

	mobileTokens.List = []*MobileToken{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			mobileToken := NewMobileToken().FromMap(m)
			mobileTokens.List = append(mobileTokens.List, mobileToken)
		}
	}

	return mobileTokens
}

func (mobileTokens *MobileTokens) GetMobileToken(tokenString string) (mobileToken *MobileToken, ok bool) {
	mobileTokens.mutex.Lock()
	defer mobileTokens.mutex.Unlock()

	for _, mobileToken := range mobileTokens.List {
		if mobileToken.TokenString == tokenString {
			return mobileToken, true
		}
	}

	return nil, false
}

func (mobileTokens *MobileTokens) Read(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
	)

	mobileTokens.mutex.Lock()
	defer mobileTokens.mutex.Unlock()

	mobileTokens.List = []*MobileToken{}

	formatError := errorFormatter("mobiletokens", "read")

	query = `SELECT "tokenString", "userId", "createdAt" FROM "mobileTokens"`
	if rows, err = db.Sql.Query(query); err != nil {
		return formatError(err, query)
	}

	for rows.Next() {
		mobileToken := NewMobileToken()

		if err = rows.Scan(&mobileToken.TokenString, &mobileToken.UserId, &mobileToken.CreatedAt); err != nil {
			break
		}

		mobileTokens.List = append(mobileTokens.List, mobileToken)
	}

	rows.Close()

	if err != nil {
		return formatError(err, "")
	}

	sort.Slice(mobileTokens.List, func(i int, j int) bool {
		return mobileTokens.List[i].CreatedAt < mobileTokens.List[j].CreatedAt
	})

	return nil
}

func (mobileTokens *MobileTokens) Remove(mobileToken *MobileToken) (*MobileTokens, bool) {
	mobileTokens.mutex.Lock()
	defer mobileTokens.mutex.Unlock()

	removed := false

	for i, t := range mobileTokens.List {
		if t.TokenString == mobileToken.TokenString {
			mobileTokens.List = append(mobileTokens.List[:i], mobileTokens.List[i+1:]...)
			removed = true
			break
		}
	}

	return mobileTokens, removed
}

func (mobileTokens *MobileTokens) Write(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
		tx    *sql.Tx
	)

	mobileTokens.mutex.Lock()
	defer mobileTokens.mutex.Unlock()

	formatError := errorFormatter("mobiletokens", "write")

	if tx, err = db.Sql.Begin(); err != nil {
		return formatError(err, "")
	}

	query = `SELECT "tokenString" FROM "mobileTokens"`
	if rows, err = tx.Query(query); err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	for rows.Next() {
		var tokenString string
		if err = rows.Scan(&tokenString); err != nil {
			break
		}

		remove := true
		for _, mobileToken := range mobileTokens.List {
			if mobileToken.TokenString == tokenString {
				remove = false
				break
			}
		}

		if remove {
			query = fmt.Sprintf(`DELETE FROM "mobileTokens" WHERE "tokenString" = '%s'`, escapeQuotes(tokenString))
			if _, err = tx.Exec(query); err != nil {
				break
			}
		}
	}

	rows.Close()

	if err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	for _, mobileToken := range mobileTokens.List {
		var count uint

		query = fmt.Sprintf(`SELECT COUNT(*) FROM "mobileTokens" WHERE "tokenString" = '%s'`, escapeQuotes(mobileToken.TokenString))
		if err = tx.QueryRow(query).Scan(&count); err != nil {
			break
		}

		if count == 0 {
			query = fmt.Sprintf(`INSERT INTO "mobileTokens" ("tokenString", "userId", "createdAt") VALUES ('%s', %d, %d)`, escapeQuotes(mobileToken.TokenString), mobileToken.UserId, mobileToken.CreatedAt)
			if _, err = tx.Exec(query); err != nil {
				break
			}

		} else {
			query = fmt.Sprintf(`UPDATE "mobileTokens" SET "userId" = %d, "createdAt" = %d WHERE "tokenString" = '%s'`, mobileToken.UserId, mobileToken.CreatedAt, escapeQuotes(mobileToken.TokenString))
			if _, err = tx.Exec(query); err != nil {
				break
			}
		}
	}

	if err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return formatError(err, "")
	}

	return nil
}
