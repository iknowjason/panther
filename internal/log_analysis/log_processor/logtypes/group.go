package logtypes

/**
 * Panther is a Cloud-Native SIEM for the Modern Security Team.
 * Copyright (C) 2020 Panther Labs Inc
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Finder can find a log entry by name.
// It should return nil if the entry is not found.
type Finder interface {
	Find(logType string) Entry
}

// Group is a named collection of log type entries.
// The purpose of Group is to provide read-only access to a set of log types
type Group interface {
	Name() string
	Collection
	Finder
}

// Collection is a collection of log type entries
type Collection interface {
	Entries() []Entry
	Len() int
}

// FilterPrefix is a helper that filters log type entries in a collection based on a prefix
func FilterPrefix(col Collection, prefix string) (entries []Entry) {
	if col == nil {
		return
	}
	for _, entry := range col.Entries() {
		if strings.HasPrefix(entry.Name(), prefix) {
			entries = append(entries, entry)
		}
	}
	return
}

// AppendFind is a low allocation helper to find multiple entries
func AppendFind(entries []Entry, finder Finder, names ...string) []Entry {
	for _, name := range names {
		if entry := finder.Find(name); entry != nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

type group struct {
	name    string
	entries map[string]Entry
}

var _ Finder = (*group)(nil)

// Must find panics if a log type entry is not found
func MustFind(f Finder, name string) Entry {
	if entry := f.Find(name); entry != nil {
		return entry
	}
	panic(fmt.Sprintf(`entry %q not found`, name))
}

// MustMerge panics the groups cannot be merged
func MustMerge(name string, groups ...Group) Group {
	merged, err := Merge(name, groups...)
	if err != nil {
		panic(err)
	}
	return merged
}

// Merge merges log type entry groups without name conflicts
func Merge(name string, groups ...Group) (Group, error) {
	merged := group{
		name:    name,
		entries: map[string]Entry{},
	}
	for _, g := range groups {
		for _, e := range g.Entries() {
			name := e.String()
			if _, duplicate := merged.entries[name]; duplicate {
				return nil, errors.Errorf(`duplicate entry %q`, name)
			}
			merged.entries[name] = e
		}
	}
	return &merged, nil
}

// Must builds a group of log type entries or panics
func Must(name string, entries ...EntryBuilder) Group {
	index, err := BuildGroup(name, entries...)
	if err != nil {
		panic(err)
	}
	return index
}

// BuildGroup builds a read-only collection of distinct log type entries.
func BuildGroup(name string, entries ...EntryBuilder) (Group, error) {
	index := group{
		entries: make(map[string]Entry, len(entries)),
	}
	for _, b := range entries {
		entry, err := b.BuildEntry()
		if err != nil {
			return nil, err
		}
		name := entry.String()
		if _, duplicate := index.entries[name]; duplicate {
			return nil, errors.Errorf("duplicate log entry %q", name)
		}
		index.entries[name] = entry
	}
	return &index, nil
}

// Find implements Group
func (g *group) Find(name string) Entry {
	return g.entries[name]
}

// Entries implements Group
func (g *group) Entries() (entries []Entry) {
	for _, entry := range g.entries {
		entries = append(entries, entry)
	}
	return entries
}

// Len implements Group
func (g *group) Len() int {
	return len(g.entries)
}

// Name implements Group
func (g *group) Name() string {
	return g.name
}
