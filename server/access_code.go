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
	"strings"
	"sync"
	"time"
)

type AccessCode struct {
	Id        uint64
	UserId    uint64
	Code      string
	Label     string
	CreatedAt uint64
}

func NewAccessCode() *AccessCode {
	return &AccessCode{}
}

func (accessCode *AccessCode) FromMap(m map[string]any) *AccessCode {
	switch v := m["id"].(type) {
	case float64:
		accessCode.Id = uint64(v)
	}

	switch v := m["userId"].(type) {
	case float64:
		accessCode.UserId = uint64(v)
	}

	switch v := m["code"].(type) {
	case string:
		accessCode.Code = v
	}

	switch v := m["label"].(type) {
	case string:
		accessCode.Label = v
	}

	switch v := m["createdAt"].(type) {
	case float64:
		accessCode.CreatedAt = uint64(v)
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			accessCode.CreatedAt = uint64(t.Unix())
		}
	}

	return accessCode
}

func (accessCode *AccessCode) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"id":        accessCode.Id,
		"userId":    accessCode.UserId,
		"code":      accessCode.Code,
		"label":     accessCode.Label,
		"createdAt": time.Unix(int64(accessCode.CreatedAt), 0).UTC(),
	}

	return json.Marshal(m)
}

type AccessCodes struct {
	List  []*AccessCode
	mutex sync.Mutex
}

func NewAccessCodes() *AccessCodes {
	return &AccessCodes{List: []*AccessCode{}, mutex: sync.Mutex{}}
}

func (accessCodes *AccessCodes) Add(accessCode *AccessCode) (*AccessCodes, bool) {
	accessCodes.mutex.Lock()
	defer accessCodes.mutex.Unlock()

	added := true

	for _, c := range accessCodes.List {
		if c.Id > 0 && accessCode.Id > 0 && c.Id == accessCode.Id {
			c.UserId = accessCode.UserId
			c.Code = accessCode.Code
			c.Label = accessCode.Label
			c.CreatedAt = accessCode.CreatedAt
			added = false
			break
		}

		if c.Code == accessCode.Code {
			c.UserId = accessCode.UserId
			c.Label = accessCode.Label
			if accessCode.CreatedAt > 0 {
				c.CreatedAt = accessCode.CreatedAt
			}
			added = false
			break
		}
	}

	if added {
		accessCodes.List = append(accessCodes.List, accessCode)
	}

	return accessCodes, added
}

func (accessCodes *AccessCodes) GetByCode(code string) (*AccessCode, bool) {
	accessCodes.mutex.Lock()
	defer accessCodes.mutex.Unlock()

	for _, accessCode := range accessCodes.List {
		if accessCode.Code == code {
			return accessCode, true
		}
	}

	return nil, false
}

func (accessCodes *AccessCodes) ListByUser(userId uint64) []*AccessCode {
	accessCodes.mutex.Lock()
	defer accessCodes.mutex.Unlock()

	codes := []*AccessCode{}
	for _, accessCode := range accessCodes.List {
		if accessCode.UserId == userId {
			copyCode := *accessCode
			codes = append(codes, &copyCode)
		}
	}

	sort.Slice(codes, func(i int, j int) bool {
		if codes[i].CreatedAt == codes[j].CreatedAt {
			return codes[i].Id < codes[j].Id
		}
		return codes[i].CreatedAt > codes[j].CreatedAt
	})

	return codes
}

func (accessCodes *AccessCodes) RemoveByIdAndUser(id uint64, userId uint64) bool {
	accessCodes.mutex.Lock()
	defer accessCodes.mutex.Unlock()

	for i, accessCode := range accessCodes.List {
		if accessCode.Id == id && accessCode.UserId == userId {
			accessCodes.List = append(accessCodes.List[:i], accessCodes.List[i+1:]...)
			return true
		}
	}

	return false
}

func (accessCodes *AccessCodes) Read(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
	)

	accessCodes.mutex.Lock()
	defer accessCodes.mutex.Unlock()

	accessCodes.List = []*AccessCode{}

	formatError := errorFormatter("accesscodes", "read")

	query = `SELECT "accessCodeId", "userId", "code", "label", "createdAt" FROM "accessCodes"`
	if rows, err = db.Sql.Query(query); err != nil {
		return formatError(err, query)
	}

	for rows.Next() {
		accessCode := NewAccessCode()
		if err = rows.Scan(&accessCode.Id, &accessCode.UserId, &accessCode.Code, &accessCode.Label, &accessCode.CreatedAt); err != nil {
			break
		}
		accessCodes.List = append(accessCodes.List, accessCode)
	}

	rows.Close()

	if err != nil {
		return formatError(err, "")
	}

	sort.Slice(accessCodes.List, func(i int, j int) bool {
		if accessCodes.List[i].CreatedAt == accessCodes.List[j].CreatedAt {
			return accessCodes.List[i].Id < accessCodes.List[j].Id
		}
		return accessCodes.List[i].CreatedAt > accessCodes.List[j].CreatedAt
	})

	return nil
}

func (accessCodes *AccessCodes) Write(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
		tx    *sql.Tx
	)

	accessCodes.mutex.Lock()
	defer accessCodes.mutex.Unlock()

	formatError := errorFormatter("accesscodes", "write")

	if tx, err = db.Sql.Begin(); err != nil {
		return formatError(err, "")
	}

	query = `SELECT "accessCodeId" FROM "accessCodes"`
	if rows, err = tx.Query(query); err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			break
		}

		remove := true
		for _, accessCode := range accessCodes.List {
			if accessCode.Id == 0 || accessCode.Id == id {
				remove = false
				break
			}
		}

		if remove {
			query = fmt.Sprintf(`DELETE FROM "accessCodes" WHERE "accessCodeId" = %d`, id)
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

	for _, accessCode := range accessCodes.List {
		var count uint

		if accessCode.CreatedAt == 0 {
			accessCode.CreatedAt = uint64(time.Now().Unix())
		}

		if accessCode.Id > 0 {
			query = fmt.Sprintf(`SELECT COUNT(*) FROM "accessCodes" WHERE "accessCodeId" = %d`, accessCode.Id)
			if err = tx.QueryRow(query).Scan(&count); err != nil {
				break
			}
		}

		if count == 0 {
			query = fmt.Sprintf(`INSERT INTO "accessCodes" ("userId", "code", "label", "createdAt") VALUES (%d, '%s', '%s', %d)`, accessCode.UserId, escapeQuotes(accessCode.Code), escapeQuotes(accessCode.Label), accessCode.CreatedAt)
			if _, err = tx.Exec(query); err != nil {
				break
			}

			query = fmt.Sprintf(`SELECT "accessCodeId" FROM "accessCodes" WHERE "code" = '%s'`, escapeQuotes(accessCode.Code))
			if err = tx.QueryRow(query).Scan(&accessCode.Id); err != nil {
				break
			}
		} else {
			query = fmt.Sprintf(`UPDATE "accessCodes" SET "userId" = %d, "code" = '%s', "label" = '%s', "createdAt" = %d WHERE "accessCodeId" = %d`, accessCode.UserId, escapeQuotes(accessCode.Code), escapeQuotes(accessCode.Label), accessCode.CreatedAt, accessCode.Id)
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

func normalizeAccessCodeLabel(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return "Personal Access Code"
	}
	return trimmed
}
