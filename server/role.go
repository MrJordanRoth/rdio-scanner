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
)

type roleAccessScope struct {
	allTalkgroups bool
	talkgroups    map[uint]bool
}

type Role struct {
	Id              uint64
	Name            string
	Description     string
	ConnectionLimit uint
	DelaySeconds uint
	Managers        []uint64
	Systems         []uint64
	Talkgroups      []uint64
}

func NewRole() *Role {
	return &Role{Managers: []uint64{}, Systems: []uint64{}, Talkgroups: []uint64{}}
}

func (role *Role) FromMap(m map[string]any) *Role {
	switch v := m["id"].(type) {
	case float64:
		role.Id = uint64(v)
	}

	switch v := m["name"].(type) {
	case string:
		role.Name = v
	}

	switch v := m["description"].(type) {
	case string:
		role.Description = v
	}

	switch v := m["connectionLimit"].(type) {
	case float64:
		if v > 0 {
			role.ConnectionLimit = uint(v)
		}
	}

	switch v := m["delaySeconds"].(type) {
	case float64:
		role.DelaySeconds = normalizeRoleDelaySeconds(uint(v))
	}

	role.Managers = userIdsFromAny(m["managers"])
	if len(role.Managers) == 0 {
		role.Managers = userIdsFromAny(m["managerIds"])
	}

	role.Systems = userIdsFromAny(m["systems"])
	if len(role.Systems) == 0 {
		role.Systems = userIdsFromAny(m["systemIds"])
	}

	role.Talkgroups = userIdsFromAny(m["talkgroups"])
	if len(role.Talkgroups) == 0 {
		role.Talkgroups = userIdsFromAny(m["talkgroupIds"])
	}

	return role
}

func (role *Role) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"id":   role.Id,
		"name": role.Name,
	}

	if len(role.Description) > 0 {
		m["description"] = role.Description
	}

	if role.ConnectionLimit > 0 {
		m["connectionLimit"] = role.ConnectionLimit
	}

	if role.DelaySeconds > 0 {
		m["delaySeconds"] = role.DelaySeconds
	}

	m["managers"] = role.Managers
	m["systems"] = role.Systems
	m["talkgroups"] = role.Talkgroups

	return json.Marshal(m)
}

const (
	RoleDelaySecondsMin uint = 60
	RoleDelaySecondsMax uint = 600
)

func normalizeRoleDelaySeconds(delay uint) uint {
	if delay == 0 {
		return 0
	}

	if delay < RoleDelaySecondsMin || delay > RoleDelaySecondsMax {
		return 0
	}

	return delay
}

type Roles struct {
	List  []*Role
	mutex sync.Mutex
}

func NewRoles() *Roles {
	return &Roles{
		List:  []*Role{},
		mutex: sync.Mutex{},
	}
}

func (roles *Roles) Add(role *Role) (*Roles, bool) {
	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	added := true

	for _, r := range roles.List {
		if strings.EqualFold(r.Name, role.Name) {
			r.Description = role.Description
			added = false
		}
	}

	if added {
		roles.List = append(roles.List, role)
	}

	return roles, added
}

func (roles *Roles) FromMap(f []any) *Roles {
	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	roles.List = []*Role{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			role := NewRole().FromMap(m)
			roles.List = append(roles.List, role)
		}
	}

	return roles
}

func (roles *Roles) GetRoleById(id uint64) (role *Role, ok bool) {
	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	for _, role := range roles.List {
		if role.Id == id {
			return role, true
		}
	}

	return nil, false
}

func (roles *Roles) GetRoleByName(name string) (role *Role, ok bool) {
	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	for _, role := range roles.List {
		if strings.EqualFold(role.Name, name) {
			return role, true
		}
	}

	return nil, false
}

func (roles *Roles) Read(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
	)

	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	roles.List = []*Role{}

	formatError := errorFormatter("roles", "read")

	query = `SELECT "roleId", "name", "description", "connectionLimit", "delaySeconds" FROM "roles"`
	if rows, err = db.Sql.Query(query); err != nil {
		return formatError(err, query)
	}

	for rows.Next() {
		role := NewRole()

		if err = rows.Scan(&role.Id, &role.Name, &role.Description, &role.ConnectionLimit, &role.DelaySeconds); err != nil {
			break
		}

		role.DelaySeconds = normalizeRoleDelaySeconds(role.DelaySeconds)

		if role.Managers, err = roles.readManagers(db, role.Id); err != nil {
			break
		}

		if role.Systems, err = roles.readRoleRelationIds(db, "roleSystems", "systemId", role.Id); err != nil {
			break
		}

		if role.Talkgroups, err = roles.readRoleRelationIds(db, "roleTalkgroups", "talkgroupId", role.Id); err != nil {
			break
		}

		roles.List = append(roles.List, role)
	}

	rows.Close()

	if err != nil {
		return formatError(err, "")
	}

	sort.Slice(roles.List, func(i int, j int) bool {
		return strings.ToLower(roles.List[i].Name) < strings.ToLower(roles.List[j].Name)
	})

	return nil
}

