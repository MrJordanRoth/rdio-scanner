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

type User struct {
	Id           uint64
	Username     string
	PasswordHash string
	Email        string
	IsSuspended  bool
	Groups       []uint64
}

func NewUser() *User {
	return &User{Groups: []uint64{}}
}

func (user *User) FromMap(m map[string]any) *User {
	switch v := m["id"].(type) {
	case float64:
		user.Id = uint64(v)
	}

	switch v := m["username"].(type) {
	case string:
		user.Username = v
	}

	switch v := m["passwordHash"].(type) {
	case string:
		user.PasswordHash = v
	}

	switch v := m["email"].(type) {
	case string:
		user.Email = v
	}

	switch v := m["isSuspended"].(type) {
	case bool:
		user.IsSuspended = v
	}

	user.Groups = userIdsFromAny(m["groups"])
	if len(user.Groups) == 0 {
		user.Groups = userIdsFromAny(m["roles"])
	}

	return user
}

func (user *User) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"id":           user.Id,
		"username":     user.Username,
		"passwordHash": user.PasswordHash,
		"email":        user.Email,
		"isSuspended":  user.IsSuspended,
		"groups":       user.Groups,
	}

	return json.Marshal(m)
}

type Users struct {
	List  []*User
	mutex sync.Mutex
}

func NewUsers() *Users {
	return &Users{
		List:  []*User{},
		mutex: sync.Mutex{},
	}
}

func (users *Users) Add(user *User) (*Users, bool) {
	users.mutex.Lock()
	defer users.mutex.Unlock()

	added := true

	for _, u := range users.List {
		if strings.EqualFold(u.Username, user.Username) {
			u.PasswordHash = user.PasswordHash
			u.Email = user.Email
			u.Groups = user.Groups
			added = false
		}
	}

	if added {
		users.List = append(users.List, user)
	}

	return users, added
}

func (users *Users) FromMap(f []any) *Users {
	users.mutex.Lock()
	defer users.mutex.Unlock()

	users.List = []*User{}

	for _, r := range f {
		switch m := r.(type) {
		case map[string]any:
			user := NewUser().FromMap(m)
			users.List = append(users.List, user)
		}
	}

	return users
}

func (users *Users) GetUserById(id uint64) (user *User, ok bool) {
	users.mutex.Lock()
	defer users.mutex.Unlock()

	for _, user := range users.List {
		if user.Id == id {
			return user, true
		}
	}

	return nil, false
}

func (users *Users) GetUserByUsername(username string) (user *User, ok bool) {
	users.mutex.Lock()
	defer users.mutex.Unlock()

	for _, user := range users.List {
		if strings.EqualFold(user.Username, username) {
			return user, true
		}
	}

	return nil, false
}

func (users *Users) Read(db *Database) error {
	var (
		err   error
		query string
		rows  *sql.Rows
	)

	users.mutex.Lock()
	defer users.mutex.Unlock()

	users.List = []*User{}

	formatError := errorFormatter("users", "read")

	query = `SELECT "userId", "username", "passwordHash", "email", "isSuspended" FROM "users"`
	if rows, err = db.Sql.Query(query); err != nil {
		return formatError(err, query)
	}

	for rows.Next() {
		user := NewUser()

		if err = rows.Scan(&user.Id, &user.Username, &user.PasswordHash, &user.Email, &user.IsSuspended); err != nil {
			break
		}

		if user.Groups, err = users.readGroups(db, user.Id); err != nil {
			break
		}

		users.List = append(users.List, user)
	}

	rows.Close()

	if err != nil {
		return formatError(err, "")
	}

	sort.Slice(users.List, func(i int, j int) bool {
		return strings.ToLower(users.List[i].Username) < strings.ToLower(users.List[j].Username)
	})

	return nil
}

func (users *Users) Remove(user *User) (*Users, bool) {
	users.mutex.Lock()
	defer users.mutex.Unlock()

	removed := false

	for i, u := range users.List {
		if u.Id > 0 && user.Id > 0 && u.Id == user.Id {
			users.List = append(users.List[:i], users.List[i+1:]...)
			removed = true
			break
		}

		if strings.EqualFold(u.Username, user.Username) {
			users.List = append(users.List[:i], users.List[i+1:]...)
			removed = true
			break
		}
	}

	return users, removed
}

