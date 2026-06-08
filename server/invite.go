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

type Invite struct {
	Id             uint64
	InviteCode     string
	IsUsed         bool
	ExpirationDate uint64
	RoleId         uint64
}

func NewInvite() *Invite {
	return &Invite{}
}

func (invite *Invite) FromMap(m map[string]any) *Invite {
	switch v := m["id"].(type) {
	case float64:
		invite.Id = uint64(v)
	}

	switch v := m["inviteCode"].(type) {
	case string:
		invite.InviteCode = v
	}

	switch v := m["isUsed"].(type) {
	case bool:
		invite.IsUsed = v
	}

	switch v := m["expirationDate"].(type) {
	case float64:
		invite.ExpirationDate = uint64(v)
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			invite.ExpirationDate = uint64(t.Unix())
		}
	}

	switch v := m["roleId"].(type) {
	case float64:
		invite.RoleId = uint64(v)
	}

	return invite
}

func (invite *Invite) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"id":         invite.Id,
		"inviteCode": invite.InviteCode,
		"isUsed":     invite.IsUsed,
		"roleId":     invite.RoleId,
	}

	if invite.ExpirationDate > 0 {
		m["expirationDate"] = invite.ExpirationDate
	}

	return json.Marshal(m)
}

type Invites struct {
	List  []*Invite
	mutex sync.Mutex
}

func NewInvites() *Invites {
	return &Invites{
		List:  []*Invite{},
		mutex: sync.Mutex{},
	}
}

func (invites *Invites) Add(invite *Invite) (*Invites, bool) {
	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	added := true

	for _, i := range invites.List {
		if i.InviteCode == invite.InviteCode {
			i.IsUsed = invite.IsUsed
			i.ExpirationDate = invite.ExpirationDate
			i.RoleId = invite.RoleId
			added = false
		}
	}

	if added {
		invites.List = append(invites.List, invite)
	}

	return invites, added
}

func (invites *Invites) FromMap(f []any) *Invites {
	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	invites.List = []*Invite{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			invite := NewInvite().FromMap(m)
			invites.List = append(invites.List, invite)
		}
	}

	return invites
}

func (invites *Invites) GetInvite(inviteCode string) (invite *Invite, ok bool) {
	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	for _, invite := range invites.List {
		if invite.InviteCode == inviteCode {
			return invite, true
		}
	}

	return nil, false
}

func (invites *Invites) GetInviteById(id uint64) (invite *Invite, ok bool) {
	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	for _, invite := range invites.List {
		if invite.Id == id {
			return invite, true
		}
	}

	return nil, false
}

func (invites *Invites) Read(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
	)

	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	invites.List = []*Invite{}

	formatError := errorFormatter("invites", "read")

	query = `SELECT "inviteId", "inviteCode", "isUsed", "expirationDate", "roleId" FROM "invites"`
	if rows, err = db.Sql.Query(query); err != nil {
		return formatError(err, query)
	}

	for rows.Next() {
		invite := NewInvite()

		if err = rows.Scan(&invite.Id, &invite.InviteCode, &invite.IsUsed, &invite.ExpirationDate, &invite.RoleId); err != nil {
			break
		}

		invites.List = append(invites.List, invite)
	}

	rows.Close()

	if err != nil {
		return formatError(err, "")
	}

	sort.Slice(invites.List, func(i int, j int) bool {
		return strings.ToLower(invites.List[i].InviteCode) < strings.ToLower(invites.List[j].InviteCode)
	})

	return nil
}

func (invites *Invites) Remove(invite *Invite) (*Invites, bool) {
	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	removed := false

	for i, r := range invites.List {
		if r.Id > 0 && invite.Id > 0 && r.Id == invite.Id {
			invites.List = append(invites.List[:i], invites.List[i+1:]...)
			removed = true
			break
		}

		if r.InviteCode == invite.InviteCode {
			invites.List = append(invites.List[:i], invites.List[i+1:]...)
			removed = true
			break
		}
	}

	return invites, removed
}

func (invites *Invites) Write(db *Database) error {
	var (
		err       error
		inviteIds = []uint64{}
		query     string
		rows      *sql.Rows
		tx        *sql.Tx
	)

	invites.mutex.Lock()
	defer invites.mutex.Unlock()

	formatError := errorFormatter("invites", "write")

	if tx, err = db.Sql.Begin(); err != nil {
		return formatError(err, "")
	}

	query = `SELECT "inviteId" FROM "invites"`
	if rows, err = tx.Query(query); err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	for rows.Next() {
		var inviteId uint64
		if err = rows.Scan(&inviteId); err != nil {
			break
		}
		remove := true
		for _, invite := range invites.List {
			if invite.Id == 0 || invite.Id == inviteId {
				remove = false
				break
			}
		}
		if remove {
			inviteIds = append(inviteIds, inviteId)
		}
	}

	rows.Close()

	if err != nil {
		tx.Rollback()
		return formatError(err, "")
	}

	if len(inviteIds) > 0 {
		if b, err := json.Marshal(inviteIds); err == nil {
			in := strings.ReplaceAll(strings.ReplaceAll(string(b), "[", "("), "]", ")")
			query = fmt.Sprintf(`DELETE FROM "invites" WHERE "inviteId" IN %s`, in)
			if _, err = tx.Exec(query); err != nil {
				tx.Rollback()
				return formatError(err, query)
			}
		}
	}

	for _, invite := range invites.List {
		var count uint

		if invite.Id > 0 {
			query = fmt.Sprintf(`SELECT COUNT(*) FROM "invites" WHERE "inviteId" = %d`, invite.Id)
			if err = tx.QueryRow(query).Scan(&count); err != nil {
				break
			}
		}

		if count == 0 {
			query = fmt.Sprintf(`INSERT INTO "invites" ("inviteCode", "isUsed", "expirationDate", "roleId") VALUES ('%s', %t, %d, %d)`, escapeQuotes(invite.InviteCode), invite.IsUsed, invite.ExpirationDate, invite.RoleId)
			if _, err = tx.Exec(query); err != nil {
				break
			}

		} else {
			query = fmt.Sprintf(`UPDATE "invites" SET "inviteCode" = '%s', "isUsed" = %t, "expirationDate" = %d, "roleId" = %d WHERE "inviteId" = %d`, escapeQuotes(invite.InviteCode), invite.IsUsed, invite.ExpirationDate, invite.RoleId, invite.Id)
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