func (roles *Roles) Remove(role *Role) (*Roles, bool) {
	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	removed := false

	for i, r := range roles.List {
		if r.Id > 0 && role.Id > 0 && r.Id == role.Id {
			roles.List = append(roles.List[:i], roles.List[i+1:]...)
			removed = true
			break
		}

		if strings.EqualFold(r.Name, role.Name) {
			roles.List = append(roles.List[:i], roles.List[i+1:]...)
			removed = true
			break
		}
	}

	return roles, removed
}

func (roles *Roles) Write(db *Database) error {
	var (
		err     error
		query   string
		roleIds = []uint64{}
		rows    *sql.Rows
		tx      *sql.Tx
	)

	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	formatError := errorFormatter("roles", "write")

	if tx, err = db.Sql.Begin(); err != nil {
		return formatError(err, "")
	}

	query = `SELECT "roleId" FROM "roles"`
	if rows, err = tx.Query(query); err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	for rows.Next() {
		var roleId uint64
		if err = rows.Scan(&roleId); err != nil {
			break
		}
		remove := true
		for _, role := range roles.List {
			if role.Id == 0 || role.Id == roleId {
				remove = false
				break
			}
		}
		if remove {
			roleIds = append(roleIds, roleId)
		}
	}

	rows.Close()

	if err != nil {
		tx.Rollback()
		return formatError(err, "")
	}

	if len(roleIds) > 0 {
		if b, err := json.Marshal(roleIds); err == nil {
			in := strings.ReplaceAll(strings.ReplaceAll(string(b), "[", "("), "]", ")")
			query = fmt.Sprintf(`DELETE FROM "roles" WHERE "roleId" IN %s`, in)
			if _, err = tx.Exec(query); err != nil {
				tx.Rollback()
				return formatError(err, query)
			}
		}
	}

	for _, role := range roles.List {
		var count uint

		if role.Id > 0 {
			query = fmt.Sprintf(`SELECT COUNT(*) FROM "roles" WHERE "roleId" = %d`, role.Id)
			if err = tx.QueryRow(query).Scan(&count); err != nil {
				break
			}
		}

		if count == 0 {
			role.DelaySeconds = normalizeRoleDelaySeconds(role.DelaySeconds)
			query = fmt.Sprintf(`INSERT INTO "roles" ("name", "description", "connectionLimit", "delaySeconds") VALUES ('%s', '%s', %d, %d)`, escapeQuotes(role.Name), escapeQuotes(role.Description), role.ConnectionLimit, role.DelaySeconds)

			if db.Config.DbType == DbTypePostgresql {
				query = query + ` RETURNING "roleId"`
				if err = tx.QueryRow(query).Scan(&role.Id); err != nil {
					break
				}
			} else {
				if res, execErr := tx.Exec(query); execErr == nil {
					if id, idErr := res.LastInsertId(); idErr == nil {
						role.Id = uint64(id)
					}
				} else {
					err = execErr
					break
				}
			}

		} else {
			role.DelaySeconds = normalizeRoleDelaySeconds(role.DelaySeconds)
			query = fmt.Sprintf(`UPDATE "roles" SET "name" = '%s', "description" = '%s', "connectionLimit" = %d, "delaySeconds" = %d WHERE "roleId" = %d`, escapeQuotes(role.Name), escapeQuotes(role.Description), role.ConnectionLimit, role.DelaySeconds, role.Id)
			if _, err = tx.Exec(query); err != nil {
				break
			}
		}

		query = fmt.Sprintf(`DELETE FROM "roleManagers" WHERE "roleId" = %d`, role.Id)
		if _, err = tx.Exec(query); err != nil {
			break
		}

		added := map[uint64]bool{}
		for _, managerId := range role.Managers {
			if managerId == 0 || added[managerId] {
				continue
			}
			added[managerId] = true
			query = fmt.Sprintf(`INSERT INTO "roleManagers" ("roleId", "userId") VALUES (%d, %d)`, role.Id, managerId)
			if _, err = tx.Exec(query); err != nil {
				break
			}
		}

		if err != nil {
			break
		}

		if err = roles.syncRoleRelationTx(tx, "roleSystems", "systemId", role.Id, role.Systems); err != nil {
			break
		}

		if err = roles.syncRoleRelationTx(tx, "roleTalkgroups", "talkgroupId", role.Id, role.Talkgroups); err != nil {
			break
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

func (roles *Roles) readManagers(db *Database, roleId uint64) ([]uint64, error) {
	return roles.readRoleRelationIds(db, "roleManagers", "userId", roleId)
}

func (roles *Roles) readRoleRelationIds(db *Database, table string, column string, roleId uint64) ([]uint64, error) {
	var (
		err    error
		result = []uint64{}
		query    string
		rows     *sql.Rows
	)

	query = fmt.Sprintf(`SELECT "%s" FROM "%s" WHERE "roleId" = %d`, column, table, roleId)
	if rows, err = db.Sql.Query(query); err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uint64
		if err = rows.Scan(&id); err != nil {
			return result, err
		}
		result = append(result, id)
	}

	return result, nil
}

func (roles *Roles) syncRoleRelationTx(tx *sql.Tx, table string, column string, roleId uint64, ids []uint64) error {
	query := fmt.Sprintf(`DELETE FROM "%s" WHERE "roleId" = %d`, table, roleId)
	if _, err := tx.Exec(query); err != nil {
		return err
	}

	added := map[uint64]bool{}
	for _, id := range ids {
		if id == 0 || added[id] {
			continue
		}
		added[id] = true
		query = fmt.Sprintf(`INSERT INTO "%s" ("roleId", "%s") VALUES (%d, %d)`, table, column, roleId, id)
		if _, err := tx.Exec(query); err != nil {
			return err
		}
	}

	return nil
}

func (roles *Roles) BuildAccessFromUserRoles(user *User, systems *Systems) (*Access, uint) {
	roles.mutex.Lock()
	defer roles.mutex.Unlock()

	if user == nil {
		return nil, 0
	}

	roleIds := map[uint64]bool{}
	for _, roleId := range user.Groups {
		if roleId > 0 {
			roleIds[roleId] = true
		}
	}

	if len(roleIds) == 0 {
		return &Access{Ident: user.Username, Systems: []any{}}, 0
	}

	maxConnectionLimit := uint(0)
	accessBySystemRef := map[uint]*roleAccessScope{}

	for _, role := range roles.List {
		if !roleIds[role.Id] {
			continue
		}

		if role.ConnectionLimit > maxConnectionLimit {
			maxConnectionLimit = role.ConnectionLimit
		}

		for _, systemId := range role.Systems {
			system, ok := systems.GetSystemById(systemId)
			if !ok {
				continue
			}

			scope, found := accessBySystemRef[system.SystemRef]
			if !found {
				scope = &roleAccessScope{talkgroups: map[uint]bool{}}
				accessBySystemRef[system.SystemRef] = scope
			}
			scope.allTalkgroups = true
		}

		for _, talkgroupId := range role.Talkgroups {
			systemRef, talkgroupRef, ok := systems.GetRefsByTalkgroupId(talkgroupId)
			if !ok {
				continue
			}

			scope, found := accessBySystemRef[systemRef]
			if !found {
				scope = &roleAccessScope{talkgroups: map[uint]bool{}}
				accessBySystemRef[systemRef] = scope
			}

			if !scope.allTalkgroups {
				scope.talkgroups[talkgroupRef] = true
			}
		}
	}

	systemRefs := []uint{}
	for systemRef := range accessBySystemRef {
		systemRefs = append(systemRefs, systemRef)
	}
	sort.Slice(systemRefs, func(i int, j int) bool { return systemRefs[i] < systemRefs[j] })

	systemsScope := []any{}
	for _, systemRef := range systemRefs {
		scope := accessBySystemRef[systemRef]

		entry := map[string]any{"id": float64(systemRef)}
		if scope.allTalkgroups {
			entry["talkgroups"] = "*"
		} else {
			talkgroupRefs := []uint{}
			for talkgroupRef := range scope.talkgroups {
				talkgroupRefs = append(talkgroupRefs, talkgroupRef)
			}
			sort.Slice(talkgroupRefs, func(i int, j int) bool { return talkgroupRefs[i] < talkgroupRefs[j] })

			talkgroups := []any{}
			for _, talkgroupRef := range talkgroupRefs {
				talkgroups = append(talkgroups, float64(talkgroupRef))
			}
			entry["talkgroups"] = talkgroups
		}

		systemsScope = append(systemsScope, entry)
	}

	return &Access{
		Ident:   user.Username,
		Systems: systemsScope,
	}, maxConnectionLimit
}