func (users *Users) Write(db *Database) error {
	var (
		err     error
		query   string
		rows    *sql.Rows
		tx      *sql.Tx
		userIds = []uint64{}
	)

	users.mutex.Lock()
	defer users.mutex.Unlock()

	formatError := errorFormatter("users", "write")

	if tx, err = db.Sql.Begin(); err != nil {
		return formatError(err, "")
	}

	query = `SELECT "userId" FROM "users"`
	if rows, err = tx.Query(query); err != nil {
		tx.Rollback()
		return formatError(err, query)
	}

	for rows.Next() {
		var userId uint64
		if err = rows.Scan(&userId); err != nil {
			break
		}
		remove := true
		for _, user := range users.List {
			if user.Id == 0 || user.Id == userId {
				remove = false
				break
			}
		}
		if remove {
			userIds = append(userIds, userId)
		}
	}

	rows.Close()

	if err != nil {
		tx.Rollback()
		return formatError(err, "")
	}

	if len(userIds) > 0 {
		if b, err := json.Marshal(userIds); err == nil {
			in := strings.ReplaceAll(strings.ReplaceAll(string(b), "[", "("), "]", ")")
			query = fmt.Sprintf(`DELETE FROM "users" WHERE "userId" IN %s`, in)
			if _, err = tx.Exec(query); err != nil {
				tx.Rollback()
				return formatError(err, query)
			}
		}
	}

	for _, user := range users.List {
		var count uint

		if user.Id > 0 {
			query = fmt.Sprintf(`SELECT COUNT(*) FROM "users" WHERE "userId" = %d`, user.Id)
			if err = tx.QueryRow(query).Scan(&count); err != nil {
				break
			}
		}

		if count == 0 {
			query = fmt.Sprintf(`INSERT INTO "users" ("username", "passwordHash", "email", "isSuspended") VALUES ('%s', '%s', '%s', %t)`, escapeQuotes(user.Username), escapeQuotes(user.PasswordHash), escapeQuotes(user.Email), user.IsSuspended)
			if _, err = tx.Exec(query); err != nil {
				break
			}
		} else {
			if strings.TrimSpace(user.PasswordHash) == "" {
				query = fmt.Sprintf(`UPDATE "users" SET "username" = '%s', "email" = '%s', "isSuspended" = %t WHERE "userId" = %d`, escapeQuotes(user.Username), escapeQuotes(user.Email), user.IsSuspended, user.Id)
			} else {
				query = fmt.Sprintf(`UPDATE "users" SET "username" = '%s', "passwordHash" = '%s', "email" = '%s', "isSuspended" = %t WHERE "userId" = %d`, escapeQuotes(user.Username), escapeQuotes(user.PasswordHash), escapeQuotes(user.Email), user.IsSuspended, user.Id)
			}
			if _, err = tx.Exec(query); err != nil {
				break
			}
		}

		if user.Id == 0 {
			query = fmt.Sprintf(`SELECT "userId" FROM "users" WHERE "username" = '%s'`, escapeQuotes(user.Username))
			if err = tx.QueryRow(query).Scan(&user.Id); err != nil {
				break
			}
		}

		query = fmt.Sprintf(`DELETE FROM "userRoles" WHERE "userId" = %d`, user.Id)
		if _, err = tx.Exec(query); err != nil {
			break
		}

		added := map[uint64]bool{}
		for _, roleId := range user.Groups {
			if roleId == 0 {
				continue
			}
			if added[roleId] {
				continue
			}
			added[roleId] = true
			query = fmt.Sprintf(`INSERT INTO "userRoles" ("userId", "roleId") VALUES (%d, %d)`, user.Id, roleId)
			if _, err = tx.Exec(query); err != nil {
				break
			}
		}

		if err != nil {
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

func (users *Users) readGroups(db *Database, userId uint64) ([]uint64, error) {
	var (
		err    error
		groups = []uint64{}
		query  string
		rows   *sql.Rows
	)

	query = fmt.Sprintf(`SELECT "roleId" FROM "userRoles" WHERE "userId" = %d`, userId)
	if rows, err = db.Sql.Query(query); err != nil {
		return groups, err
	}
	defer rows.Close()

	for rows.Next() {
		var roleId uint64
		if err = rows.Scan(&roleId); err != nil {
			return groups, err
		}
		groups = append(groups, roleId)
	}

	return groups, nil
}

func userIdsFromAny(v any) []uint64 {
	ids := []uint64{}

	switch values := v.(type) {
	case []uint64:
		return values
	case []any:
		for _, value := range values {
			switch i := value.(type) {
			case float64:
				ids = append(ids, uint64(i))
			case int:
				ids = append(ids, uint64(i))
			case uint64:
				ids = append(ids, i)
			case map[string]any:
				switch id := i["id"].(type) {
				case float64:
					ids = append(ids, uint64(id))
				}
			}
		}
	}

	return ids
}
